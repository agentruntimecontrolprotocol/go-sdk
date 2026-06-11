package server

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"sort"
	"sync"
	"time"

	arcp "github.com/agentruntimecontrolprotocol/go-sdk"
	"github.com/agentruntimecontrolprotocol/go-sdk/internal/eventlog"
	"github.com/agentruntimecontrolprotocol/go-sdk/internal/idstore"
	"github.com/agentruntimecontrolprotocol/go-sdk/messages"
	"github.com/agentruntimecontrolprotocol/go-sdk/transport"
)

// AgentFunc is the agent body. The runtime invokes it inside a job;
// returning (any, nil) produces job.result with the marshalled output,
// returning a non-nil error produces job.error. To stream results,
// call jc.StreamResult and return (nil, nil) after the writer is
// closed.
type AgentFunc func(ctx context.Context, input json.RawMessage, jc *JobContext) (any, error)

type agentEntry struct {
	name       string
	versions   map[string]AgentFunc
	defaultVer string
	bare       AgentFunc
}

// Server is the runtime. Construct with New and call Accept once per
// session.
type Server struct {
	opts Options

	mu      sync.RWMutex
	agents  map[string]*agentEntry
	jobs    map[string]*Job
	subs    map[string][]*subscription
	idStore idstore.Store
	idTTL   time.Duration
	log     *eventlog.Memory

	resumeMu sync.Mutex
	resumes  map[string]*resumeEntry

	// lifeCtx is the server-owned root context. It is cancelled
	// exactly once, by Close, and is the parent of every accepted
	// session and accepted job context. Decoupling job context from
	// the submitting session context lets jobs continue running
	// (and emitting into the event log) across an unexpected
	// transport drop, which is what the resume contract promises.
	lifeCtx    context.Context
	lifeCancel context.CancelFunc

	sessMu           sync.Mutex
	sessions         map[*session]struct{}
	sessionsDone     chan struct{}
	sessionsDoneOnce sync.Once

	// seqAllocs holds the per-session-id event_seq counter. It is
	// allocated at handshake and reused on resume so events emitted by
	// jobs that survive a transport disconnect cannot collide with
	// events emitted by the resumed session.
	allocsMu  sync.Mutex
	seqAllocs map[string]*seqAlloc

	// curMu guards current, the session-id → live-session map. Event
	// delivery resolves the *current* session for a session id so a
	// surviving job (or subscription) whose original session struct was
	// replaced by a resume delivers to the reconnected transport rather
	// than the original (closed) outbox.
	curMu   sync.RWMutex
	current map[string]*session

	closeOnce sync.Once
	closeCh   chan struct{}
}

// seqAlloc is the per-session-id monotonic event_seq counter shared
// across the original session struct and any successor session
// created by resume.
type seqAlloc struct {
	mu  sync.Mutex
	val uint64
}

func (a *seqAlloc) next() uint64 {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.val++
	return a.val
}

func (a *seqAlloc) current() uint64 {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.val
}

func (a *seqAlloc) setIfGreater(v uint64) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if v > a.val {
		a.val = v
	}
}

// allocFor returns the seqAlloc for sessionID, creating one if absent.
func (s *Server) allocFor(sessionID string) *seqAlloc {
	s.allocsMu.Lock()
	defer s.allocsMu.Unlock()
	a, ok := s.seqAllocs[sessionID]
	if !ok {
		a = &seqAlloc{}
		s.seqAllocs[sessionID] = a
	}
	return a
}

func (s *Server) dropAlloc(sessionID string) {
	s.allocsMu.Lock()
	defer s.allocsMu.Unlock()
	delete(s.seqAllocs, sessionID)
}

// setCurrentSession marks sess as the live session for its id, so event
// delivery routed via any (possibly stale) session struct with the same
// id reaches sess's transport.
func (s *Server) setCurrentSession(sess *session) {
	s.curMu.Lock()
	s.current[sess.id] = sess
	s.curMu.Unlock()
}

// clearCurrentSession removes sess as the live session for its id, but
// only if it is still the current one (a later resume may have already
// installed a successor).
func (s *Server) clearCurrentSession(sess *session) {
	s.curMu.Lock()
	if s.current[sess.id] == sess {
		delete(s.current, sess.id)
	}
	s.curMu.Unlock()
}

// currentSession returns the live session for id, or nil when none is
// connected (events are then persisted to the log for later replay).
func (s *Server) currentSession(id string) *session {
	s.curMu.RLock()
	defer s.curMu.RUnlock()
	return s.current[id]
}

// setIDStore replaces the server's idempotency store. Exported via a
// test-only constructor below for fault injection.
func (s *Server) setIDStore(store idstore.Store) {
	s.idStore = store
}

// resumeEntry is the per-session state kept across an unexpected
// disconnect so a subsequent session.hello carrying a matching
// resume_token can pick up the event stream where it left off. The
// entry is removed on graceful session.bye and on successful resume,
// and lazily purged after expiresAt.
type resumeEntry struct {
	sessionID   string
	principal   string
	features    []string
	clientCap   messages.HelloCapabilities
	seq         uint64
	resumeToken string
	expiresAt   time.Time
}

// New returns a Server initialised with opts.
func New(opts Options) *Server {
	o := opts.withDefaults()
	lifeCtx, lifeCancel := context.WithCancel(context.Background())
	const idTTL = 24 * time.Hour
	s := &Server{
		opts:         o,
		agents:       map[string]*agentEntry{},
		jobs:         map[string]*Job{},
		subs:         map[string][]*subscription{},
		idStore:      idstore.NewMemory(idTTL),
		idTTL:        idTTL,
		log:          eventlog.NewMemory(10_000),
		resumes:      map[string]*resumeEntry{},
		lifeCtx:      lifeCtx,
		lifeCancel:   lifeCancel,
		sessions:     map[*session]struct{}{},
		sessionsDone: make(chan struct{}),
		seqAllocs:    map[string]*seqAlloc{},
		current:      map[string]*session{},
		closeCh:      make(chan struct{}),
	}
	// Start the background janitor that reclaims expired idempotency
	// keys, expired resume entries, and their event logs without
	// requiring any submit/resume traffic. It reschedules itself via
	// the (test-injectable) Clock and stops rescheduling on Close.
	s.scheduleJanitor()
	return s
}

// scheduleJanitor arms the next janitor tick on the configured Clock.
func (s *Server) scheduleJanitor() {
	s.opts.Clock.AfterFunc(s.janitorInterval(), s.janitorTick)
}

// janitorInterval chooses how often the janitor runs: frequent enough
// to reclaim resume entries soon after they expire, but bounded so the
// idempotency TTL is swept several times over its window.
func (s *Server) janitorInterval() time.Duration {
	iv := s.idTTL / 24
	if s.opts.ResumeWindow > 0 && s.opts.ResumeWindow < iv {
		iv = s.opts.ResumeWindow
	}
	if iv <= 0 {
		iv = time.Minute
	}
	return iv
}

// janitorTick performs one reclamation pass and reschedules itself
// unless the server is closing.
func (s *Server) janitorTick() {
	select {
	case <-s.closeCh:
		return
	default:
	}
	s.sweepIdle()
	s.purgeExpiredResumes()
	s.scheduleJanitor()
}

// sweepIdle removes idempotency entries older than the configured TTL.
func (s *Server) sweepIdle() {
	cutoff := s.opts.Clock.Now().Add(-s.idTTL)
	_, _ = s.idStore.Sweep(s.lifeCtx, cutoff)
}

// purgeExpiredResumes drops resume entries past their window and trims
// their event logs and seq allocators, independent of resume traffic.
func (s *Server) purgeExpiredResumes() {
	now := s.opts.Clock.Now()
	var expired []string
	s.resumeMu.Lock()
	for id, e := range s.resumes {
		if now.After(e.expiresAt) {
			delete(s.resumes, id)
			expired = append(expired, id)
		}
	}
	s.resumeMu.Unlock()
	for _, id := range expired {
		_ = s.log.Trim(id, ^uint64(0))
		s.dropAlloc(id)
	}
}

// registerSession adds sess to the active set; returns false if the
// server is already closed.
func (s *Server) registerSession(sess *session) bool {
	s.sessMu.Lock()
	defer s.sessMu.Unlock()
	select {
	case <-s.closeCh:
		return false
	default:
	}
	s.sessions[sess] = struct{}{}
	return true
}

func (s *Server) unregisterSession(sess *session) {
	s.sessMu.Lock()
	delete(s.sessions, sess)
	last := len(s.sessions) == 0
	closed := false
	select {
	case <-s.closeCh:
		closed = true
	default:
	}
	s.sessMu.Unlock()
	if last && closed {
		s.sessionsDoneOnce.Do(func() { close(s.sessionsDone) })
	}
}

// stashResume records sess as resumable until now+ResumeWindow. Called
// on non-graceful session exit (drop, heartbeat-lost, transport
// error). The recorded seq is read from the shared per-session-id
// allocator so a successor session takes over with a counter that
// reflects every event a still-running job has emitted since the
// transport dropped.
func (s *Server) stashResume(sess *session, token string) {
	s.resumeMu.Lock()
	defer s.resumeMu.Unlock()
	s.resumes[sess.id] = &resumeEntry{
		sessionID:   sess.id,
		principal:   sess.principal,
		features:    append([]string(nil), sess.features...),
		clientCap:   sess.clientCap,
		seq:         sess.currentSeq(),
		resumeToken: token,
		expiresAt:   s.opts.Clock.Now().Add(s.opts.ResumeWindow),
	}
}

// claimResume validates that a hello.Resume targets a known, unexpired
// resume entry whose token AND authenticated principal match. The entry
// is only deleted once every check passes, so a failed token or
// principal check leaves the legitimate owner's resume state intact and
// claimable. On failure it returns a structured *arcp.Error.
// Concurrently expired entries are purged opportunistically.
func (s *Server) claimResume(req messages.ResumeRequest, principal string) (*resumeEntry, error) {
	s.resumeMu.Lock()
	defer s.resumeMu.Unlock()
	now := s.opts.Clock.Now()
	// Lazy purge of expired entries.
	for id, e := range s.resumes {
		if now.After(e.expiresAt) {
			delete(s.resumes, id)
			_ = s.log.Trim(id, ^uint64(0))
		}
	}
	entry, ok := s.resumes[req.SessionID]
	if !ok {
		return nil, arcp.ErrResumeWindowExpired.WithMessage("session " + req.SessionID + " is not resumable")
	}
	if now.After(entry.expiresAt) {
		delete(s.resumes, req.SessionID)
		return nil, arcp.ErrResumeWindowExpired
	}
	if subtle.ConstantTimeCompare([]byte(entry.resumeToken), []byte(req.ResumeToken)) != 1 {
		// Leave the entry in place: a leaked/incorrect token must not
		// destroy the rightful owner's resume state.
		return nil, arcp.ErrUnauthenticated.WithMessage("resume_token mismatch")
	}
	if entry.principal != principal {
		// Same rationale: a principal mismatch must not strand the
		// legitimate owner's buffered events.
		return nil, arcp.ErrUnauthenticated.WithMessage("resume principal mismatch")
	}
	delete(s.resumes, req.SessionID)
	return entry, nil
}

// dropResume removes any resume entry for sessionID, for graceful
// session.bye. The seq allocator is dropped too because no future
// resume can reattach to this session id once it has been gracefully
// closed.
func (s *Server) dropResume(sessionID string) {
	s.resumeMu.Lock()
	delete(s.resumes, sessionID)
	s.resumeMu.Unlock()
	// Reclaim the gracefully-closed session's buffered event log too;
	// no future resume can reattach to a gracefully-closed session id.
	_ = s.log.Trim(sessionID, ^uint64(0))
	s.dropAlloc(sessionID)
}

// RegisterAgent registers fn under the bare name.
func (s *Server) RegisterAgent(name string, fn AgentFunc) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e := s.agents[name]
	if e == nil {
		e = &agentEntry{name: name, versions: map[string]AgentFunc{}}
		s.agents[name] = e
	}
	e.bare = fn
}

// RegisterAgentVersion registers fn under name@version.
func (s *Server) RegisterAgentVersion(name, version string, fn AgentFunc) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e := s.agents[name]
	if e == nil {
		e = &agentEntry{name: name, versions: map[string]AgentFunc{}}
		s.agents[name] = e
	}
	e.versions[version] = fn
}

// SetDefaultAgentVersion records the version returned for bare-name
// resolution.
func (s *Server) SetDefaultAgentVersion(name, version string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.agents[name]
	if !ok {
		return arcp.ErrAgentNotAvailable.WithMessage("agent " + name + " is not registered")
	}
	if _, ok := e.versions[version]; !ok {
		return arcp.ErrAgentVersionNotAvailable.WithMessage(name + "@" + version + " is not registered")
	}
	e.defaultVer = version
	return nil
}

// resolveAgent picks the AgentFunc and canonical "name@version" for
// the requested ref.
func (s *Server) resolveAgent(ref messages.AgentRef) (AgentFunc, string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	e, ok := s.agents[ref.Name]
	if !ok {
		return nil, "", arcp.ErrAgentNotAvailable.WithMessage("agent " + ref.Name + " is not registered")
	}
	if ref.Version != "" {
		fn, ok := e.versions[ref.Version]
		if !ok {
			return nil, "", arcp.ErrAgentVersionNotAvailable.WithMessage(ref.String() + " is not registered")
		}
		return fn, ref.String(), nil
	}
	if e.defaultVer != "" {
		fn, ok := e.versions[e.defaultVer]
		if !ok {
			return nil, "", arcp.ErrAgentVersionNotAvailable.WithMessage(ref.Name + "@" + e.defaultVer + " missing despite default")
		}
		return fn, ref.Name + "@" + e.defaultVer, nil
	}
	if e.bare != nil {
		return e.bare, ref.Name, nil
	}
	if len(e.versions) > 0 {
		var chosen string
		for k := range e.versions {
			if chosen == "" || k < chosen {
				chosen = k
			}
		}
		return e.versions[chosen], ref.Name + "@" + chosen, nil
	}
	return nil, "", arcp.ErrAgentNotAvailable.WithMessage("agent " + ref.Name + " has no registered handlers")
}

// inventory snapshots the registered agent table for session.welcome.
func (s *Server) inventory() []messages.AgentEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]messages.AgentEntry, 0, len(s.agents))
	for _, e := range s.agents {
		entry := messages.AgentEntry{Name: e.name, Default: e.defaultVer}
		for v := range e.versions {
			entry.Versions = append(entry.Versions, v)
		}
		sort.Strings(entry.Versions)
		out = append(out, entry)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// Accept runs one session over t. It blocks until t is closed, ctx is
// cancelled, or an unrecoverable error fires. On a non-graceful exit
// the session is recorded as resumable for ResumeWindow so a
// subsequent session.hello carrying the matching resume_token can pick
// up the event stream. If Server.Close has already fired before
// handshake completes, Accept returns ErrServerClosed without running
// the session.
func (s *Server) Accept(ctx context.Context, t transport.Transport) error {
	sess, err := s.handshake(ctx, t)
	if err != nil {
		_ = t.Close()
		return err
	}
	if !s.registerSession(sess) {
		_ = t.Close()
		return ErrServerClosed
	}
	defer s.unregisterSession(sess)
	runErr := sess.run(ctx)
	if sess.gracefulBye.Load() {
		s.dropResume(sess.id)
	} else if sess.resumeToken != "" {
		s.stashResume(sess, sess.resumeToken)
	}
	return runErr
}

// ErrServerClosed is returned by Accept when Server.Close has already
// fired.
var ErrServerClosed = arcp.ErrInternalError.WithMessage("server closed")

// Close terminates all active sessions and active jobs. It is
// idempotent: subsequent calls are no-ops and return nil.
//
// Close cancels the server's lifetime context, which propagates to
// every job context, then cancels each active session context (which
// unblocks the per-session transport read loop) and closes its
// transport. Close returns after every session's run loop has exited.
func (s *Server) Close() error {
	first := false
	s.closeOnce.Do(func() {
		first = true
		close(s.closeCh)
		// Cancel every job: their contexts descend from lifeCtx.
		s.lifeCancel()
	})
	if !first {
		return nil
	}
	s.sessMu.Lock()
	sessions := make([]*session, 0, len(s.sessions))
	for sess := range s.sessions {
		sessions = append(sessions, sess)
	}
	if len(sessions) == 0 {
		s.sessionsDoneOnce.Do(func() { close(s.sessionsDone) })
	}
	s.sessMu.Unlock()
	for _, sess := range sessions {
		sess.cancel()
		_ = sess.transport.Close()
	}
	<-s.sessionsDone
	return nil
}

// features returns the advertised feature list.
func (s *Server) features() []string {
	if len(s.opts.Features) > 0 {
		return s.filterFeatures(s.opts.Features)
	}
	return s.filterFeatures(arcp.Features)
}

func (s *Server) filterFeatures(in []string) []string {
	out := make([]string, 0, len(in))
	for _, f := range in {
		if (f == "provisioned_credentials" || f == "model.use") && s.opts.Provisioner == nil {
			continue
		}
		out = append(out, f)
	}
	return out
}

func (s *Server) registerJob(j *Job) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.jobs[j.id]; ok {
		return false
	}
	s.jobs[j.id] = j
	return true
}

func (s *Server) lookupJob(id string) *Job {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.jobs[id]
}

func (s *Server) unregisterJob(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.jobs, id)
	delete(s.subs, id)
}

func (s *Server) listJobs(principal string, filter messages.ListJobsFilter, limit int, cursor string) ([]messages.JobInfo, string, error) {
	s.mu.RLock()
	jobs := make([]*Job, 0, len(s.jobs))
	for _, j := range s.jobs {
		if j.principal == principal {
			jobs = append(jobs, j)
		}
	}
	s.mu.RUnlock()
	var out []messages.JobInfo
	for _, j := range jobs {
		info := j.snapshot()
		if !filterMatch(filter, info) {
			continue
		}
		out = append(out, info)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].JobID < out[j].JobID
		}
		return out[i].CreatedAt.Before(out[j].CreatedAt)
	})
	if cursor != "" {
		out = out[skipUntilCursor(out, cursor):]
	}
	if limit > 0 && len(out) > limit {
		next := out[limit-1].JobID
		return out[:limit], next, nil
	}
	return out, "", nil
}

func skipUntilCursor(items []messages.JobInfo, cursor string) int {
	for i, it := range items {
		if it.JobID > cursor {
			return i
		}
	}
	return len(items)
}

func filterMatch(f messages.ListJobsFilter, info messages.JobInfo) bool {
	if len(f.Status) > 0 {
		found := false
		for _, s := range f.Status {
			if s == info.Status {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	if f.Agent != "" && info.Agent != f.Agent {
		return false
	}
	if f.CreatedAfter != nil && !info.CreatedAt.After(*f.CreatedAfter) {
		return false
	}
	if f.CreatedBefore != nil && !info.CreatedAt.Before(*f.CreatedBefore) {
		return false
	}
	return true
}

// fanoutEvent sends env to every subscriber of jobID.
func (s *Server) fanoutEvent(ctx context.Context, jobID string, env arcp.Envelope) {
	s.mu.RLock()
	subs := append([]*subscription(nil), s.subs[jobID]...)
	s.mu.RUnlock()
	for _, sub := range subs {
		sub.publish(ctx, env)
	}
}

func (s *Server) addSubscriber(jobID string, sub *subscription) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.subs[jobID] = append(s.subs[jobID], sub)
}

func (s *Server) removeSubscriber(jobID string, sub *subscription) {
	s.mu.Lock()
	defer s.mu.Unlock()
	subs := s.subs[jobID]
	for i, x := range subs {
		if x == sub {
			s.subs[jobID] = append(subs[:i], subs[i+1:]...)
			break
		}
	}
}

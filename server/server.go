package server

import (
	"context"
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
	log     *eventlog.Memory

	closeOnce sync.Once
	closeCh   chan struct{}
}

// New returns a Server initialised with opts.
func New(opts Options) *Server {
	o := opts.withDefaults()
	return &Server{
		opts:    o,
		agents:  map[string]*agentEntry{},
		jobs:    map[string]*Job{},
		subs:    map[string][]*subscription{},
		idStore: idstore.NewMemory(24 * time.Hour),
		log:     eventlog.NewMemory(10_000),
		closeCh: make(chan struct{}),
	}
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
// cancelled, or an unrecoverable error fires.
func (s *Server) Accept(ctx context.Context, t transport.Transport) error {
	sess, err := s.handshake(ctx, t)
	if err != nil {
		_ = t.Close()
		return err
	}
	return sess.run(ctx)
}

// Close terminates all sessions and active jobs.
func (s *Server) Close() error {
	s.closeOnce.Do(func() { close(s.closeCh) })
	return nil
}

// features returns the advertised feature list.
func (s *Server) features() []string {
	if len(s.opts.Features) > 0 {
		out := make([]string, len(s.opts.Features))
		copy(out, s.opts.Features)
		return out
	}
	out := make([]string, len(arcp.Features))
	copy(out, arcp.Features)
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

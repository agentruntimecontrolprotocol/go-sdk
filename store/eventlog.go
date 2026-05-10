package store

import (
	"context"
	"database/sql"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/agentruntimecontrolprotocol/go-sdk"

	// modernc.org/sqlite is a pure-Go SQLite driver; importing for
	// side effects registers it as "sqlite" with database/sql.
	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schema string

// EventLog is an append-only SQLite-backed log of ARCP envelopes
// (RFC §6.4 / §19). It enforces transport-level idempotency via a
// unique (session_id, id) primary key and supports ordered replay for
// resume and subscription backfill.
type EventLog struct {
	db *sql.DB

	// Sequence counters per session, kept in memory and persisted via
	// the `sequence` column. Loaded lazily on first append for a
	// session via SELECT MAX(sequence).
	seqMu sync.Mutex
	seq   map[arcp.SessionID]int64
}

// Open opens an EventLog at dsn. Use ":memory:" for in-process
// testing, or a file path for persistent storage. The schema is
// applied via CREATE IF NOT EXISTS, so calling Open against an
// existing database is safe.
func Open(ctx context.Context, dsn string) (*EventLog, error) {
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite %q: %w", dsn, err)
	}
	// modernc.org/sqlite is fine with multiple connections, but the
	// in-memory mode requires single-connection access to share state
	// across calls. Constrain connections to be safe regardless of
	// dsn.
	db.SetMaxOpenConns(1)

	if _, err := db.ExecContext(ctx, schema); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("apply schema: %w", err)
	}
	return &EventLog{db: db, seq: make(map[arcp.SessionID]int64)}, nil
}

// Close releases the underlying database handle.
func (l *EventLog) Close() error {
	return l.db.Close()
}

// Append writes env to the log under env.SessionID. If a row with the
// same (session_id, id) already exists, Append returns an error that
// satisfies errors.Is(err, arcp.ErrAlreadyExists). The envelope is
// stored as canonical JSON.
//
// Append assigns env.SessionID and env.ID itself if not set: a missing
// SessionID is rejected with INVALID_ARGUMENT; a missing ID is also
// rejected (callers MUST allocate ids).
func (l *EventLog) Append(ctx context.Context, env arcp.Envelope) error {
	if env.SessionID == "" {
		return arcp.NewError(arcp.CodeInvalidArgument, "event log append: empty session_id")
	}
	if env.ID == "" {
		return arcp.NewError(arcp.CodeInvalidArgument, "event log append: empty id")
	}
	if env.Payload == nil {
		return arcp.NewError(arcp.CodeInvalidArgument, "event log append: nil payload")
	}
	envBytes, err := json.Marshal(env)
	if err != nil {
		return fmt.Errorf("marshal envelope: %w", err)
	}
	seq := l.nextSequence(env.SessionID)
	ts := env.Timestamp
	if ts.IsZero() {
		ts = time.Now().UTC()
	}
	const q = `INSERT INTO events
		(session_id, id, type, timestamp, sequence,
		 job_id, stream_id, subscription_id,
		 trace_id, span_id, correlation_id, causation_id,
		 priority, envelope)
		VALUES (?, ?, ?, ?, ?,
		        ?, ?, ?,
		        ?, ?, ?, ?,
		        ?, ?)`
	_, err = l.db.ExecContext(ctx, q,
		string(env.SessionID), string(env.ID), env.Type(), ts.UTC().Format(time.RFC3339Nano), seq,
		nullIfEmpty(string(env.JobID)), nullIfEmpty(string(env.StreamID)), nullIfEmpty(string(env.SubscriptionID)),
		nullIfEmpty(string(env.TraceID)), nullIfEmpty(string(env.SpanID)),
		nullIfEmpty(string(env.CorrelationID)), nullIfEmpty(string(env.CausationID)),
		string(envPriority(env.Priority)), envBytes,
	)
	if err != nil {
		if isUniqueConstraintViolation(err) {
			// Roll the per-session counter back so we don't leave a
			// permanent gap in the sequence numbering when a duplicate
			// id is rejected.
			l.rewindSequence(env.SessionID, seq)
			return arcp.ErrAlreadyExists.WithMessage(
				fmt.Sprintf("event log: duplicate id %q in session %q", env.ID, env.SessionID),
			).WithCause(err)
		}
		l.rewindSequence(env.SessionID, seq)
		return fmt.Errorf("insert event: %w", err)
	}
	return nil
}

// Replay returns all envelopes for sessionID in append order, starting
// after afterID (exclusive). If afterID is empty, replay starts from
// the beginning. The returned slice may be empty.
func (l *EventLog) Replay(ctx context.Context, sessionID arcp.SessionID, afterID arcp.MessageID) ([]arcp.Envelope, error) {
	var afterSeq int64 = -1
	if afterID != "" {
		row := l.db.QueryRowContext(ctx,
			`SELECT sequence FROM events WHERE session_id = ? AND id = ?`,
			string(sessionID), string(afterID))
		if err := row.Scan(&afterSeq); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, arcp.ErrNotFound.WithMessage(
					fmt.Sprintf("event log: no message %q in session %q", afterID, sessionID),
				)
			}
			return nil, fmt.Errorf("locate after_message_id: %w", err)
		}
	}
	rows, err := l.db.QueryContext(ctx,
		`SELECT envelope FROM events
		 WHERE session_id = ? AND sequence > ?
		 ORDER BY sequence ASC`,
		string(sessionID), afterSeq)
	if err != nil {
		return nil, fmt.Errorf("replay query: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []arcp.Envelope
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			return nil, fmt.Errorf("replay scan: %w", err)
		}
		var env arcp.Envelope
		if err := json.Unmarshal(raw, &env); err != nil {
			return nil, fmt.Errorf("replay decode: %w", err)
		}
		out = append(out, env)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("replay rows: %w", err)
	}
	return out, nil
}

// Count returns the number of envelopes stored for sessionID.
// Primarily useful for tests; production code should not rely on it.
func (l *EventLog) Count(ctx context.Context, sessionID arcp.SessionID) (int64, error) {
	var n int64
	err := l.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM events WHERE session_id = ?`,
		string(sessionID)).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("count events: %w", err)
	}
	return n, nil
}

func (l *EventLog) nextSequence(sid arcp.SessionID) int64 {
	l.seqMu.Lock()
	defer l.seqMu.Unlock()
	if cur, ok := l.seq[sid]; ok {
		cur++
		l.seq[sid] = cur
		return cur
	}
	// Cold path: query the persisted max so we resume sequencing
	// correctly after restart. We tolerate the query running outside
	// any caller-supplied ctx because nextSequence is called from
	// Append which holds its own ctx; the lookup is fast.
	var maxSeq sql.NullInt64
	_ = l.db.QueryRow(`SELECT MAX(sequence) FROM events WHERE session_id = ?`,
		string(sid)).Scan(&maxSeq)
	cur := maxSeq.Int64 + 1
	l.seq[sid] = cur
	return cur
}

// rewindSequence undoes a nextSequence reservation when an Append
// fails (e.g. duplicate id). Without rewind, repeated dup attempts
// would create permanent gaps.
func (l *EventLog) rewindSequence(sid arcp.SessionID, attempted int64) {
	l.seqMu.Lock()
	defer l.seqMu.Unlock()
	if cur, ok := l.seq[sid]; ok && cur == attempted {
		l.seq[sid] = cur - 1
	}
}

// nullIfEmpty maps "" to a sql.NullString so the column ends up as
// NULL rather than the empty string. Filter indexes are smaller and
// joins behave correctly.
func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// envPriority returns p, defaulting to PriorityNormal when unset.
func envPriority(p arcp.Priority) arcp.Priority {
	if p == "" {
		return arcp.PriorityNormal
	}
	return p
}

// isUniqueConstraintViolation reports whether err is a SQLite
// UNIQUE-constraint failure. modernc.org/sqlite does not export a
// stable error code for this, so we match on the message prefix used
// by the underlying SQLite C library and ported to Go.
func isUniqueConstraintViolation(err error) bool {
	if err == nil {
		return false
	}
	// modernc.org/sqlite reports messages like:
	//   "constraint failed: UNIQUE constraint failed: events.session_id, events.id (1555)"
	msg := err.Error()
	return strings.Contains(msg, "UNIQUE constraint failed") ||
		strings.Contains(msg, "constraint failed: 1555") ||
		strings.Contains(msg, "(2067)") || // SQLITE_CONSTRAINT_UNIQUE
		strings.Contains(msg, "(1555)") // SQLITE_CONSTRAINT_PRIMARYKEY
}

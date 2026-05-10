-- Event log schema for the ARCP reference Go SDK (RFC §6.4 / §19).
--
-- Every accepted envelope is appended exactly once. The unique
-- constraint on (session_id, id) provides transport-level idempotency
-- for at-least-once delivery: a retransmit with the same id is dropped
-- silently.
--
-- The full envelope (as canonical JSON) is stored in the `envelope`
-- column so resume (§19) can replay it byte-identically. Routing
-- columns are denormalized for cheap filtering during subscription
-- backfill (§13.3).

CREATE TABLE IF NOT EXISTS events (
    session_id      TEXT    NOT NULL,
    id              TEXT    NOT NULL,
    type            TEXT    NOT NULL,
    timestamp       TEXT    NOT NULL,
    sequence        INTEGER NOT NULL,
    job_id          TEXT,
    stream_id       TEXT,
    subscription_id TEXT,
    trace_id        TEXT,
    span_id         TEXT,
    correlation_id  TEXT,
    causation_id    TEXT,
    priority        TEXT    NOT NULL DEFAULT 'normal',
    envelope        BLOB    NOT NULL,
    PRIMARY KEY (session_id, id)
);

CREATE INDEX IF NOT EXISTS events_session_seq_idx
    ON events(session_id, sequence);

CREATE INDEX IF NOT EXISTS events_correlation_idx
    ON events(correlation_id);

CREATE INDEX IF NOT EXISTS events_causation_idx
    ON events(causation_id);

CREATE INDEX IF NOT EXISTS events_trace_idx
    ON events(trace_id);

CREATE INDEX IF NOT EXISTS events_timestamp_idx
    ON events(timestamp);

-- Logical idempotency table (RFC §6.4). The same (principal, key) pair
-- across reconnects MUST return the previous outcome rather than
-- re-executing.
CREATE TABLE IF NOT EXISTS idempotency (
    principal       TEXT    NOT NULL,
    idempotency_key TEXT    NOT NULL,
    outcome         BLOB,
    created_at      TEXT    NOT NULL,
    PRIMARY KEY (principal, idempotency_key)
);

// Package store contains the SQLite-backed event log that supports
// transport idempotency (RFC §6.4), replay, and resume
// (§19, after_message_id only in v0.1).
package store

package store

import (
	"context"
	"errors"
	"testing"

	"github.com/agentruntimecontrolprotocol/go-sdk"
)

// White-box tests for store internals that the external test package
// cannot reach.

func TestIsUniqueConstraintViolation(t *testing.T) {
	t.Parallel()
	if isUniqueConstraintViolation(nil) {
		t.Errorf("nil should not be a unique violation")
	}
	if !isUniqueConstraintViolation(errors.New("UNIQUE constraint failed: events.id")) {
		t.Errorf("classic UNIQUE message should match")
	}
	if !isUniqueConstraintViolation(errors.New("constraint failed: 1555")) {
		t.Errorf("error code 1555 should match")
	}
	if !isUniqueConstraintViolation(errors.New("blah blah (2067)")) {
		t.Errorf("error code 2067 should match")
	}
	if isUniqueConstraintViolation(errors.New("some unrelated error")) {
		t.Errorf("unrelated error should not match")
	}
}

// TestReplayAfterDBClosed exercises the Replay query-error branch by
// closing the database before invoking Replay.
func TestReplayAfterDBClosed(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	l, err := Open(ctx, ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := l.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	_, err = l.Replay(ctx, arcp.SessionID("sess_x"), "")
	if err == nil {
		t.Errorf("expected error when querying closed db")
	}
}

// TestCountAfterDBClosed exercises Count's query-error branch.
func TestCountAfterDBClosed(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	l, err := Open(ctx, ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := l.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	_, err = l.Count(ctx, arcp.SessionID("sess_x"))
	if err == nil {
		t.Errorf("expected error when counting closed db")
	}
}

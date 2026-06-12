package transport

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"runtime"
	"sync"
	"testing"
	"time"

	arcp "github.com/agentruntimecontrolprotocol/go-sdk"
)

// TestStdioLargeLineRoundTrips covers #122: an NDJSON line larger than
// the default 64KiB scanner token (but within the 1MiB limit) must
// round-trip instead of killing the transport with ErrTooLong.
func TestStdioLargeLineRoundTrips(t *testing.T) {
	big := make([]byte, 512*1024)
	for i := range big {
		big[i] = 'a' + byte(i%26)
	}
	env := arcp.Envelope{Type: "job.event", SessionID: "s", Payload: mustJSONString(string(big))}
	line, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	line = append(line, '\n')
	rt := NewStdioTransport(bytes.NewReader(line), io.Discard)
	defer rt.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	got, err := rt.Recv(ctx)
	if err != nil {
		t.Fatalf("large line failed: %v", err)
	}
	if got.Type != "job.event" {
		t.Fatalf("got type %q", got.Type)
	}
}

func mustJSONString(s string) json.RawMessage {
	b, _ := json.Marshal(s)
	return b
}

// TestStdioRecvCancellationDoesNotLeak covers #54: many cancelled Recv
// calls must not spawn per-call scanner goroutines and the transport
// must still deliver a subsequent envelope normally.
func TestStdioRecvCancellationDoesNotLeak(t *testing.T) {
	pr, pw := io.Pipe()
	t.Cleanup(func() { _ = pr.Close(); _ = pw.Close() })
	tx := NewStdioTransport(pr, io.Discard)
	defer tx.Close()

	before := runtime.NumGoroutine()
	for i := 0; i < 200; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
		_, err := tx.Recv(ctx)
		cancel()
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("iter %d: want DeadlineExceeded, got %v", i, err)
		}
	}
	// Allow scheduler to clean up.
	time.Sleep(50 * time.Millisecond)
	after := runtime.NumGoroutine()
	if delta := after - before; delta > 5 {
		t.Fatalf("goroutine leak: %d new goroutines after 200 cancelled Recv calls", delta)
	}

	// And a subsequent envelope is still received cleanly.
	env := arcp.Envelope{ARCP: arcp.ProtocolVersion, ID: "x", Type: "session.ping"}
	body, _ := json.Marshal(env)
	go func() { _, _ = pw.Write(append(body, '\n')) }()
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	got, err := tx.Recv(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got.Type != "session.ping" {
		t.Fatalf("got type %q, want session.ping", got.Type)
	}
}

// TestStdioCloseUnblocksRecv covers transport closure semantics.
func TestStdioCloseUnblocksRecv(t *testing.T) {
	pr, pw := io.Pipe()
	tx := NewStdioTransport(pr, io.Discard)
	done := make(chan error, 1)
	go func() {
		_, err := tx.Recv(context.Background())
		done <- err
	}()
	// Give Recv a moment to enter its select.
	time.Sleep(20 * time.Millisecond)
	if err := tx.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("Recv after Close must return an error")
		}
	case <-time.After(1 * time.Second):
		t.Fatal("Recv did not unblock after Close")
	}
	_ = pw.Close()
}

// TestStdioSendAndRecvRoundtrip is a sanity test for the happy path.
func TestStdioSendAndRecvRoundtrip(t *testing.T) {
	var buf bytes.Buffer
	pr, pw := io.Pipe()
	t.Cleanup(func() { _ = pr.Close(); _ = pw.Close() })
	tx := NewStdioTransport(pr, &buf)
	defer tx.Close()

	go func() {
		out := arcp.Envelope{ARCP: arcp.ProtocolVersion, ID: "1", Type: "session.ping"}
		body, _ := json.Marshal(out)
		_, _ = pw.Write(append(body, '\n'))
	}()
	got, err := tx.Recv(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got.Type != "session.ping" {
		t.Fatalf("type=%s", got.Type)
	}
	if err := tx.Send(context.Background(), arcp.Envelope{Type: "session.pong"}); err != nil {
		t.Fatal(err)
	}
	if buf.Len() == 0 || buf.Bytes()[buf.Len()-1] != '\n' {
		t.Fatal("Send must produce a newline-terminated frame")
	}
}

// TestStdioConcurrentRecvSerializes is mostly defensive: Recv is
// designed to be called from one goroutine, but if two goroutines do
// call it, both should still get a sensible error/result rather than
// race on the scanner.
func TestStdioConcurrentRecvSerializes(t *testing.T) {
	pr, pw := io.Pipe()
	t.Cleanup(func() { _ = pr.Close(); _ = pw.Close() })
	tx := NewStdioTransport(pr, io.Discard)
	defer tx.Close()

	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
			defer cancel()
			_, _ = tx.Recv(ctx)
		}()
	}
	wg.Wait()
}

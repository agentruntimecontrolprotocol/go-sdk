package otel

import (
	"context"
	"errors"
	"testing"

	arcp "github.com/agentruntimecontrolprotocol/go-sdk"
	"github.com/agentruntimecontrolprotocol/go-sdk/transport"
)

// stubTransport records the last sent envelope and returns a canned
// envelope on Recv.
type stubTransport struct {
	lastSent arcp.Envelope
	recv     arcp.Envelope
	recvErr  error
}

func (s *stubTransport) Send(ctx context.Context, env arcp.Envelope) error {
	s.lastSent = env
	return nil
}
func (s *stubTransport) Recv(ctx context.Context) (arcp.Envelope, error) {
	return s.recv, s.recvErr
}
func (s *stubTransport) Close() error { return nil }

func TestWrapTransportPassesThroughWithoutSpan(t *testing.T) {
	inner := &stubTransport{recv: arcp.Envelope{Type: "session.ping"}}
	wrapped := WrapTransport(inner, Options{})
	defer wrapped.Close()
	if err := wrapped.Send(context.Background(), arcp.Envelope{Type: "job.submit"}); err != nil {
		t.Fatal(err)
	}
	if inner.lastSent.Type != "job.submit" {
		t.Fatalf("inner did not receive Send; got %+v", inner.lastSent)
	}
	got, err := wrapped.Recv(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got.Type != "session.ping" {
		t.Fatalf("Recv = %s, want session.ping", got.Type)
	}
}

func TestWrapTransportPropagatesRecvError(t *testing.T) {
	want := errors.New("recv boom")
	inner := &stubTransport{recvErr: want}
	wrapped := WrapTransport(inner, Options{FrameSpans: true})
	if _, err := wrapped.Recv(context.Background()); !errors.Is(err, want) {
		t.Fatalf("Recv err = %v, want %v", err, want)
	}
}

func TestWithDefaultsAtLeastOneSpanKind(t *testing.T) {
	o := Options{}.withDefaults()
	if !o.JobSpans || !o.ToolCallSpans {
		t.Fatal("withDefaults must enable JobSpans + ToolCallSpans when all are zero")
	}
	// Explicit choice is preserved.
	o2 := Options{FrameSpans: true}.withDefaults()
	if o2.JobSpans || o2.ToolCallSpans {
		t.Fatal("withDefaults must NOT toggle other spans when one is set")
	}
}

// Ensure transport.Transport interface is satisfied.
var _ transport.Transport = (*stubTransport)(nil)

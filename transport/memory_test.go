package transport

import (
	"context"
	"errors"
	"testing"
	"time"

	arcp "github.com/agentruntimecontrolprotocol/go-sdk"
)

func TestMemoryPairRoundtrip(t *testing.T) {
	a, b := NewMemoryPair()
	defer a.Close()
	defer b.Close()
	want := arcp.Envelope{ARCP: arcp.ProtocolVersion, ID: "1", Type: "session.ping"}
	ctx := context.Background()
	go func() { _ = a.Send(ctx, want) }()
	got, err := b.Recv(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != want.ID {
		t.Fatalf("got %s, want %s", got.ID, want.ID)
	}
}

func TestMemoryPairCloseReturnsErrClosed(t *testing.T) {
	a, b := NewMemoryPair()
	_ = a.Close()
	if err := a.Send(context.Background(), arcp.Envelope{}); !errors.Is(err, ErrClosed) {
		t.Fatalf("Send after close: %v, want ErrClosed", err)
	}
	if _, err := b.Recv(context.Background()); !errors.Is(err, ErrClosed) {
		t.Fatalf("Recv on closed pair: %v, want ErrClosed", err)
	}
}

func TestMemoryPairRespectsContext(t *testing.T) {
	a, b := NewMemoryPair()
	defer a.Close()
	defer b.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, err := b.Recv(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Recv timeout: %v, want DeadlineExceeded", err)
	}
}

// Package arcptest_test contains cross-package integration tests.
// This file holds shared helpers for setting up paired runtime/client
// instances over the in-memory transport.
package arcptest_test

import (
	"context"
	"io"
	"log/slog"
	"sync"
	"testing"

	"github.com/fizzpop/arcp-go/auth"
	"github.com/fizzpop/arcp-go/messages"
	"github.com/fizzpop/arcp-go/runtime"
	"github.com/fizzpop/arcp-go/transport"
)

// silentLogger discards all logging from runtime/client during tests.
func silentLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// pair starts a runtime in a goroutine and returns a (transport,
// runtimeServeDone) pair that the test code can use as a connected
// client transport.
func pair(t *testing.T, opts runtime.Options) (transport.Transport, <-chan error) {
	t.Helper()
	if opts.Logger == nil {
		opts.Logger = silentLogger()
	}
	if opts.Identity.Kind == "" {
		opts.Identity = messages.RuntimeIdentity{
			Kind:       "test-runtime",
			Version:    "0.1.0",
			TrustLevel: messages.TrustTrusted,
		}
	}
	r, err := runtime.New(opts)
	if err != nil {
		t.Fatalf("runtime.New: %v", err)
	}
	clientT, runtimeT := transport.NewInMemoryPair()
	done := make(chan error, 1)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		ctx, cancel := context.WithCancel(context.Background())
		t.Cleanup(cancel)
		done <- r.Serve(ctx, runtimeT)
	}()
	t.Cleanup(func() { _ = clientT.Close(); _ = runtimeT.Close(); wg.Wait() })
	return clientT, done
}

// bearerVerifier returns a Verifier accepting a single token.
func bearerVerifier(token string, principal auth.Principal) auth.Verifier {
	return &auth.MultiVerifier{
		BySchema: map[messages.AuthScheme]auth.Verifier{
			messages.AuthSchemeBearer: &auth.BearerVerifier{Tokens: map[string]auth.Principal{token: principal}},
		},
	}
}

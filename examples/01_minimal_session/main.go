// Minimal in-memory session: runtime + client, bearer auth, ping/pong.
package main

import (
	"context"
	"io"
	"log"
	"log/slog"
	"os"
	"time"

	"github.com/agentruntimecontrolprotocol/go-sdk/auth"
	"github.com/agentruntimecontrolprotocol/go-sdk/client"
	"github.com/agentruntimecontrolprotocol/go-sdk/messages"
	"github.com/agentruntimecontrolprotocol/go-sdk/runtime"
	"github.com/agentruntimecontrolprotocol/go-sdk/transport"
)

func main() {
	const token = "demo-token"
	principal := auth.Principal{Subject: "demo@example.com", Trust: messages.TrustTrusted}
	verifier := &auth.MultiVerifier{
		ByScheme: map[messages.AuthScheme]auth.Verifier{
			messages.AuthSchemeBearer: &auth.BearerVerifier{
				Tokens: map[string]auth.Principal{token: principal},
			},
		},
	}

	r, err := runtime.New(runtime.Options{
		Auth: verifier,
		Identity: messages.RuntimeIdentity{
			Kind:       "demo-runtime",
			Version:    "0.1.0",
			TrustLevel: messages.TrustTrusted,
		},
		Capabilities: messages.Capabilities{Streaming: true},
		Logger:       slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if err != nil {
		log.Fatalf("runtime.New: %v", err)
	}

	clientT, runtimeT := transport.NewInMemoryPair()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() { errCh <- r.Serve(ctx, runtimeT) }()

	c, err := client.Open(ctx, clientT, client.OpenOptions{
		Auth:   messages.Auth{Scheme: messages.AuthSchemeBearer, Token: token},
		Client: messages.ClientIdentity{Kind: "demo-client", Version: "0.1.0"},
		Capabilities: messages.Capabilities{
			Streaming: true,
		},
		Logger: slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn})),
	})
	if err != nil {
		log.Fatalf("client.Open: %v", err)
	}

	log.Printf("session_id=%s", c.SessionID())
	pong, err := c.Ping(ctx, "hello")
	if err != nil {
		log.Fatalf("ping: %v", err)
	}
	log.Printf("pong=%q", pong)

	_ = c.Close(ctx)
	_ = clientT.Close()
	_ = runtimeT.Close()
	if err := <-errCh; err != nil && err != context.Canceled {
		log.Printf("runtime.Serve: %v", err)
	}
}

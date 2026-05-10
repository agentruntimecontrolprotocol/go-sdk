package arcptest_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/fizzpop/arcp-go"
	"github.com/fizzpop/arcp-go/auth"
	"github.com/fizzpop/arcp-go/client"
	"github.com/fizzpop/arcp-go/messages"
	"github.com/fizzpop/arcp-go/runtime"
)

const (
	testToken         = "secret-token"
	testUser          = "tester@example.com"
	testClientVersion = "0.0.1"
	testClientKind    = "test-client"
)

func defaultPrincipal() auth.Principal {
	return auth.Principal{Subject: testUser, Trust: messages.TrustTrusted}
}

func TestHandshakeBearerHappyPath(t *testing.T) {
	t.Parallel()
	t1, done := pair(t, runtime.Options{
		Auth: bearerVerifier(testToken, defaultPrincipal()),
		Capabilities: messages.Capabilities{
			Streaming:  true,
			HumanInput: true,
		},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	c, err := client.Open(ctx, t1, client.OpenOptions{
		Auth:   messages.Auth{Scheme: messages.AuthSchemeBearer, Token: testToken},
		Client: messages.ClientIdentity{Kind: testClientKind, Version: testClientVersion},
		Capabilities: messages.Capabilities{
			Streaming: true,
		},
		Logger: silentLogger(),
	})
	if err != nil {
		t.Fatalf("client.Open: %v", err)
	}
	if c.SessionID() == "" {
		t.Errorf("session id not assigned")
	}
	if !c.Capabilities().Streaming {
		t.Errorf("expected streaming capability negotiated")
	}
	if c.Capabilities().HumanInput {
		t.Errorf("human_input should not be negotiated (client did not offer)")
	}

	// Round-trip a ping.
	pong, err := c.Ping(ctx, "hello")
	if err != nil {
		t.Fatalf("ping: %v", err)
	}
	if pong != "hello" {
		t.Errorf("pong note = %q, want hello", pong)
	}

	if err := c.Close(ctx); err != nil {
		t.Fatalf("client.Close: %v", err)
	}
	// Drain server side.
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("runtime.Serve returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Errorf("runtime.Serve did not return after client close")
	}
}

func TestHandshakeRejectsBadToken(t *testing.T) {
	t.Parallel()
	t1, _ := pair(t, runtime.Options{
		Auth: bearerVerifier(testToken, defaultPrincipal()),
	})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := client.Open(ctx, t1, client.OpenOptions{
		Auth:   messages.Auth{Scheme: messages.AuthSchemeBearer, Token: "wrong-token"},
		Client: messages.ClientIdentity{Kind: testClientKind, Version: testClientVersion},
	})
	if err == nil {
		t.Fatalf("expected unauthenticated error")
	}
	if !errors.Is(err, arcp.ErrUnauthenticated) {
		t.Errorf("expected ErrUnauthenticated, got %v (code=%q)", err, arcp.Code(err))
	}
}

func TestHandshakeAnonymousRejectedWithoutCapability(t *testing.T) {
	t.Parallel()
	t1, _ := pair(t, runtime.Options{
		// Server does NOT advertise Anonymous: true.
		Auth: &auth.MultiVerifier{
			ByScheme: map[messages.AuthScheme]auth.Verifier{
				messages.AuthSchemeNone: auth.AnonymousVerifier{},
			},
		},
	})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := client.Open(ctx, t1, client.OpenOptions{
		Auth:   messages.Auth{Scheme: messages.AuthSchemeNone},
		Client: messages.ClientIdentity{Kind: "anon", Version: testClientVersion},
	})
	if err == nil {
		t.Fatalf("expected error for anonymous without capability")
	}
	if !errors.Is(err, arcp.ErrUnauthenticated) {
		t.Errorf("expected ErrUnauthenticated, got %v", err)
	}
}

func TestHandshakeAnonymousAllowedWithCapability(t *testing.T) {
	t.Parallel()
	t1, _ := pair(t, runtime.Options{
		Auth: auth.AnonymousVerifier{},
		Capabilities: messages.Capabilities{
			Anonymous: true,
		},
	})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	c, err := client.Open(ctx, t1, client.OpenOptions{
		Auth:   messages.Auth{Scheme: messages.AuthSchemeNone},
		Client: messages.ClientIdentity{Kind: "anon", Version: testClientVersion, Principal: "guest"},
		Capabilities: messages.Capabilities{
			Anonymous: true,
		},
	})
	if err != nil {
		t.Fatalf("client.Open anonymous: %v", err)
	}
	if c.SessionID() == "" {
		t.Errorf("session id missing")
	}
}

func TestHandshakeRejectsRequiredUnsupportedExtension(t *testing.T) {
	t.Parallel()
	t1, _ := pair(t, runtime.Options{
		Auth: bearerVerifier(testToken, defaultPrincipal()),
		// Server does NOT advertise this extension.
	})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := client.Open(ctx, t1, client.OpenOptions{
		Auth:   messages.Auth{Scheme: messages.AuthSchemeBearer, Token: testToken},
		Client: messages.ClientIdentity{Kind: testClientKind, Version: testClientVersion},
		Capabilities: messages.Capabilities{
			Extensions: []string{"arcpx.required-but-missing.v1"},
		},
	})
	if err == nil {
		t.Fatalf("expected rejection for unknown extension")
	}
	if !errors.Is(err, arcp.ErrUnimplemented) {
		t.Errorf("expected ErrUnimplemented, got %v (code=%q)", err, arcp.Code(err))
	}
}

func TestHandshakeRejectsNonHandshakeFirstMessage(t *testing.T) {
	t.Parallel()
	t1, done := pair(t, runtime.Options{
		Auth: bearerVerifier(testToken, defaultPrincipal()),
	})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Send a ping as the very first message — runtime should reject.
	if err := t1.Send(ctx, arcp.Envelope{
		ID:        arcp.NewMessageID(),
		Timestamp: time.Now().UTC(),
		Payload:   &messages.Ping{Note: "premature"},
	}); err != nil {
		t.Fatalf("send ping: %v", err)
	}
	resp, err := t1.Recv(ctx)
	if err != nil {
		t.Fatalf("recv: %v", err)
	}
	rej, ok := resp.Payload.(*messages.SessionRejected)
	if !ok {
		t.Fatalf("expected SessionRejected, got %T", resp.Payload)
	}
	if rej.Code != arcp.CodeFailedPrecondition {
		t.Errorf("expected FAILED_PRECONDITION, got %q", rej.Code)
	}
	// Server should have returned now.
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Errorf("runtime.Serve did not return after rejection")
	}
}

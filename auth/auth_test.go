package auth

import (
	"context"
	"errors"
	"testing"

	arcp "github.com/agentruntimecontrolprotocol/go-sdk"
)

func TestStaticBearerAccepted(t *testing.T) {
	v := StaticBearer(map[string]string{"good-token": "alice"})
	principal, err := v.Verify(context.Background(), "good-token")
	if err != nil {
		t.Fatal(err)
	}
	if principal != "alice" {
		t.Fatalf("principal = %s, want alice", principal)
	}
}

func TestStaticBearerRejectedWrapsErrInvalidToken(t *testing.T) {
	v := StaticBearer(map[string]string{"good-token": "alice"})
	_, err := v.Verify(context.Background(), "bad-token")
	if err == nil {
		t.Fatal("expected error for unknown token")
	}
	if !errors.Is(err, arcp.ErrUnauthenticated) {
		t.Fatalf("err must wrap ErrUnauthenticated, got %v", err)
	}
	if !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("err must wrap ErrInvalidToken, got %v", err)
	}
}

func TestVerifierFunc(t *testing.T) {
	called := false
	v := VerifierFunc(func(ctx context.Context, token string) (string, error) {
		called = true
		return "p", nil
	})
	p, err := v.Verify(context.Background(), "x")
	if err != nil {
		t.Fatal(err)
	}
	if p != "p" || !called {
		t.Fatalf("VerifierFunc not invoked: p=%s called=%v", p, called)
	}
}

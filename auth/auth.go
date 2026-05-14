// Package auth defines the Verifier interface used by the server to
// validate bearer tokens at the session.hello handshake.
package auth

import (
	"context"
	"errors"

	arcp "github.com/agentruntimecontrolprotocol/go-sdk"
)

// Verifier validates a bearer token and returns the authenticated
// principal name.
type Verifier interface {
	Verify(ctx context.Context, token string) (principal string, err error)
}

// VerifierFunc adapts a plain function to the Verifier interface.
type VerifierFunc func(ctx context.Context, token string) (string, error)

// Verify implements Verifier.
func (f VerifierFunc) Verify(ctx context.Context, token string) (string, error) {
	return f(ctx, token)
}

// StaticBearer returns a Verifier that accepts a fixed set of tokens.
// The map keys are the accepted token strings; the values are the
// resulting principal identifiers.
func StaticBearer(tokens map[string]string) Verifier {
	cp := make(map[string]string, len(tokens))
	for k, v := range tokens {
		cp[k] = v
	}
	return VerifierFunc(func(ctx context.Context, token string) (string, error) {
		if principal, ok := cp[token]; ok {
			return principal, nil
		}
		return "", arcp.ErrUnauthenticated
	})
}

// ErrInvalidToken is returned by StaticBearer when the token is not in
// the configured set.
var ErrInvalidToken = errors.New("auth: invalid token")

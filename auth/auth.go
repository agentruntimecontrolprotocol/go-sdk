// Package auth defines the Verifier interface used by the server to
// validate bearer tokens at the session.hello handshake.
package auth

import (
	"context"
	"crypto/subtle"
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
// resulting principal identifiers. Unknown tokens produce
// arcp.ErrUnauthenticated wrapping ErrInvalidToken via
// (*arcp.Error).WithCause, whose Unwrap returns the cause, so both
// errors.Is(err, arcp.ErrUnauthenticated) and
// errors.Is(err, ErrInvalidToken) hold (see TestStaticBearer* in
// auth_test.go).
func StaticBearer(tokens map[string]string) Verifier {
	type tokenEntry struct {
		token     []byte
		principal string
	}
	entries := make([]tokenEntry, 0, len(tokens))
	for k, v := range tokens {
		entries = append(entries, tokenEntry{token: []byte(k), principal: v})
	}
	return VerifierFunc(func(ctx context.Context, token string) (string, error) {
		// Compare against every configured token with a constant-time
		// comparison and without an early exit, so we do not leak
		// timing information about which secret (if any) the candidate
		// matched. This mirrors the resume_token comparison in the
		// server package.
		candidate := []byte(token)
		principal := ""
		matched := 0
		for _, e := range entries {
			if subtle.ConstantTimeCompare(candidate, e.token) == 1 {
				principal = e.principal
				matched = 1
			}
		}
		if matched == 1 {
			return principal, nil
		}
		return "", arcp.ErrUnauthenticated.WithCause(ErrInvalidToken)
	})
}

// ErrInvalidToken is the sentinel returned by StaticBearer (wrapped
// inside arcp.ErrUnauthenticated) when the supplied token is not in
// the configured set. Custom Verifier implementations are encouraged
// to do the same when the failure is specifically an unrecognised
// token, so generic auth-failure handlers can detect it with
// errors.Is.
var ErrInvalidToken = errors.New("auth: invalid token")

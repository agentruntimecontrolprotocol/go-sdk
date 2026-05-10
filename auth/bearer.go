package auth

import (
	"context"

	"github.com/agentruntimecontrolprotocol/go-sdk"
	"github.com/agentruntimecontrolprotocol/go-sdk/messages"
)

// BearerVerifier validates `bearer`-scheme credentials against an
// in-memory token map (RFC §8.2). Production deployments would back
// this with a token introspection service; for v0.1 the static map
// suffices.
type BearerVerifier struct {
	// Tokens maps token strings to the Principal they authenticate.
	// Lookup is constant time; values are compared by-value.
	Tokens map[string]Principal
}

// Verify implements Verifier.
func (v *BearerVerifier) Verify(_ context.Context, auth messages.Auth, _ messages.ClientIdentity) (Principal, error) {
	if auth.Scheme != messages.AuthSchemeBearer {
		return Principal{}, arcp.ErrUnauthenticated.WithMessage("bearer verifier requires scheme=bearer")
	}
	if auth.Token == "" {
		return Principal{}, arcp.ErrUnauthenticated.WithMessage("missing bearer token")
	}
	p, ok := v.Tokens[auth.Token]
	if !ok {
		return Principal{}, arcp.ErrUnauthenticated.WithMessage("unknown bearer token")
	}
	return p, nil
}

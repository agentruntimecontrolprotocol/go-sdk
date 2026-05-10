package auth

import (
	"context"

	"github.com/fizzpop/arcp-go"
	"github.com/fizzpop/arcp-go/messages"
)

// Principal is the verified identity of an authenticated session
// participant (RFC §8). Subject is the canonical principal name (e.g.
// an email or service id); Trust is the assigned trust level.
type Principal struct {
	Subject string
	Trust   messages.TrustLevel
}

// Verifier validates the credentials advertised in session.open or
// session.authenticate (RFC §8.2). Implementations MUST return an
// error wrapping arcp.ErrUnauthenticated for invalid credentials and
// wrapping arcp.ErrUnimplemented for schemes they do not support.
type Verifier interface {
	Verify(ctx context.Context, auth messages.Auth, client messages.ClientIdentity) (Principal, error)
}

// AnonymousVerifier accepts only the `none` scheme and assigns
// principals the `untrusted` trust level. The runtime gates this on
// the negotiated `anonymous` capability before invoking it.
type AnonymousVerifier struct{}

// Verify implements Verifier.
func (AnonymousVerifier) Verify(_ context.Context, auth messages.Auth, client messages.ClientIdentity) (Principal, error) {
	if auth.Scheme != messages.AuthSchemeNone {
		return Principal{}, arcp.ErrUnauthenticated.WithMessage("anonymous verifier requires scheme=none")
	}
	subj := client.Principal
	if subj == "" {
		subj = "anonymous"
	}
	return Principal{Subject: subj, Trust: messages.TrustUntrusted}, nil
}

// MultiVerifier dispatches to a per-scheme Verifier based on auth.Scheme.
// A request whose scheme is not in the map fails with UNIMPLEMENTED.
type MultiVerifier struct {
	BySchema map[messages.AuthScheme]Verifier
}

// Verify implements Verifier.
func (m *MultiVerifier) Verify(ctx context.Context, auth messages.Auth, client messages.ClientIdentity) (Principal, error) {
	v, ok := m.BySchema[auth.Scheme]
	if !ok {
		return Principal{}, arcp.ErrUnimplemented.WithMessage(
			"auth scheme not configured: " + string(auth.Scheme))
	}
	return v.Verify(ctx, auth, client)
}

# Package `auth`

Import path: `github.com/agentruntimecontrolprotocol/go-sdk/auth`

`auth` defines the verifier interface used by `server.Options` and
ships an in-memory helper for tests and small static deployments.

## API

```go
type Verifier interface {
    Verify(ctx context.Context, token string) (principal string, err error)
}

// Function adapter.
type VerifierFunc func(ctx context.Context, token string) (string, error)
func (f VerifierFunc) Verify(ctx context.Context, token string) (string, error)

// Helper: fixed token → principal map.
func StaticBearer(tokens map[string]string) Verifier

// Sentinel returned by StaticBearer (wrapped inside arcp.ErrUnauthenticated)
// for unrecognised tokens.
var ErrInvalidToken = errors.New("auth: invalid token")
```

Return `arcp.ErrUnauthenticated` for invalid bearer tokens. The
principal string returned from a successful verification is used for
job ownership and visibility decisions. Custom verifiers can attach
`auth.ErrInvalidToken` as a cause (`arcp.ErrUnauthenticated.WithCause(auth.ErrInvalidToken)`)
so generic auth-failure handlers can detect it with `errors.Is`.

See [guides/auth.md](../guides/auth.md) for end-to-end usage.

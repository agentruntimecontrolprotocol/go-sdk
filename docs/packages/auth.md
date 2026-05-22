# Package `auth`

Import path: `github.com/agentruntimecontrolprotocol/go-sdk/auth`

`auth` defines the verifier interface used by `server.Options`.

```go
type Verifier interface {
	Verify(ctx context.Context, token string) (principal string, err error)
}
```

Return `arcp.ErrUnauthenticated` for invalid bearer tokens. The
principal string returned from a successful verification is used for
job ownership and visibility decisions.

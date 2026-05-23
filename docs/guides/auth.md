# Authentication (§6.1)

ARCP defines bearer authentication for `session.hello`. The Go SDK's
client always sends `scheme: "bearer"` with the token from
`client.Options.Token`; servers plug in a `auth.Verifier` to map
tokens to principals.

## `auth.Verifier`

```go
type Verifier interface {
    Verify(ctx context.Context, token string) (principal string, err error)
}
```

`VerifierFunc(fn)` adapts a plain function. Return any non-nil error
to reject the session; the runtime then sends `session.error` with
code `UNAUTHENTICATED` and closes. Returning `arcp.ErrUnauthenticated`
(optionally `.WithCause(auth.ErrInvalidToken)` to label the failure
mode) keeps the response on the canonical taxonomy:

```go
srv := server.New(server.Options{
    Verifier: auth.VerifierFunc(func(ctx context.Context, token string) (string, error) {
        if token != expected {
            return "", arcp.ErrUnauthenticated.WithCause(auth.ErrInvalidToken)
        }
        return "principal-123", nil
    }),
})
```

The returned string is the runtime principal. It is used for job
ownership checks, cancellation authorization, subscriptions, and
`session.list_jobs` visibility.

## `auth.StaticBearer`

For tests, local demos, and small static deployments, use the
`StaticBearer` helper:

```go
srv := server.New(server.Options{
    Verifier: auth.StaticBearer(map[string]string{
        "tok-alice": "alice",
        "tok-bob":   "bob",
    }),
})
```

Unknown tokens fail with `arcp.ErrUnauthenticated` wrapping
`auth.ErrInvalidToken`, so callers can route on either with
`errors.Is`.

## No verifier

When `Verifier` is nil the runtime accepts the session unconditionally
and uses `session.hello.client.name` as the principal. That mode is
useful for tests and examples, never for untrusted clients.

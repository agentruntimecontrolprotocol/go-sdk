# Authentication (§6.1)

ARCP defines bearer authentication for `session.hello`.

```go
srv := server.New(server.Options{
	Verifier: auth.VerifierFunc(func(ctx context.Context, token string) (string, error) {
		if token != expected {
			return "", arcp.ErrUnauthenticated
		}
		return "principal-123", nil
	}),
})
```

The returned string is the runtime principal. It is used for job
ownership checks, cancellation authorization, subscriptions, and
`session.list_jobs` visibility.

When no verifier is configured, the runtime accepts the session and
uses `session.hello.client.name` as the principal. That mode is useful
for tests and examples, not for untrusted clients.

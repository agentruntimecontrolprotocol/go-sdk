# Sessions (§6)

A session starts with `session.hello` and succeeds with
`session.welcome` or fails with `session.error`.

```go
cli, err := client.Connect(ctx, t, client.Options{
	ClientName: "reporter",
	Token:      os.Getenv("ARCP_TOKEN"),
})
```

The client advertises supported features, the runtime returns the
intersection, and `Client.HasFeature` reports what can be used.

Server-side authentication is optional. Configure
`server.Options.Verifier` to validate bearer tokens and map them to a
principal. Without a verifier, the runtime uses the client name as the
principal for local demos and tests.

Close cleanly with `Client.Close`, which sends `session.bye` and closes
the underlying transport.

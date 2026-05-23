# Sessions (§6)

A session starts with `session.hello` and succeeds with
`session.welcome` or fails with `session.error`.

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="../diagrams/session-lifecycle-dark.svg">
  <img alt="ARCP session lifecycle" src="../diagrams/session-lifecycle-light.svg">
</picture>

```go
cli, err := client.Connect(ctx, t, client.Options{
    ClientName:    "reporter",
    ClientVersion: "1.0.0",
    Token:         os.Getenv("ARCP_TOKEN"),
})
if err != nil {
    log.Fatal(err)
}
defer cli.Close(ctx)
```

`client.Options` covers identity (`ClientName`, `ClientVersion`),
authentication (`Token`), the advertised feature list (`Features`,
empty uses the SDK default), auto-ack tuning (`AutoAckWindow`,
`AutoAckInterval`), and the optional `Resume` block — see
[Resume](./resume.md).

## Welcome accessors

After `Connect` returns, the runtime's welcome is available via:

| Accessor | Value |
| --- | --- |
| `cli.SessionID()` | The negotiated session id (a ULID). |
| `cli.Welcome()` | The full `*messages.SessionWelcome` — runtime name, resume token, resume window, heartbeat interval, agent inventory. |
| `cli.Features()` | The effective negotiated feature set (intersection of client + server). |
| `cli.HasFeature(name)` | Convenience predicate over `Features()`. |

The client advertises supported features, the runtime returns the
intersection, and `Client.HasFeature` reports what can be used.

## Authentication

Server-side authentication is optional. Configure
`server.Options.Verifier` to validate bearer tokens and map them to a
principal. Without a verifier, the runtime uses the client name as
the principal — useful for local demos and tests, never for
production. See [Authentication](./auth.md) for the `Verifier`
interface, the `StaticBearer` helper, and the `ErrInvalidToken`
sentinel.

## Heartbeats

When both peers negotiate the `heartbeat` feature, the server sends
`session.ping` every `HeartbeatInterval` (default `30s`) and arms a
watchdog timer of `2 × HeartbeatInterval`. If no inbound envelope
arrives in that window the session is force-closed with
`HEARTBEAT_LOST` and becomes resumable. The Go client auto-responds
to inbound pings; agents don't need to manage this themselves.

## Graceful close

`Client.Close` sends `session.bye` and closes the underlying
transport. Graceful close also clears the runtime's resume state for
the session, so a subsequent `Resume` attempt against that
`session_id` is refused with `RESUME_WINDOW_EXPIRED`. Use [resume](./resume.md)
for unexpected transport drops, not for planned shutdowns.

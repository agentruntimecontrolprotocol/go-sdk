# Resume (§6.3)

The runtime advertises a resume token and window in `session.welcome`.
The Go client exposes the welcome payload through `Client.Welcome()`.

Current reconnect flows are built from the same transport and session
primitives as a fresh connection. Event replay for subscriptions uses
the runtime's in-memory event log and `job.subscribe` with
`History: true` plus `FromEventSeq`.

For durable resume across process restarts, embed the server with a
transport and event log strategy appropriate for your deployment.

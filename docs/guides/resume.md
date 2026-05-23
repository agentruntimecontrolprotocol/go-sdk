# Resume (§6.3)

The runtime advertises a fresh `resume_token` and `resume_window_sec`
in every `session.welcome`. After a transport drop, present the prior
session id and token back on the next `session.hello` and the runtime
replays buffered events that the client never confirmed.

```go
// Capture welcome state for later resume.
welcome := cli.Welcome()
prior := messages.ResumeRequest{
    SessionID:    cli.SessionID(),
    ResumeToken:  welcome.ResumeToken,
    LastEventSeq: cli.HighestSeq(),
}
// ...
// Reconnect: dial a fresh transport, then present the resume block.
t, _ := transport.DialWebSocket(ctx, url, transport.WebSocketOptions{})
cli2, err := client.Connect(ctx, t, client.Options{
    Token:  os.Getenv("ARCP_TOKEN"),
    Resume: &prior,
})
if err != nil {
    // RESUME_WINDOW_EXPIRED, UNAUTHENTICATED, or a transport error.
    log.Fatal(err)
}
// cli2.SessionID() == prior.SessionID; cli2.Welcome().ResumeToken is fresh.
```

On the server side, the runtime stashes resume state on any
non-graceful exit (transport drop, heartbeat-lost, read error) and
retains it for `ResumeWindow` (default `10m`). Validating a resume
hello does three things: matches the `resume_token` in constant time,
confirms the new connection authenticates as the same principal, and
mints a new token before the welcome is sent. The previous token is
single-use; reusing it returns `UNAUTHENTICATED`.

A graceful `session.bye` clears the resume state immediately. So does
client-side `Client.Close`, which sends the bye for you.

## What replays

The eventlog buffers every job-scoped envelope (`job.event`,
`job.result`, `job.error`) under its session id. On resume the
runtime returns every entry with `event_seq > LastEventSeq` ordered
by seq, then resumes live traffic on the same seq sequence the prior
transport had reached.

`session.ack` trims the log: events whose seq is at or below the
client's last-acked seq are eligible for eviction. The buffer caps at
10,000 entries per session; sustained acks well below the high-water
mark keep the resume tail bounded.

## Subscription replay

Subscriptions are independent of session resume. Use
`SubscribeOptions{History: true, FromEventSeq: N}` to replay buffered
events for a specific job from any session — this is the read-only
dashboard pattern, not a recovery mechanism for the original
submitter.

## Known limitation: job lifetime

The Go SDK currently binds each `AgentFunc` invocation to the
submitting session's context. When the transport drops, that context
is cancelled and the agent goroutine stops. Resume replays the
buffered tail so the client catches up gap-free, but new work does
**not** continue while the session is dead. Jobs that need to
outlive a transport drop must implement their own checkpointing and
re-submission today.

For durable resume across process restarts, swap the in-memory event
log for a persistent implementation of `eventlog.Log` — the resume
registry is also in-memory and resets on `server.New`.

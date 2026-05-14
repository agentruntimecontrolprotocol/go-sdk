---
title: Heartbeat
sdk: go
spec_sections: [§6.4, §12]
order: 1
kind: feature
---

# Heartbeat

**Negotiation flag:** `heartbeat`.

When both peers advertise `heartbeat`, the server's
`Options.HeartbeatInterval` is mirrored back as
`session.welcome.payload.heartbeat_interval_sec`. The runtime emits
`session.ping` on an idle outbound timer; the client must respond with
`session.pong` within `2 × heartbeat_interval_sec` or the runtime
fires `HEARTBEAT_LOST` and closes the transport. Jobs continue to
run; the session is recoverable via resume.

## Wire

- `session.ping { nonce, sent_at }`
- `session.pong { ping_nonce, received_at }`

## Use

```go
srv := server.New(server.Options{HeartbeatInterval: 30 * time.Second})
```

The client side is automatic: `client.Connect` reads
`HeartbeatIntervalSec` from `welcome` and reflects pings back to the
server.

## Errors

- `HEARTBEAT_LOST` — surfaced as a session-scoped close; client
  handles see the cause via `Subscription.Err` and `JobHandle.Err`.

## See also

- [examples/heartbeat](../../examples/heartbeat)

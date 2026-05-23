# Troubleshooting

## `session.error: INVALID_REQUEST`

Check the first envelope type, JSON payload shape, and feature-gated
operations. The most common cause is a feature-gated request issued
when the feature was not negotiated:

- `session.list_jobs` needs `list_jobs`
- `job.subscribe` needs `subscribe`
- `session.ack` needs `ack`

The Go client surfaces these as `arcp.ErrInvalidRequest` with a
message of the form `"<feature> feature not negotiated"`. Confirm
both sides advertise the feature, then check the effective set with
`Client.HasFeature`.

## Missing feature

Both sides advertise features and the runtime uses the intersection.
Use `Client.HasFeature` after connect to confirm the effective set.
The server filters its advertised set: `provisioned_credentials` **and
`model.use`** are dropped unless `server.Options.Provisioner` is set,
so both depend on having a credential provider configured.

## `PERMISSION_DENIED`

Lease patterns did not match the operation target. Validate with
`JobContext.ValidateLeaseOp` before invoking external tools or models.
Cross-session attempts to cancel or subscribe to a job submitted by a
different principal are also reported as `PERMISSION_DENIED`.

## `BUDGET_EXHAUSTED`

The job consumed its `cost.budget`, or an upstream credential provider
reported budget exhaustion. Treat it as a terminal policy boundary
unless the agent can continue without the costly operation. The
runtime emits `cost.budget.remaining` metric events after each
`cost.*` debit so callers can observe drawdown before exhaustion.

## `HEARTBEAT_LOST`

When `heartbeat` is negotiated, the server arms a watchdog timer of
`2 × Options.HeartbeatInterval` (default `60 s`). If no inbound
envelope (ping/pong, ack, or any client message) arrives in that
window the session is force-closed with `HEARTBEAT_LOST` and is
eligible for resume.

## Resume failures

A `RESUME_WINDOW_EXPIRED` or `UNAUTHENTICATED` error during
`client.Connect` with `Options.Resume` means: the session id is
unknown to this runtime, the resume token doesn't match what the
runtime stashed, the runtime has restarted (its in-memory resume
registry is lost), or the previous session ended with a graceful
`session.bye` (which clears the resume state). The token rotates on
every successful welcome; the *previous* token is single-use.

Note: this SDK's resume replays the buffered tail of the event log so
clients catch up gap-free, but the in-process `AgentFunc` is bound to
the original session's context and stops running when the transport
drops. Jobs that need to outlive a session drop must implement their
own checkpointing today.

## WebSocket connect failures

Confirm the handler path, scheme, and any auth verifier. WebSocket
URLs passed to `transport.DialWebSocket` must point at the ARCP
upgrade endpoint. By default the server-side `Handler` accepts only
loopback `Host` headers (`localhost`, `127.0.0.1`, `[::1]`) and
returns HTTP 421 otherwise — broaden `Options.AllowedHosts`
explicitly to expose the runtime beyond loopback.

## `INTERNAL_ERROR: client closed before job.accepted arrived`

The transport dropped (or `Client.Close` was called) after `Submit`
returned but before the runtime echoed `job.accepted`. The handle's
`Wait` then returns this error so callers don't block forever.
Retry the submit when the next session is up.

# ARCP v1.0 Conformance Matrix

Status legend:

- **Implemented** — fully implemented and tested.
- **Partial** — implemented for a documented subset; the rest returns
  `arcp.ErrUnimplemented`.
- **Deferred** — out of scope for v0.1; calls return
  `arcp.ErrUnimplemented` with the section reference.

| RFC Section                              | Status      | Notes                                           |
| ---------------------------------------- | ----------- | ----------------------------------------------- |
| §6.1 Envelope                            | Implemented | Custom JSON dispatch, 91.7% coverage            |
| §6.4 Delivery semantics / idempotency    | Implemented | SQLite event log w/ unique (session_id, id)     |
| §6.5 Priority and QoS                    | Partial     | Field carried; scheduling deferred              |
| §7 Capability negotiation                | Implemented | Boolean intersection + extension AND-must-match |
| §8 Authentication: bearer                | Implemented | auth.BearerVerifier with token map              |
| §8 Authentication: signed_jwt            | Deferred    | Pushed to v0.2 (challenge flow not implemented) |
| §8 Authentication: none (anonymous)      | Implemented | auth.AnonymousVerifier; gated on capability     |
| §8 Authentication: mtls                  | Deferred    | v0.2                                            |
| §8 Authentication: oauth2                | Deferred    | v0.2                                            |
| §8.4 Re-authentication                   | Deferred    | session.refresh round-trip pending v0.2         |
| §8.5 Eviction                            | Partial     | Message defined; runtime emits not yet wired up |
| §9 Sessions: stateless                   | Implemented | runtime.Runtime serves session over Transport   |
| §9 Sessions: stateful                    | Partial     | Per-session state in handshake; durable in v0.2 |
| §9 Sessions: durable                     | Deferred    | v0.2                                            |
| §10.2 Job state machine                  | Pending     | Phase 3                                         |
| §10.3 Heartbeats                         | Pending     | Phase 3                                         |
| §10.4 Cancellation                       | Pending     | Phase 3                                         |
| §10.5 Interrupts                         | Pending     | Phase 3                                         |
| §10.6 Scheduled jobs                     | Deferred    | v0.2                                            |
| §11 Streams: text/event/log/thought      | Pending     | Phase 3                                         |
| §11.3 Binary encoding (base64)           | Pending     | Phase 3                                         |
| §11.3 Binary encoding (sidecar)          | Deferred    | v0.2                                            |
| §11.2 Backpressure                       | Pending     | Phase 3                                         |
| §12 Human-in-the-loop                    | Pending     | Phase 4                                         |
| §13 Subscriptions                        | Pending     | Phase 5                                         |
| §14 Multi-agent (delegate/handoff)       | Deferred    | v0.2                                            |
| §15 Permissions / leases                 | Pending     | Phase 4                                         |
| §15.6 Trust elevation                    | Deferred    | v0.2                                            |
| §16 Artifacts (inline base64)            | Pending     | Phase 5                                         |
| §17 Observability (log/metric/trace)     | Pending     | Phase 1+ (logging via slog)                     |
| §18 Error model                          | Implemented | Full taxonomy, sentinels, errors.Is/As support  |
| §19 Resumability (after_message_id only) | Partial     | Event log Replay(after) ready; runtime in P5    |
| §19 Checkpoint-based resume              | Deferred    | v0.2                                            |
| §20 MCP compatibility                    | Deferred    | v0.2 (out-of-scope wrappers)                    |
| §21 Extensions                           | Implemented | Namespace validation; registry; unknown-msg in P2 |
| §22 Transport: WebSocket                 | Pending     | Phase 6                                         |
| §22 Transport: stdio                     | Pending     | Phase 6                                         |
| §22 Transport: HTTP/2                    | Deferred    | v0.2                                            |
| §22 Transport: QUIC                      | Deferred    | v0.2                                            |

This file is updated at the close of each phase. The "Pending" entries become
"Implemented" once the corresponding gate has passed and the integration tests
exercise the surface end-to-end.

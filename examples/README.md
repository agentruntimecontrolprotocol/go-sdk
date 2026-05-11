# Go SDK examples

Two flavors live here.

`01_minimal_session/` is the only example wired end-to-end — it
boots an in-memory transport pair, runs the Go runtime, opens a
client session, and pings. Use it as a smoke test of the actual
SDK surface.

The remaining 14 directories are **illustrative**, one per ARCP
primitive from RFC-0001 v2. They import from the SDK as if the
high-level driver (`Session`, with `Request` / `Send` / `Events`
helpers) were already published. Setup boilerplate (transport URL,
identity, auth) is elided behind a small `Session` shim per
example. The protocol code in `main.go` is what you read.

## The fourteen

| Directory | Demonstrates | Spec |
|---|---|---|
| [`subscriptions/`](./subscriptions) | Three Observer clients on one session, three filters, three sinks. | §5, §13 |
| [`leases/`](./leases) | Lease-gated shell agent. Read leases coarse, write leases scoped. | §15.4–§15.5 |
| [`lease_revocation/`](./lease_revocation) | Per-table leases with `lease.revoked` / `lease.extended` mid-flight. | §15.5 |
| [`permission_challenge/`](./permission_challenge) | Two-party permission challenge — generator asks, reviewer holds veto. | §15.4, §6.4 |
| [`delegation/`](./delegation) | `agent.delegate` fan-out + `JobMux` channel router demuxing by `job_id`. | §14, §6.4 |
| [`handoff/`](./handoff) | `agent.handoff` with transcript packed as artifact, runtime fingerprint pinned. | §14, §16, §8.3 |
| [`heartbeats/`](./heartbeats) | Worker federation; heartbeat-loss reroute via `idempotency_key`. | §10.3, §6.4 |
| [`capability_negotiation/`](./capability_negotiation) | Capability-driven peer routing; standard `cost.usd` rollups. | §7, §17.3.1, §18.3 |
| [`resumability/`](./resumability) | **Actually crash and resume.** `os.Exit` mid-flight; second invocation picks up at the next step. | §10, §19, §6.4 |
| [`reasoning_streams/`](./reasoning_streams) | `kind: thought` stream + a peer runtime that subscribes and delegates critiques back. | §11.4, §13, §14 |
| [`extensions/`](./extensions) | Custom `arcpx.sdr.*.v1` extension namespace with correct unknown-message handling. | §21 |
| [`human_input/`](./human_input) | `human.input.request` fanned across phone/email/Slack; first-wins resolution. | §12 |
| [`cancellation/`](./cancellation) | Cooperative `cancel` (terminate) vs `interrupt` (pause and ask). | §10.4–§10.5 |
| [`mcp/`](./mcp) | ARCP runtime fronting an MCP server: `tool.invoke` → MCP `call_tool`. | §20 |

## Conventions

- Go 1.25+, `gofmt`-clean, `go vet ./examples/...` clean.
- Each example is one `main.go` (the protocol code) plus 0–2 stub
  files in the same `package main` (`agents.go`, `synth.go`,
  `cheap.go`, `work.go`, `channels.go`, `sql.go`, `upstream.go`,
  `sinks.go`, etc.). Stubs `panic("not implemented: ...")`.
- The `Session` shim in each example stands in for the as-if-
  published high-level driver. The real SDK exposes
  `client.Open(ctx, transport, opts)`; the shim adds `Request`,
  `Events`, and an envelope helper that fills in `session_id` /
  timestamps. Once the high-level driver lands these examples
  collapse to its real method names.
- Envelopes match RFC-0001 v2 exactly. Custom message types follow
  §21.1 `arcpx.<domain>.<name>.v<n>` naming.

## What's where in the SDK

- `arcp.Envelope`, `arcp.Error`, `arcp.ErrorCode`, `arcp.NewMessageID()`
  / `NewJobID()` / `NewStreamID()` / `NewArtifactID()` —
  wire primitives in the root package.
- `arcp/messages` — typed payload structs for every wire type.
- `arcp/client` — low-level handshake driver (`client.Open`, `Send`,
  `Recv`, `Ping`, `Close`).
- `arcp/runtime` — server-side runtime.
- `arcp/transport` — in-memory + transport interfaces (WebSocket
  vendored separately).

## Reading order

For a brisk tour: `subscriptions`, `leases`, `delegation`,
`resumability`, `cancellation`, `extensions`, `mcp`. Those seven
exercise the bulk of the protocol.

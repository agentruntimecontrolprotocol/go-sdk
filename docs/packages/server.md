# Package `server`

Import path: `github.com/agentruntimecontrolprotocol/go-sdk/server`

`server` hosts ARCP agents and runtime sessions. It implements the
session loop, agent registry, job lifecycle, lease enforcement, the
resume registry, and credential provisioning.

## Entry point

```go
srv := server.New(server.Options{Name: "runtime"})
srv.RegisterAgent("echo", func(ctx context.Context, input json.RawMessage, jc *server.JobContext) (any, error) {
    return map[string]json.RawMessage{"echo": input}, nil
})
// One Accept call per session.
_ = srv.Accept(ctx, t)
```

## Types

| Type | Purpose |
| --- | --- |
| `Server` | Runtime instance; accepts transports and owns jobs. |
| `Options` | Identity, heartbeats, resume window, verifier, logger, clock, ack-lag threshold, feature override, provisioner, result-size cap, chunk size. |
| `AgentFunc` | `func(ctx, input, *JobContext) (any, error)` invoked per accepted job. |
| `Job` | Server-side job snapshot exposed via `JobContext`; accessors `ID`, `Agent`, `Principal`, `Lease`. |
| `JobContext` | Agent-facing surface for events, lease ops, budget, streaming, credentials, traceability. |

## `Server` methods

| Method | Purpose |
| --- | --- |
| `RegisterAgent(name, fn)` | Register a bare-name handler. |
| `RegisterAgentVersion(name, version, fn)` | Register `name@version`. |
| `SetDefaultAgentVersion(name, version)` | Choose the default for bare-name resolution. |
| `Accept(ctx, t)` | Run one session over t; stashes resume state on non-graceful exit. |
| `Close()` | Terminate sessions and active jobs. |

## `Options` fields

| Field | Default | Notes |
| --- | --- | --- |
| `Name`, `Version` | `"arcp-go-runtime"`, `"1.0.0"` | Echoed in `session.welcome`. |
| `HeartbeatInterval` | `30s` | Server `session.ping` cadence; watchdog at `2 ×`. |
| `ResumeWindow` | `10m` | How long resume state survives a non-graceful exit. |
| `Verifier` | nil | `auth.Verifier` for token authentication. |
| `Logger` | `slog.Default()` | Runtime logger. |
| `Clock` | `clock.Real()` | Injectable time source for tests. |
| `AckLagThreshold` | `0` | Trigger a `back_pressure` status when N events go unacked. |
| `Features` | `arcp.Features` | Override the advertised set. |
| `Provisioner` | nil | When nil, `provisioned_credentials` and `model.use` are removed from the advertised set. |
| `MaxResultBytes` | `32 MiB` | Cap on streamed results. |
| `ChunkSize` | `1 MiB` | Cap on a single `result_chunk` body. |

## `JobContext` surface

Identity/state: `JobID`, `SessionID`, `TraceID`, `Context`, `Lease`,
`Budget`, `ValidateLeaseOp`.

Event emitters: `Log`, `Thought`, `ToolCall`, `ToolResult`,
`ToolError`, `Status`, `Metric`, `ArtifactRef`, `Progress`,
`RotateCredential`, `StreamResult`.

See [guides/job-events.md](../guides/job-events.md) for what each
emitter writes to the wire.

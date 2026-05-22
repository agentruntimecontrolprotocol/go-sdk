# Architecture

The Go SDK is a set of small packages around the ARCP envelope and
message catalog.

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="./diagrams/architecture-dark.svg">
  <img alt="ARCP Go SDK architecture" src="./diagrams/architecture-light.svg">
</picture>

| Package | Responsibility |
| --- | --- |
| `go-sdk` | Envelope construction, IDs, errors, feature flags, lease types. |
| `messages` | Typed payload structs for protocol envelopes. |
| `client` | Session handshake, submit, cancel, subscribe, list jobs, result streaming. |
| `server` | Runtime session loop, agent registry, job lifecycle, leases, budgets, credentials. |
| `transport` | WebSocket, stdio NDJSON, and in-memory transports. |
| `auth` | Bearer verifier interface and helpers. |
| `middleware/*` | Adapters for existing HTTP routers and OpenTelemetry propagation. |

The runtime owns agent execution. Agents receive a `*server.JobContext`
for events, lease validation, budget debits, chunked results, and
credential rotation.

## Versioning

`arcp.ProtocolVersion` is the wire version. `arcp.SDKVersion` is the Go
module version advertised by default clients. Feature flags are
negotiated during `session.hello` and `session.welcome`; callers can
override the advertised set through client and server options.

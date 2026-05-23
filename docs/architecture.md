# Architecture

The Go SDK is a set of small packages around the ARCP envelope and
message catalog.

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="./diagrams/architecture-dark.svg">
  <img alt="ARCP Go SDK architecture" src="./diagrams/architecture-light.svg">
</picture>

| Package | Responsibility |
| --- | --- |
| `go-sdk` (root, `arcp`) | Envelope, ID constructors, the fifteen-code error taxonomy, feature flags, `Lease`/`Capability` types, `IsLeaseSubset`. |
| `messages` | Typed payload structs and wire-type tokens for every protocol envelope. |
| `client` | Session handshake, submit, cancel, subscribe, list jobs, result streaming, resume. |
| `server` | Runtime session loop, agent registry with `name@version` resolution, job lifecycle, lease enforcement, budgets, credentials, resume registry. |
| `transport` | WebSocket (`coder/websocket`), stdio NDJSON, and in-memory transports. |
| `auth` | `Verifier` interface, `VerifierFunc` adapter, `StaticBearer` helper. |
| `credentials` | `Provisioner` interface plus a `Memory` reference implementation for `provisioned_credentials` sessions. |
| `middleware/nethttp` | `Handler` wrapping `server.Server` for `*http.Server`; carries the loopback-only `AllowedHosts` default. |
| `middleware/chi` | `Mount` helper that attaches the nethttp handler to a `chi.Router`. |
| `middleware/otel` | `WrapTransport` propagates W3C trace context inside `envelope.extensions` and (per `Options`) emits frame, job, and tool-call spans. |
| `cmd/arcp` | Sample CLI with `serve` and `submit` subcommands. |

The runtime owns agent execution. Agents receive a `*server.JobContext`
for events, lease validation, budget debits, chunked results, and
credential rotation.

## Versioning

`arcp.ProtocolVersion` is the wire version. `arcp.SDKVersion` is the Go
module version advertised by default clients. Feature flags are
negotiated during `session.hello` and `session.welcome`; callers can
override the advertised set through client and server options.

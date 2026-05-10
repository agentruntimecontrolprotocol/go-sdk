# arcp-go

Go reference implementation of the Agent Runtime Control Protocol
(ARCP) v1.0. The canonical specification is `RFC-0001-v2.md` in this
directory.

This is the v0.1 cut. See `CONFORMANCE.md` for the implemented vs deferred
matrix and `PLAN.md` for the design rationale, message-type registry, and
test plan.

## Quickstart

Requires Go 1.25 or newer (see `go.mod`).

```bash
git clone <this repo>
cd go-sdk
make test          # run the full suite under -race
make build         # build library and CLI
go install ./cmd/arcp
arcp --help
```

Run an example:

```bash
go run ./examples/01_minimal_session
```

## Layout

- `envelope.go`, `ids.go`, `errors.go`, `extensions.go`, `trace.go` —
  protocol foundations.
- `messages/` — typed payloads, one file per message group.
- `runtime/` — server-side runtime (sessions, jobs, streams, leases,
  subscriptions, artifacts).
- `client/` — client wrapper.
- `transport/` — WebSocket, stdio, and in-memory transports.
- `store/` — SQLite event log.
- `auth/` — bearer and signed-JWT auth schemes.
- `cmd/arcp/` — CLI binary.
- `examples/` — runnable example programs.
- `tests/` — cross-package integration tests.

## Development

```bash
make fmt           # gofmt -w
make vet           # go vet
make lint          # golangci-lint
make test          # tests under -race
make cover         # coverage report
make gates         # full gate set (must pass before committing)
```

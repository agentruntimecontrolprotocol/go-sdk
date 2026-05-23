# CLI

The SDK ships a sample `arcp` binary under [`cmd/arcp`](../cmd/arcp/).
It is intended for local smoke tests and scripted protocol exercises;
for production embedding, prefer the `client`, `server`, and
`transport` packages directly so your application owns logging, auth,
lifecycle, and routing.

```sh
go install github.com/agentruntimecontrolprotocol/go-sdk/cmd/arcp@latest
# or
go run github.com/agentruntimecontrolprotocol/go-sdk/cmd/arcp <subcommand> [flags]
```

Two subcommands are wired:

| Subcommand | Purpose |
| --- | --- |
| `serve` | Runs an HTTP server that upgrades `/arcp` to ARCP-over-WebSocket and registers a single `echo` agent. |
| `submit` | Dials a runtime, submits one job, prints every event, and prints the final result. |

`arcp` uses Go's `flag` package, so flags are single-dash and a
subcommand is required before any flag is meaningful. Run `arcp serve
-h` or `arcp submit -h` for per-subcommand help. Running with no
arguments prints `usage: arcp <serve|submit> [flags]` and exits with
status 2.

## `arcp serve`

| Flag | Default | Meaning |
| --- | --- | --- |
| `-addr` | `:7777` | HTTP listen address. |
| `-path` | `/arcp` | WebSocket upgrade path. |

The handler is wired with the loopback-only `AllowedHosts` default
(`localhost`, `127.0.0.1`, `[::1]`). Requests with another `Host`
header are rejected with HTTP 421 (`StatusMisdirectedRequest`). To
expose the runtime beyond loopback, embed the server programmatically
and configure your own `nethttp.Options.AllowedHosts`.

## `arcp submit`

| Flag | Default | Meaning |
| --- | --- | --- |
| `-addr` | `ws://127.0.0.1:7777/arcp` | Runtime WebSocket URL. |
| `-agent` | `echo` | Agent identifier (optionally `name@version`). |
| `-input` | `{}` | JSON input payload. |
| `-token` | empty | Bearer token sent in `session.hello.auth.token`. |

The command exits non-zero on dial, connect, submit, or terminal
job-error failure, and zero on a job that ends `success`.

# ARCP Go SDK

[![License](https://img.shields.io/badge/license-Apache--2.0-blue)](./LICENSE)
[![Go version](https://img.shields.io/badge/go-%E2%89%A5%201.23-blue)](./go.mod)

Go SDK for [ARCP](../spec/docs/draft-arcp-02.1.md), the wire protocol
an agent uses to talk to the runtime that hosts it. Ships a client, a
server, transports for WebSocket / stdio / in-memory, OTel middleware,
and the `arcp` CLI.

## Install

```sh
go get github.com/agentruntimecontrolprotocol/go-sdk@v1.0.0
```

## Packages

| Import path                                                | Use when                                                                 |
| ---------------------------------------------------------- | ------------------------------------------------------------------------ |
| `.../go-sdk`                                               | Envelope, errors, ids, feature constants, lease types.                   |
| `.../go-sdk/client`                                        | Building a client that talks to a runtime.                               |
| `.../go-sdk/server`                                        | Hosting agents.                                                          |
| `.../go-sdk/transport`                                     | Custom transports, or pairing `MemoryTransport` in tests.                |
| `.../go-sdk/messages`                                      | Direct envelope construction (rare; clients and servers do it for you).  |
| `.../go-sdk/auth`                                          | The `Verifier` interface for bearer-token authentication.                |
| `.../go-sdk/middleware/nethttp`                            | Attaching the WS upgrade to an existing `*http.Server`.                  |
| `.../go-sdk/middleware/chi`                                | Mounting on a `chi.Router`.                                              |
| `.../go-sdk/middleware/otel`                               | W3C trace-context propagation per spec §11.                              |
| `.../go-sdk/cmd/arcp`                                      | `arcp serve` / `arcp submit` CLI.                                        |

## Core concepts

- **Envelope (§5).** Every wire message is `arcp:"1"` + `id` + `type`
  + `payload`, with optional session/job/event/trace ids. Unknown
  fields round-trip; v1.0 clients see future fields as opaque.
- **Session (§6).** `session.hello` → `session.welcome` (or
  `session.error`); closes on `session.bye` or transport close.
- **Job (§7).** `job.submit` → `job.accepted` → `job.event*` →
  terminal `job.result` ∣ `job.error`.
- **Lease (§9).** Capability namespace → glob patterns, immutable at
  submit. Optional `expires_at` and `cost.budget` add time and budget
  bounds.
- **Event (§8).** One `job.event` envelope, `payload.kind` ∈ the ten
  reserved values plus `x-vendor.*`.
- **Subscribe (§7.6).** Re-attach to a job from a different session.
  Subscribers observe; they cannot cancel.

## Quickstart

```go
package main

import (
    "context"
    "encoding/json"
    "log"

    "github.com/agentruntimecontrolprotocol/go-sdk/client"
    "github.com/agentruntimecontrolprotocol/go-sdk/server"
    "github.com/agentruntimecontrolprotocol/go-sdk/transport"
)

func main() {
    srv := server.New(server.Options{})
    srv.RegisterAgent("echo", func(ctx context.Context, input json.RawMessage, jc *server.JobContext) (any, error) {
        return map[string]json.RawMessage{"echo": input}, nil
    })

    a, b := transport.NewMemoryPair()
    ctx := context.Background()
    go srv.Accept(ctx, b)
    cli, err := client.Connect(ctx, a, client.Options{Token: "demo"})
    if err != nil {
        log.Fatal(err)
    }
    defer cli.Close(ctx)

    h, _ := cli.Submit(ctx, client.SubmitRequest{
        Agent: "echo",
        Input: map[string]string{"hi": "there"},
    })
    res, _ := h.Wait(ctx)
    log.Println("result:", string(res.Output))
}
```

## Capabilities

| Flag                | Section | What it gates                                              |
| ------------------- | ------- | ---------------------------------------------------------- |
| `heartbeat`         | §6.4    | `session.ping`/`session.pong`, watchdog                    |
| `ack`               | §6.5    | `session.ack`, back-pressure status emission               |
| `list_jobs`         | §6.6    | `session.list_jobs` / `session.jobs`                       |
| `subscribe`         | §7.6    | `job.subscribe` / `job.subscribed` / `job.unsubscribe`     |
| `agent_versions`    | §7.5    | `name@version` grammar; rich `agents` inventory            |
| `lease_expires_at`  | §9.5    | `lease_constraints.expires_at`                             |
| `cost.budget`       | §9.6    | `cost.budget` lease capability and runtime counters        |
| `progress`          | §8.2.1  | `progress` event kind                                      |
| `result_chunk`      | §8.4    | `result_chunk` event kind and streamed `job.result`        |

## Examples

See [`examples/`](./examples/) for one runnable directory per
scenario. Each has `server/main.go`, `client/main.go`, and a short
README citing the spec section it exercises.

## Conformance

The `tests/conformance` package emits a JSON summary against the spec
sections via `go test ./tests/conformance/...`. See
[CONFORMANCE.md](./CONFORMANCE.md).

## License

[Apache-2.0](./LICENSE).

<h3 align="center">ARCP Go SDK</h3>

<p align="center"><strong>Go SDK for the Agent Runtime Control Protocol (ARCP) — submit, observe, and control long-running agent jobs from Go.</strong></p>

<p align="center">
  <a href="https://pkg.go.dev/github.com/agentruntimecontrolprotocol/go-sdk"><img alt="Go reference" src="https://pkg.go.dev/badge/github.com/agentruntimecontrolprotocol/go-sdk.svg"></a>
  <a href="https://github.com/agentruntimecontrolprotocol/go-sdk/actions/workflows/test.yml"><img alt="CI" src="https://github.com/agentruntimecontrolprotocol/go-sdk/actions/workflows/test.yml/badge.svg"></a>
  <a href="https://codecov.io/gh/agentruntimecontrolprotocol/go-sdk"><img alt="codecov" src="https://codecov.io/gh/agentruntimecontrolprotocol/go-sdk/graph/badge.svg"></a>
  <a href="https://goreportcard.com/report/github.com/agentruntimecontrolprotocol/go-sdk"><img alt="Go report card" src="https://goreportcard.com/badge/github.com/agentruntimecontrolprotocol/go-sdk"></a>
  <a href="https://github.com/agentruntimecontrolprotocol/spec/blob/main/docs/draft-arcp-1.1.md"><img alt="ARCP" src="https://img.shields.io/badge/ARCP-v1.1%20draft-blue"></a>
  <a href="LICENSE"><img alt="License" src="https://img.shields.io/badge/license-Apache--2.0-lightgrey"></a>
</p>

<p align="center">
  <a href="https://github.com/agentruntimecontrolprotocol/spec/blob/main/docs/draft-arcp-1.1.md">Specification</a> ·
  <a href="#concepts">Concepts</a> ·
  <a href="#installation">Install</a> ·
  <a href="#quick-start">Quick start</a> ·
  <a href="docs/">Guides</a> ·
  <a href="https://pkg.go.dev/github.com/agentruntimecontrolprotocol/go-sdk">API reference</a>
</p>

---

`github.com/agentruntimecontrolprotocol/go-sdk` is the Go reference implementation of [ARCP](https://github.com/agentruntimecontrolprotocol/spec/blob/main/docs/draft-arcp-1.1.md), the Agent Runtime Control Protocol. It covers both sides of the wire — the `client` package for submitting and observing jobs, the `server` package for hosting agents — along with WebSocket, stdio, and in-memory transports, OTel and HTTP-router middleware, and the `arcp` CLI, so either side can talk to any conformant peer in any language without hand-rolling the envelope, sequencing, or lease handling.

ARCP itself is a transport-agnostic wire protocol for long-running AI agent jobs. It owns the parts of agent infrastructure that don't change between products — sessions, durable event streams, capability leases, budgets, resume — and stays out of the parts that do. ARCP wraps the agent function; it does not define how agents are built, how tools are exposed (that's MCP), or how telemetry is exported (that's OpenTelemetry).

## Installation

Requires Go 1.25 or newer (matching the `go` directive in `go.mod`). The module is fetched by the Go toolchain on first build; no separate install step is required.

```sh
go get github.com/agentruntimecontrolprotocol/go-sdk@latest
```

The CLI binary lives under `cmd/arcp` and is installable on its own with `go install github.com/agentruntimecontrolprotocol/go-sdk/cmd/arcp@latest`. Optional host integrations ship as sub-packages: `middleware/nethttp`, `middleware/chi`, and `middleware/otel`.

## Quick start

Connect to a runtime, submit a job, stream its events to completion:

```go
package main

import (
    "context"
    "encoding/json"
    "fmt"
    "log"
    "os"
    "time"

    "github.com/agentruntimecontrolprotocol/go-sdk/client"
    "github.com/agentruntimecontrolprotocol/go-sdk/transport"
)

func main() {
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    t, err := transport.DialWebSocket(ctx, "wss://runtime.example.com/arcp", transport.WebSocketOptions{})
    if err != nil {
        log.Fatal(err)
    }
    cli, err := client.Connect(ctx, t, client.Options{Token: os.Getenv("ARCP_TOKEN")})
    if err != nil {
        log.Fatal(err)
    }
    defer cli.Close(ctx)

    h, err := cli.Submit(ctx, client.SubmitRequest{
        Agent: "data-analyzer",
        Input: map[string]any{"dataset": "s3://example/sales.csv"},
    })
    if err != nil {
        log.Fatal(err)
    }
    go func() {
        for ev := range h.Events() {
            fmt.Printf("[%s] %s\n", ev.Kind, string(ev.Body))
        }
    }()
    res, err := h.Wait(ctx)
    if err != nil {
        log.Fatal(err)
    }
    out, _ := json.Marshal(res.Output)
    log.Println("final:", res.FinalStatus, string(out))
}
```

This is the whole shape of the SDK: open a session, submit work, consume an ordered event stream, get a terminal result or error. Everything below is detail on those four moves.

## Concepts

ARCP organizes everything around four concerns — **identity**, **durability**, **authority**, and **observability** — expressed through five core objects:

- **Session** — a connection between a client and a runtime. A session carries identity (a bearer token), negotiates a feature set in a `hello`/`welcome` handshake, and is *resumable*: if the transport drops, you reconnect with a resume token and the runtime replays buffered events. Jobs outlive the session that started them. See [§6](https://github.com/agentruntimecontrolprotocol/spec/blob/main/docs/draft-arcp-1.1.md).
- **Job** — one unit of agent work submitted into a session. A job has an identity, an optional idempotency key, a resolved agent version, and a lifecycle that ends in exactly one terminal state: `success`, `error`, `cancelled`, or `timed_out`. See [§7](https://github.com/agentruntimecontrolprotocol/spec/blob/main/docs/draft-arcp-1.1.md).
- **Event** — the ordered, session-scoped stream a job emits: logs, thoughts, tool calls and results, status, metrics, artifact references, progress, and streamed result chunks. Events carry strictly monotonic sequence numbers so the stream survives reconnects gap-free. See [§8](https://github.com/agentruntimecontrolprotocol/spec/blob/main/docs/draft-arcp-1.1.md).
- **Lease** — the authority a job runs under, expressed as capability grants (`fs.read`, `fs.write`, `net.fetch`, `tool.call`, `agent.delegate`, `cost.budget`, `model.use`). The runtime enforces the lease at every operation boundary; a job can never act outside it. Leases may carry a budget and an expiry, and may be subset and handed to sub-agents via delegation. See [§9](https://github.com/agentruntimecontrolprotocol/spec/blob/main/docs/draft-arcp-1.1.md).
- **Subscription** — read-only attachment to a job started elsewhere (e.g. a dashboard watching a job a CLI submitted). A subscriber observes the live event stream but cannot cancel or mutate the job. Distinct from *resume*, which continues the original session and carries cancel authority. See [§7.6](https://github.com/agentruntimecontrolprotocol/spec/blob/main/docs/draft-arcp-1.1.md).

The SDK models each of these as first-class objects; the rest of this README shows how.

## Guides

### Sessions and resume

Open a session, negotiate features, and reconnect transparently after a transport drop using the resume token — jobs keep running server-side while you're gone.

```go
ctx := context.Background()
t, err := transport.DialWebSocket(ctx, "wss://runtime.example.com/arcp", transport.WebSocketOptions{})
if err != nil {
    log.Fatal(err)
}
cli, err := client.Connect(ctx, t, client.Options{
    ClientName:    "resumable",
    ClientVersion: "1.0.0",
    Token:         os.Getenv("ARCP_TOKEN"),
})
if err != nil {
    log.Fatal(err)
}
defer cli.Close(ctx)

// The welcome payload carries the resume token, window, and effective
// negotiated feature set.
welcome := cli.Welcome()
fmt.Println("session:", cli.SessionID())
fmt.Println("resume window:", welcome.ResumeWindowSec, "s")
fmt.Println("features:", cli.Features())

// Subscriptions can re-anchor at a known sequence after a drop by setting
// SubscribeOptions.FromEventSeq, replaying every event with seq greater
// than that value.
```

After an unexpected drop, dial a fresh transport and present the welcome's resume block back. The runtime mints a new token, reuses the original session id, and replays buffered events with `event_seq > LastEventSeq`:

```go
prior := messages.ResumeRequest{
    SessionID:    cli.SessionID(),
    ResumeToken:  cli.Welcome().ResumeToken,
    LastEventSeq: cli.HighestSeq(),
}
t2, _ := transport.DialWebSocket(ctx, url, transport.WebSocketOptions{})
cli2, err := client.Connect(ctx, t2, client.Options{
    Token:  os.Getenv("ARCP_TOKEN"),
    Resume: &prior,
})
```

Graceful `Close` clears the resume state; only unexpected exits are resumable.

### Submitting jobs

Submit a job with an agent (optionally version-pinned as `name@version`), an input, and an optional lease request, idempotency key, and runtime limit.

```go
expires := time.Now().Add(time.Minute)
h, err := cli.Submit(ctx, client.SubmitRequest{
    Agent: "weekly-report@2.1.0",
    Input: map[string]string{"week": "2026-W19"},
    LeaseRequest: arcp.Lease{
        arcp.CapNetFetch: {"s3://reports/**"},
    },
    LeaseConstraints: &messages.LeaseConstraints{ExpiresAt: &expires},
    IdempotencyKey:   "weekly-report-2026-W19",
    MaxRuntimeSec:    300,
})
if err != nil {
    log.Fatal(err)
}

fmt.Println("job_id =", h.ID())
fmt.Println("effective lease =", h.Accepted().Lease)
fmt.Println("resolved agent =", h.Accepted().Agent)
```

### Consuming events

Iterate the ordered event stream — `log`, `thought`, `tool_call`, `tool_result`, `status`, `metric`, `artifact_ref`, `progress`, `result_chunk` — and optionally acknowledge progress so the runtime can release buffered events early.

```go
cli, err := client.Connect(ctx, t, client.Options{
    ClientName:      "ack-demo",
    Token:           os.Getenv("ARCP_TOKEN"),
    AutoAckWindow:   32,                      // coalesced session.ack
    AutoAckInterval: 250 * time.Millisecond,
})
if err != nil {
    log.Fatal(err)
}
defer cli.Close(ctx)

h, err := cli.Submit(ctx, client.SubmitRequest{Agent: "noisy"})
if err != nil {
    log.Fatal(err)
}

for ev := range h.Events() {
    switch ev.Kind {
    case messages.KindLog:
        var b messages.LogBody
        _ = json.Unmarshal(ev.Body, &b)
        fmt.Println(b.Level, b.Message)
    case messages.KindToolCall:
        fmt.Println("→ tool", string(ev.Body))
    case messages.KindMetric:
        fmt.Println("metric", string(ev.Body))
    case messages.KindProgress:
        fmt.Println("progress", string(ev.Body))
    }
    // Or ack manually: _ = cli.Ack(ctx, lastSeq)
}
```

### Leases and budgets

Request capabilities, a budget, and an expiry; read budget-remaining metrics as they arrive; handle the runtime's enforcement decisions.

```go
expires := time.Now().Add(10 * time.Minute)
h, err := cli.Submit(ctx, client.SubmitRequest{
    Agent: "web-research",
    Input: map[string]any{"iterations": 8, "perCallUSD": 0.3},
    LeaseRequest: arcp.Lease{
        arcp.CapToolCall:   {"search.*", "fetch.*"},
        arcp.CapCostBudget: {"USD:1.00"},
    },
    LeaseConstraints: &messages.LeaseConstraints{ExpiresAt: &expires},
})
if err != nil {
    log.Fatal(err)
}

fmt.Println("initial budget =", h.Accepted().Budget)

go func() {
    for ev := range h.Events() {
        if ev.Kind != messages.KindMetric {
            continue
        }
        var m messages.MetricBody
        if err := json.Unmarshal(ev.Body, &m); err != nil {
            continue
        }
        if m.Name == "cost.budget.remaining" {
            fmt.Printf("budget remaining: %.2f %s\n", m.Value, m.Unit)
        }
    }
}()

if _, err := h.Wait(ctx); err != nil {
    // BUDGET_EXHAUSTED or LEASE_EXPIRED is never retryable.
    log.Println("job ended:", err)
}
```

### Subscribing to jobs

Attach read-only to a job submitted elsewhere and observe its live stream (with optional history replay) without cancel authority.

```go
observer, err := client.Connect(ctx, t, client.Options{
    ClientName: "dashboard",
    Token:      os.Getenv("ARCP_TOKEN"),
})
if err != nil {
    log.Fatal(err)
}
defer observer.Close(ctx)

list, err := observer.ListJobs(ctx, client.ListJobsRequest{
    Filter: messages.ListJobsFilter{Status: []string{"running"}},
})
if err != nil {
    log.Fatal(err)
}

sub, err := observer.Subscribe(ctx, list.Jobs[0].JobID, client.SubscribeOptions{History: true})
if err != nil {
    log.Fatal(err)
}
fmt.Printf("subscribed job=%s status=%s agent=%s\n", sub.JobID(), sub.CurrentStatus(), sub.Agent())

for ev := range sub.Events() {
    fmt.Printf("[%s] %s\n", ev.Kind, string(ev.Body))
}
_ = sub.Close(ctx)
```

### Error handling

Catch the typed error taxonomy and respect the `retryable` flag — `LEASE_EXPIRED` and `BUDGET_EXHAUSTED` are never retryable; a naive retry fails identically.

```go
h, err := cli.Submit(ctx, client.SubmitRequest{Agent: "flaky"})
if err != nil {
    log.Fatal(err)
}
if _, err := h.Wait(ctx); err != nil {
    var arcpErr *arcp.Error
    if errors.As(err, &arcpErr) {
        switch arcpErr.Code {
        case arcp.CodeLeaseExpired, arcp.CodeBudgetExhausted:
            // Resubmit with a fresh lease / budget; never retry as-is.
            log.Fatalf("terminal: %s", arcpErr.Code)
        }
        if arcp.IsRetryable(err) {
            // Safe to retry with backoff (e.g. INTERNAL_ERROR, HEARTBEAT_LOST).
        }
    }
    log.Fatal(err)
}
```

## Feature support

ARCP features this SDK negotiates during the `hello`/`welcome` handshake:

| Feature flag | Status |
|---|---|
| `heartbeat` | Supported |
| `ack` | Supported |
| `list_jobs` | Supported |
| `subscribe` | Supported |
| `lease_expires_at` | Supported |
| `cost.budget` | Supported |
| `model.use` | Supported |
| `provisioned_credentials` | Supported |
| `progress` | Supported |
| `result_chunk` | Supported |
| `agent_versions` | Supported |

## Transport

ARCP is transport-agnostic. This SDK ships a WebSocket transport (default), an NDJSON-over-stdio transport for in-process child runtimes, and an in-memory transport for tests and same-process embedders. WebSocket is the default for networked runtimes; stdio is used for in-process child runtimes. Select one by constructing the corresponding `transport.Transport` (`transport.DialWebSocket(ctx, url, opts)`, `transport.NewStdioTransport(in, out)`, or `transport.NewMemoryPair()`) and passing it to `client.Connect(ctx, t, opts)`. Host integration: `middleware/nethttp.NewHandler(srv, opts)` builds an `http.Handler` you can mount on an `*http.Server` or `http.ServeMux`; `middleware/chi.Mount(router, srv, opts)` attaches the same handler to a `chi.Router`.

## API reference

Full API reference — every type, method, and event payload — is in [`docs/`](docs/) and at <https://pkg.go.dev/github.com/agentruntimecontrolprotocol/go-sdk>.

## Versioning and compatibility

This SDK speaks **ARCP v1.1 (draft)**. The SDK follows semantic versioning independently of the protocol; the protocol version it negotiates is shown above and in `session.hello`. A runtime advertising a different ARCP MAJOR is not guaranteed compatible. Feature mismatches degrade gracefully: the effective feature set is the intersection of what the client and runtime advertise, and the SDK will not use a feature outside it.

## Contributing

See [`CONTRIBUTING.md`](CONTRIBUTING.md). Protocol questions and proposed changes belong in the [spec repository](https://github.com/agentruntimecontrolprotocol/spec); SDK bugs and feature requests belong here.

## License

Apache-2.0 — see [`LICENSE`](LICENSE).

# mcp

An ARCP runtime that fronts an upstream MCP server. Inbound ARCP
`tool.invoke` envelopes translate into MCP `call_tool` calls, and
the result fans back as the ARCP job lifecycle.

## Before ARCP

MCP describes capabilities (one tool registry per server) but it has
no notion of multi-tenant identity, no leases, no durable jobs, no
trace propagation. Wrapping it directly in HTTP loses the structured
agent vocabulary; pretending it's a transport loses MCP's tool
catalog.

## With ARCP

```go
//   ARCP client ──tool.invoke──> bridge ──call_tool──> MCP server
//   ARCP client <─job.{accepted,started,completed,failed}─ bridge

c, _ := mcp.NewClient(...).Connect(ctx, transport, nil)
exts, _ := advertiseFromMCP(ctx, c)         // -> arcpx.mcp.tool.<name>.v1
for env := range inbound {
    if _, ok := env.Payload.(*messages.ToolInvoke); ok {
        go handleInvoke(ctx, send, c, env)  // wraps in job.* lifecycle
    }
}
```

Each upstream MCP tool becomes a namespaced ARCP capability extension
that clients can require at session open. The actual call surface is
the standard `tool.invoke` envelope.

## ARCP primitives

- §20 protocol bridge: ARCP `tool.invoke` ↔ MCP `call_tool`.
- §10 job lifecycle wrapped around each MCP call.
- §21.1 namespaced capability advertisement
  (`arcpx.mcp.tool.<name>.v1`).
- §18 canonical error mapping (MCP `isError: true` →
  `FAILED_PRECONDITION`).

## File tour

- `main.go` — `runBridge()`, `handleInvoke()`, `callViaMCP()`,
  `advertiseFromMCP()`.
- `upstream.go` — MCP transport stub + content-block flattener.

## Variations

- Promote MCP `resources` to ARCP `kind: event` streams (§11).
- Layer leases (see [leases](../leases)) on top of `tool.invoke`
  before forwarding so high-risk MCP tools require per-call
  approval.
- Bridge multiple MCP servers in one runtime; route by tool prefix.

> Note: the `github.com/modelcontextprotocol/go-sdk` import is
> aspirational. Until vendored, treat it as illustrative.

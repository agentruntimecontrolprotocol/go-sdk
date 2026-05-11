# capability_negotiation

Pick the cheapest peer that meets latency + cost ceilings, walk an
ordered fallback chain on retryable errors, and roll up `tokens.used`
+ `cost.usd` metrics into per-tenant totals.

## Before ARCP

A LiteLLM-style router with a hand-coded JSON file of model
properties. Cost is reconciled out-of-band by a billing batch job
that diverges from runtime decisions.

## With ARCP

```go
profiles[name] = profileFrom(c.Capabilities, exts) // marketplace fields
chain := candidateChain(profiles, "balanced")
reply, _ := invokeWithFallback(ctx, clients, chain,
    "chat.completion", args, traceID)
// metric envelopes feed totals.consume() in the background.
```

The `arcpx.market.cost_per_mtok.v1` / `p50_latency_ms.v1` /
`model_class.v1` keys ride on the negotiated `capabilities`
extension blob. No extra round-trip to learn pricing.

## ARCP primitives

- `capabilities.extensions` — RFC §7, §21.
- Standard `metric` envelopes (`tokens.used`, `cost.usd`) with
  `dims.peer`, `dims.tenant`, `dims.kind` — §17.3.1, §18.3.
- Retryable error semantics (`RESOURCE_EXHAUSTED`, `UNAVAILABLE`,
  `DEADLINE_EXCEEDED`, `ABORTED`).
- Custom extension `arcpx.market.peer.v1` echoed back on the reply
  for cost attribution.

## File tour

- `main.go` — chain selection, fallback walker, metric rollup.
- `peers.go` — `openPeer()` stub + `Session` shim.

## Variations

- Add a hedging mode: fire two peers in parallel after `p50_latency_ms`,
  cancel the loser via [cancellation](../cancellation).
- Surface the per-tenant rollup via [subscriptions](../subscriptions)
  to a billing dashboard.
- Promote `arcpx.market.cost_per_mtok.v1` into a core capability
  field once the marketplace stabilizes (RFC §21.5 graduation path).

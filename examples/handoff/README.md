# handoff

Try the cheap tier first. If confidence is below threshold, package
the transcript as an artifact and emit `agent.handoff` to the deep
tier with the runtime identity pinned by fingerprint.

## Before ARCP

The "router" is a fragile classifier in front of the LLM gateway.
The deep tier sees only the final user prompt — none of the cheap
tier's reasoning, none of the false starts. Re-asking is expensive
and degrades quality.

## With ARCP

```go
ref, _ := packageContext(ctx, cheap, transcript) // → artifact.ref
cheap.Send(ctx, &arcp.Envelope{
    Payload: &messages.AgentHandoff{
        TargetRuntime: messages.RuntimeIdentity{
            Kind: deepKind, Fingerprint: deepFingerprint,
        },
        SessionID: cheap.SessionID(),
    },
    Extensions: map[string]json.RawMessage{
        "arcpx.handoff.shared_memory_ref.v1": refBytes,
    },
})
```

Transcript travels as an artifact (one upload, many references). The
deep runtime is pinned by fingerprint per RFC §8.3 — if it's been
swapped under us, the handoff fails closed.

## ARCP primitives

- `agent.handoff` — RFC §14.
- `artifact.put` / `artifact.ref` for inline transcript packaging — §16.
- Runtime fingerprint pinning — §8.3.
- Custom extension `arcpx.handoff.shared_memory_ref.v1` per §21.

## File tour

- `main.go` — confidence gate + artifact upload + handoff emit.
- `cheap.go` — cheap-tier `attempt()` stub + `Session` shim that
  surfaces the runtime's `session.accepted` for fingerprint pinning.

## Variations

- Use streaming-thoughts on the cheap side (see [leases](../leases))
  and pack the full thought stream — not just the final answer — into
  the artifact.
- Replace the boolean threshold with a router LLM that emits
  `confidence` + `recommended_tier` together.

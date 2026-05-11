# reasoning_streams

The primary agent emits its `kind: thought` reasoning over a stream.
A peer "mirror" runtime subscribes to that stream, runs a critic LLM
over each thought, and feeds critiques back via `agent.delegate`.
The primary folds the critique into its next step.

## Before ARCP

Reasoning is a stdout `print()` and a critic is "open a second
shell, paste the trace by hand". You can't filter, you can't budget,
you can't audit who saw what.

## With ARCP

```go
// primary
c.Send(ctx, &arcp.Envelope{
    StreamID: streamID,
    Payload: &messages.StreamChunk{
        Sequence: step,
        Role:     "assistant_thought",
        Content:  answer,
    },
})

// mirror
mirror.Request(ctx, &arcp.Envelope{
    Payload: &messages.Subscribe{
        Filter: messages.SubscribeFilter{
            SessionID: []arcp.SessionID{target},
            Types:     []string{"stream.chunk"},
        }}})
// ... per-thought critic call → agent.delegate back to primary ...
```

The mirror enforces a `tokenBudget` and unsubscribes cleanly when
spent — the runtime stops paying for events the mirror won't act on.

## ARCP primitives

- `kind: thought` streams — RFC §11.4.
- Subscriptions with `types` filter for `stream.chunk` — §13.
- `agent.delegate` for back-channel critique — §14.
- Clean unsubscribe on budget exhaustion — §13.4.

## File tour

- `main.go` — `runPrimary()` + `runMirror()` wired through an
  in-process critique channel.
- `agents.go` — `primaryStep()` / `critiqueThought()` stubs +
  `Session` shim.

## Variations

- Multiple mirrors with disjoint critique specialties (style /
  factuality / safety) — each subscribes with a finer filter.
- Persist the thought stream into [subscriptions](../subscriptions)'
  SQLite sink for postmortem.
- Add a `severity: halt` short-circuit that emits `cancel` (see
  [cancellation](../cancellation)) on the primary's job instead of
  letting the critique loop terminate naturally.

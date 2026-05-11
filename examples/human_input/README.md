# human_input

A relay that receives `human.input.request` from any session and
fans the question across phone push (ntfy), pager email, and a
Slack DM concurrently. First channel to answer wins; the rest
receive `human.input.cancelled` so the user-visible message is
"settled" rather than abandoned.

## Before ARCP

Either pick one channel up-front (and miss responsiveness when the
oncall isn't watching it) or fan out manually with no protocol
contract for "settled elsewhere". Human stale-state proliferates.

## With ARCP

```go
for env := range c.Events(ctx) {
    if _, ok := env.Payload.(*messages.HumanInputRequest); ok {
        go fanOut(ctx, c, env)
    }
}
// fanOut: race adapters; first winner emits human.input.response,
// losers receive human.input.cancelled with reason="answered elsewhere".
```

## ARCP primitives

- `human.input.request` / `.response` / `.cancelled` — RFC §12.
- Deadline carried by `expires_at`; deadline-elapsed maps to
  `DEADLINE_EXCEEDED` per §12.4.
- First-wins resolution; explicit `cancelled` to the losers.
- Custom extension `arcpx.humaninput.cancelled_channels.v1` for
  postmortem reconciliation.

## File tour

- `main.go` — `fanOut()` driver wired to `c.Events()`.
- `channels.go` — three adapter stubs + registry + `Session` shim.

## Variations

- Pair with [cancellation](../cancellation)'s `interrupt` scenario
  for the full pause-and-ask loop.
- Add `human.choice.request` for multiple-choice questions and
  collect ranked-choice answers across channels.
- Replace ntfy with PagerDuty / Twilio / iMessage; the relay
  contract is identical.

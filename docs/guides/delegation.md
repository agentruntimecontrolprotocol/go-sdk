# Delegation (§10)

Delegation creates a child job under a narrowed lease. The Go SDK
exposes the subset rule as a public helper so callers can verify a
proposed child lease before issuing it:

```go
err := arcp.IsLeaseSubset(parentLease, childLease, parentRemaining, parentExpiry, childExpiry)
```

`parentRemaining` is a `map[arcp.Currency]float64` of the parent's
remaining per-currency budget at the moment of the check; for a
freshly issued lease this equals the initial budget. `parentExpiry`
and `childExpiry` are optional. A non-nil return is `*arcp.Error`
with code `LEASE_SUBSET_VIOLATION` (or an underlying parse error for
a malformed budget string).

Child leases may not add capabilities, widen patterns, increase
budgets, or extend expiration beyond the parent. Use
`arcp.CapAgentDelegate` patterns on the parent lease to control which
child agents can be spawned. Provisioned child credentials should be
issued from the child lease so they revoke with the child lifecycle.

## Known limitation: no public sub-job API

The Go SDK does not yet expose a runtime-mediated way for an agent
to submit a child job from inside its `AgentFunc`. The `delegate`
event kind exists on the wire and `arcp.IsLeaseSubset` will validate
child leases, but there is no `JobContext.Delegate` or equivalent
method that opens a new envelope to the runtime on behalf of the
agent.

Today, runtime authors who need delegation can:

1. Have the parent agent emit a `delegate` event (via
   `messages.DelegateBody`) so observers know a child is intended.
2. Open a second `client.Client` from inside the agent process and
   submit the child job under the (subset-verified) child lease.
3. Stitch the child's job id and trace context into the parent's
   metric/log events for traceability.

A first-class `JobContext.Delegate` API is on the roadmap; see the
[multi-agent-budget recipe](../../recipes/multi-agent-budget/) for
the pattern in use today.

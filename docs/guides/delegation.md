# Delegation (§10)

Delegation creates a child job under a narrowed lease. The Go runtime
uses the same lease subset helper as direct job submission:

```go
err := lease.IsSubset(parentLease, childLease, parentBudget, parentExpiry, childExpiry)
```

Child jobs may not add capabilities, widen patterns, increase budgets,
or extend expiration beyond the parent. Violations map to
`LEASE_SUBSET_VIOLATION`.

Use `agent.delegate` patterns to control which child agents can be
spawned. Provisioned child credentials should be issued from the child
lease so they revoke with the child lifecycle.

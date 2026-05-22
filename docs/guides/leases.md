# Leases (§9)

A lease maps capability namespaces to glob patterns:

```go
lease := arcp.Lease{
	arcp.CapNetFetch:      {"https://api.example.com/**"},
	arcp.CapToolCall:      {"search.*"},
	arcp.CapAgentDelegate: {"summarizer@*"},
	arcp.CapModelUse:      {"tier-fast/*"},
}
```

Agents validate operations through `JobContext.ValidateLeaseOp` before
the runtime or tool performs the action:

```go
if err := jc.ValidateLeaseOp(arcp.CapModelUse, "tier-fast/gpt-4o-mini"); err != nil {
	return nil, err
}
```

## `model.use` (§9.7)

`model.use` grants access to model profiles or upstream model names.
Patterns are globbed like other lease targets and are intentionally not
canonicalized.

Use it when the runtime is in the path of LLM invocation. A miss returns
`PERMISSION_DENIED`. Child jobs must request a subset of the parent
model patterns or the runtime returns `LEASE_SUBSET_VIOLATION`.

## Expiration and budgets

`LeaseConstraints.ExpiresAt` ends the lease at a fixed UTC time. The
runtime emits `LEASE_EXPIRED` if the timer fires while the job is
running.

`cost.budget` uses entries such as `USD:5.00`. The runtime tracks
remaining budget and emits budget-remaining metrics as cost metrics are
reported.

## Provisioned credentials (§9.8)

Configure `server.Options.Provisioner` to mint credentials after the
lease is finalized:

```go
srv := server.New(server.Options{
	Provisioner: credentials.NewMemory("cred-"),
})
```

When the client negotiates `provisioned_credentials`, `job.accepted`
includes `Credentials`. Each credential carries its ID, scheme, secret
value, endpoint, and constraints derived from `cost.budget`,
`model.use`, and `expires_at`.

The runtime revokes attached credentials on terminal states and exposes
`JobContext.RotateCredential` for mid-job rotation events.

# lease_revocation

A warehouse DB admin agent. Pre-grant the broad SELECTs at session
open; prompt the operator on every UPDATE / DELETE / DDL. Inbound
`lease.revoked` invalidates the cache so the next statement on that
table re-prompts.

## Before ARCP

The agent has a long-lived DB password and runs whatever the LLM
emits. Auditors get a query log and no notion of "who said yes to
this UPDATE?". The only recourse to a misbehaving agent is rotating
the password — there's no way to revoke just one capability.

## With ARCP

```go
v, _ := requestLease(ctx, c, "db.read", "public.orders", "read",
    "bootstrap", readLeaseSeconds)
cache.put(cacheKey{"public.orders", "read"}, v)
// ... a `lease.revoked` envelope arrives ...
cache.revoke(rv.LeaseID)  // next SELECT re-prompts the operator
```

Per-table lease scoping means an operator can revoke `db.write` on
one table without disturbing the other 50.

## ARCP primitives

- `permission.request` → `lease.granted` / `permission.deny` — RFC §15.4.
- `lease.revoked` mid-flight invalidation — §15.5.
- `lease.extended` for opportunistic refresh (sketched, not run).
- `db.read` / `db.write` permission strings.

## File tour

- `main.go` — bootstrap leases + `authorize()` per statement +
  inbound revocation drain.
- `sql.go` — `classify()` stub + `Session` shim.

## Variations

- Replace the LeaseCache with Redis for fleet-shared lease state.
- Pair with [permission_challenge](../permission_challenge) for a
  reviewer that gates writes on diff content rather than table.
- Send `lease.refresh` proactively when `expiresAt - now < 30s`.

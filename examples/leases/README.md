# leases

Sandboxed on-call agent. Every shell mutation is gated by a one-shot
lease scoped to the specific binary + target. Read-only commands run
under a long-lived `host.read` lease.

## Before ARCP

Two ad-hoc paths in the wild: (1) the agent has shell and just runs
things — operator finds out from `last`. (2) every command goes
through a Slack approval bot that the agent learns to game by
splitting destructive calls into innocuous-looking pairs. Neither
gives the operator a typed contract over what was approved.

## With ARCP

```go
reply, _ := c.Request(ctx, &arcp.Envelope{
    Payload: &messages.PermissionRequest{
        Permission:            "host.write",
        Resource:              "host:edge-pod-04/usr/bin/systemctl/api-gateway",
        Operation:             "restart",
        Reason:                "service is OOMing every 4 minutes",
        RequestedLeaseSeconds: 60,
    },
})
// reply is *messages.LeaseGranted (operator approved) or *messages.PermissionDeny.
```

The lease is scoped to `(binary, target)` so the agent can't reuse a
`restart api-gateway` grant to `restart database`.

## ARCP primitives

- Permission challenge — RFC §15.4.
- Lease lifecycle (request → grant → use → revoke) — §15.5.
- `kind: thought` reasoning stream — §11.4.
- `PERMISSION_DENIED`, `LEASE_EXPIRED`, `LEASE_REVOKED` — §18.2.
- Trust level `constrained` advertised in identity — §15.3.

## File tour

- `main.go` — opens session, runs the agent's tool loop.
- `agent.go` — LLM step generator stub + `Session` shim.

## Variations

- `trust.elevate.privileged` flow for the once-a-quarter
  `iptables -F` (§15.6) — same primitive, different permission.
- Replace operator approval with a policy engine (OPA, Cedar) — the
  responder is interchangeable as long as it emits `lease.granted`.
- Mirror the `kind: thought` stream into [subscriptions](../subscriptions)
  for postmortem replay.

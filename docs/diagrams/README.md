# Diagrams

Source `.dot` files for the diagrams that ship with the Go SDK docs.
Run `./render.sh` to produce paired `*-light.svg` and `*-dark.svg`
next to each source. `make diagrams` from the repo root invokes this.

## Set

1. `architecture.dot` — top-level component layout: client, transports, middleware, server, and the runtime's auth/credentials/eventlog/idstore dependencies. Embedded on the front page and in [architecture.md](../architecture.md).
2. `session-lifecycle.dot` — session FSM, including the resume edge that reuses the prior `session_id` and rotates the `resume_token`. Embedded in [guides/sessions.md](../guides/sessions.md) and [guides/resume.md](../guides/resume.md).
3. `job-lifecycle.dot` — `pending → running → terminal` with the lease, budget, and timeout edges that drive each terminal status. Embedded in [guides/jobs.md](../guides/jobs.md).
4. `lease-and-budget-enforcement.dot` — `ValidateOp` decision tree (expiry → glob → budget → proceed; `cost.*` metrics debit on the proceed path). Embedded in [guides/leases.md](../guides/leases.md).

## Conventions

- Two-anchor palette: blue (`#3B82F6`) for entry, amber (`#F59E0B`)
  for hub.
- Shapes encode meaning: `box` rounded for components/states,
  `diamond` for guards, `note` for invariants, dashed edges for
  asynchronous returns.
- Edge labels carry the spec section in parens, e.g. `job.subscribe (§7.6)`.

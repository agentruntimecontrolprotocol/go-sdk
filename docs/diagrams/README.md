# Diagrams

Source `.dot` files for the spec diagrams. Run `./render.sh` to
produce paired `*-light.svg` and `*-dark.svg` next to each source.
`make diagrams` from the repo root invokes this.

## Set

1. `package-graph.dot` — module dependency graph.
2. `session-lifecycle.dot` — session FSM with resume edge.
3. `job-lifecycle.dot` — pending → running → terminal with lease/budget edges.
4. `capability-negotiation.dot` — hello/welcome intersection.
5. `subscribe-attach-flow.dot` — cross-session attach + permission boundary.
6. `heartbeat-flow.dot` — ping/pong + watchdog HEARTBEAT_LOST.
7. `result-chunk-flow.dot` — agent → runtime → client streamed result.
8. `lease-and-budget-enforcement.dot` — validateLeaseOp decision tree.

## Conventions

- Two-anchor palette: blue (`#3B82F6`) for entry, amber (`#F59E0B`)
  for hub.
- Shapes encode meaning: `box` rounded for components/states,
  `diamond` for guards, `note` for invariants, dashed edges for
  asynchronous returns.
- Edge labels carry the spec section in parens, e.g. `job.subscribe (§7.6)`.

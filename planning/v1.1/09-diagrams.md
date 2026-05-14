# 09 — Graphviz Diagrams for ARCP v1.1 Go SDK

This file plans the diagram set that ships alongside the v1.1 Go SDK. It
mirrors the layout used by `../typescript-sdk/diagrams/` (paired
light/dark SVGs rendered from `.dot` sources, two-anchor palette,
single-line labels) so a reader who has already seen the TypeScript set
recognises the conventions immediately. No `.dot` files are written in
this pass; this file defines what each diagram contains, where it lives,
what spec section grounds it, and who reads it.

Source location: `docs/diagrams/` in the `go-sdk` module. Each diagram
ships as `<name>.dot` plus two rendered SVGs `<name>-light.svg` and
`<name>-dark.svg`. The rendering pipeline is in
[Render pipeline](#render-pipeline) below.

## Diagram set

Minimum six. The plan ships eight. Every entry cites at least one spec
section and names the reader role it serves.

| # | filename | what it shows | spec § | reader role |
|---|----------|---------------|--------|-------------|
| 1 | `package-graph.dot` | Go package dependency graph at the `go-sdk` module: `arcp` root, `client`, `server`, `transport`, `messages`, `auth`, `middleware/{nethttp,chi,otel}`, `internal/{lease,eventlog,version,features}`. Edges = imports. Acyclic by construction (Go forbids cycles); the diagram exists to show what depends on what so reviewers can sanity-check the boundary between public packages and `internal/`. | §3, §5.1 | new Go contributor; reviewer auditing package surface area |
| 2 | `session-lifecycle.dot` | Session FSM: `init → hello-sent → welcome-received → active → {closed-by-bye, closed-by-error, dropped}`. Resume edge from `dropped` back to `active` with a rotated `resume_token`. Heartbeat watchdog timer is shown as a decision diamond on the `active` state. | §6.2, §6.3, §6.4, §6.7 | SDK consumer wiring up `Client` connect/reconnect; runtime author implementing session table |
| 3 | `job-lifecycle.dot` | Job FSM with v1.1 additions: `pending → running → {success, error, cancelled, timed_out}`. v1.1 error scenarios `LEASE_EXPIRED` and `BUDGET_EXHAUSTED` shown as labeled `running → error` transitions. Lease + budget gates rendered as a decision diamond on the `running → running` self-loop labelled "any leased op (§9.3, §9.6)". | §7.3, §7.4, §9.5, §9.6, §12 | tool author understanding why an op can be rejected mid-job; runtime author wiring `validateLeaseOp` |
| 4 | `subscribe-attach-flow.dot` | Sequence: dashboard session `C2` sends `session.list_jobs`, receives `session.jobs`, sends `job.subscribe`, receives `job.subscribed`, then receives live events from a job owned by session `C1`. Cancel attempt from `C2` returns `PERMISSION_DENIED`. `C1` continues to receive its own event stream throughout. | §6.6, §7.6, §13.3 | dashboard / observer-app author; reviewer checking that non-owner sessions cannot cancel |
| 5 | `capability-negotiation.dot` | Sequence: client `hello{capabilities.features:[…]}` → runtime replies `welcome{capabilities.features:[…], agents, heartbeat_interval_sec}`. A note node shows the intersection set the SDK exposes via `Session.HasFeature`. Two side notes anchor where a feature outside the intersection is rejected: client helpers return `INVALID_REQUEST`; runtime handler tables refuse dispatch. | §6.2 | SDK consumer wondering why a method returned `INVALID_REQUEST`; protocol implementer in another language |
| 6 | `heartbeat-flow.dot` | Sequence with three actors (client, runtime, watchdog timer): idle → `session.ping` → `session.pong` → idle → `session.ping` → no pong within `2 × heartbeat_interval_sec` → `HEARTBEAT_LOST` raised → transport closed. Side note: jobs survive on the runtime side and are recoverable via resume (cross-references diagram 2). | §6.4, §13.1 | SDK consumer tuning heartbeat interval; on-call engineer triaging a dropped-connection incident |
| 7 | `result-chunk-flow.dot` | Sequence: agent emits `progress` events, then `result_chunk[seq=0]`, `result_chunk[seq=…]`, `result_chunk[seq=N, more:false]`, then `job.result{result_id, result_size}`. Client side shown reassembling chunks keyed by `result_id`. Note: agent MUST NOT mix inline result with chunks (§8.4). | §8.4, §13.6 | tool author streaming large outputs; client author implementing the reassembly buffer |
| 8 | `lease-and-budget-enforcement.dot` | Flowchart for `validateLeaseOp(ctx, op, leases, now)`: three decision diamonds in series — (a) capability glob match? else `PERMISSION_DENIED`; (b) `now < expires_at`? else `LEASE_EXPIRED`; (c) all per-currency counters > 0? else `BUDGET_EXHAUSTED`. Success path leads to "proceed; debit counters on subsequent `cost.*` metrics". | §9.3, §9.5, §9.6, §12 | runtime author implementing the guard; security reviewer tracing every rejection path |

## Reader-role coverage

Every reader role the v1.1 docs target is reached by at least one
diagram:

- **New Go contributor / package surface reviewer** — diagram 1.
- **SDK consumer building a client** — diagrams 2, 5, 6, 7.
- **Dashboard / observer app author** — diagrams 4, 6.
- **Tool / agent author** — diagrams 3, 7, 8.
- **Runtime implementer** — diagrams 2, 3, 4, 8.
- **Security reviewer** — diagrams 3, 8 (denial paths) and 4 (cancel
  permission boundary).
- **Protocol implementer in another language** — diagrams 5, 6, 7
  (wire-level sequences).

## Shared style conventions

These apply to every diagram in the set. They match the TypeScript
diagram set so a reader who has seen one set knows the other.

- **Two-tone rendering.** Each `.dot` file produces two SVGs:
  `<name>-light.svg` and `<name>-dark.svg`. The dark variant is rendered
  by re-invoking `dot` with `-Gbgcolor=...` plus node/edge color
  overrides; the source `.dot` carries the light palette inline and the
  dark variant is generated, not hand-maintained. This avoids the
  TypeScript repo's drift hazard where someone edits `*-light.dot` and
  forgets the `*-dark.dot`.
- **Shapes encode meaning, not colour.** Grayscale-print clean.
  - `box` (rounded) — components, FSM states.
  - `ellipse` — wire messages (envelope `type` values).
  - `diamond` — decision / guard.
  - `note` — invariant / cross-reference.
  - `cylinder` — persistent state (e.g. event log, lease store).
- **Two anchors max per diagram.** One ENTRY (blue `#3B82F6`) and one
  HUB (amber `#F59E0B`); every other node uses defaults. Inherited
  directly from the TS palette.
- **Font.** `Helvetica` locked at the graph level. No mixed fonts.
- **Rank direction.** `LR` for sequence and flow diagrams (4, 5, 6, 7,
  8). `TB` for FSMs (2, 3) and the package graph (1).
- **Edge labels carry spec section.** Format
  `session.hello (§6.2)` — the wire-type token, then a parenthesised
  section. Decision-diamond edges use `yes` / `no` plus the rejection
  code where relevant (e.g. `no → LEASE_EXPIRED`).
- **Single-line centred labels.** No subtitle stacks. If a node needs
  context, put it on the cluster, not on the node.
- **Background.** `bgcolor="transparent"` on both variants so the SVGs
  sit on any markdown page background.
- **No semantic colour.** The two anchor colours are emphasis, not
  meaning. Failure paths are distinguished by shape (diamond → labelled
  edge to a `box` carrying the error code) and by label text, not by
  red ink. Pink dashed edges are reserved for async/feedback return
  paths and carry their own label (`return`, `replay`, etc.).

The TypeScript set's two `diagram-template-*.dot` files are the
starting point. The Go set copies them into `docs/diagrams/` verbatim
on day one and only diverges where this document calls it out (the
dark-variant generation rule above).

## Render pipeline

Layout:

```
go-sdk/
  docs/
    diagrams/
      diagram-template-light.dot
      diagram-template-dark.dot      # kept for parity with TS set
      package-graph.dot
      session-lifecycle.dot
      job-lifecycle.dot
      subscribe-attach-flow.dot
      capability-negotiation.dot
      heartbeat-flow.dot
      result-chunk-flow.dot
      lease-and-budget-enforcement.dot
      *-light.svg                    # rendered
      *-dark.svg                     # rendered
      render.sh
      diagrams.lock                  # content hashes
      README.md                      # palette + embedding snippet
```

`render.sh` walks every `*.dot` (excluding the two template files) and
for each emits the light and dark SVG:

```sh
dot -Tsvg "$src" -o "${name}-light.svg"
dot -Tsvg -Gbgcolor=transparent \
    -Nfillcolor="#334155" -Ncolor="#475569" -Nfontcolor="#F1F5F9" \
    -Ecolor="#94A3B8" -Efontcolor="#94A3B8" \
    "$src" -o "${name}-dark.svg"
```

(Final palette token list lives in the diagrams README — exact hex
values mirror the TS palette table; reproduced above as illustration.)

Make target:

```
diagrams: ## render docs/diagrams/*.dot to paired SVGs
	@bash docs/diagrams/render.sh
```

CI check (in the existing GitHub Actions workflow):

1. `make diagrams` runs.
2. The job recomputes a content hash of every `.dot` plus its two
   rendered SVGs and writes them to `docs/diagrams/diagrams.lock`.
3. `git diff --exit-code docs/diagrams/diagrams.lock` fails the PR if
   the committed lockfile is stale.

Hazard, named so it appears in the contributor docs and the PR
template: **SVGs go stale if a contributor edits a `.dot` and forgets
to re-render.** The `diagrams.lock` check is the only thing standing
between a fresh `.dot` and an out-of-date SVG on the rendered docs
site. The PR template gains a checkbox "I ran `make diagrams` if I
touched `docs/diagrams/`".

Every `.dot` file carries the standard Apache-2.0 header used by the Go
source. Graphviz strips C-style comments (`/* ... */`) at parse time so
the header sits at the top of each file without affecting rendering:

```
/*
 * Copyright (c) 2026 The ARCP Authors
 * Licensed under the Apache License, Version 2.0 (the "License").
 * SPDX-License-Identifier: Apache-2.0
 */
digraph G { ... }
```

## What is not diagrammed

Stated up front so reviewers don't ask:

- **No standalone "architecture overview" diagram.** `package-graph.dot`
  is the closest thing the Go module has, and it is derived from
  imports — a hand-drawn architecture box-and-arrows view would either
  duplicate the package graph or drift from it. The architecture prose
  lives in `04-architecture.md`; the package graph is its visual.
- **No delegation flow diagram.** Delegation in v1.1 is a job
  (`job-lifecycle.dot`, diagram 3) whose lease is a subset of its
  parent's, validated by the lease/budget guard
  (`lease-and-budget-enforcement.dot`, diagram 8). A third diagram
  would be the composition of those two with no information gain;
  delegation is covered as prose in `04-architecture.md` and
  `06-examples.md`.
- **No wire-envelope shape diagram.** The envelope is a documented JSON
  schema (`arcp`, `id`, `type`, `session_id`, `trace_id?`, `job_id?`,
  `event_seq?`, `payload`); it is not a graph. The schema lives in the
  spec; the Go SDK references it from `messages/envelope.go`.
- **No transport-layer diagram.** The transport is one of {WebSocket,
  stdio, in-memory} chosen by config; sequences in diagrams 4–7 are
  transport-agnostic and use the wire types directly. A transport
  picker diagram would carry one node per option and one decision
  diamond and tell the reader nothing the README doesn't already.

## Read time

Eight diagrams plus this plan target a ≤8 minute first read for someone
who already knows ARCP v1.0. A reader unfamiliar with v1.0 should read
the spec first; the diagrams illustrate the v1.1 deltas in context, not
the whole protocol.

# 08 — Docs Site and README

Plan for the markdown that the shared docs site ingests from
`go-sdk/docs/`, the doc-comment contract that feeds `pkg.go.dev`, and
the top-level `go-sdk/README.md`. No Go code in this pass.

Inputs read for this plan: `../../spec/docs/draft-arcp-02.1.md`,
`01-spec-delta.md`, `02-current-audit.md`,
`../../typescript-sdk/README.md`, `../../typescript-sdk/docs/`. The
shared docs site reads frontmatter verbatim; field names match TS so
the renderer needs zero per-SDK branches.

## `docs/` tree

```
docs/
  00-overview.md
  01-quickstart.md
  02-concepts.md
  03-features/
    heartbeat.md
    ack.md
    list-jobs.md
    subscribe.md
    agent-versions.md
    lease-expires-at.md
    cost-budget.md
    progress.md
    result-chunk.md
  04-examples/
    submit-and-stream.md
    delegate.md
    resume.md
    idempotent-retry.md
    lease-violation.md
    cancel.md
    stdio.md
    vendor-extensions.md
    custom-auth.md
    heartbeat.md
    ack-backpressure.md
    list-jobs.md
    subscribe.md
    agent-versions.md
    lease-expires-at.md
    cost-budget.md
    progress.md
    result-chunk.md
    tracing.md
  05-reference/
    client.md
    server.md
    transport.md
    messages.md
    errors.md
    middleware-otel.md
  06-conformance.md
```

### Top-level files

| File                    | Covers                                                                                | Target reader                                | Spec §s                                |
| ----------------------- | ------------------------------------------------------------------------------------- | -------------------------------------------- | -------------------------------------- |
| `00-overview.md`        | What ARCP v1.1 is, what this SDK ships, package map, links to spec.                   | Newcomer arriving from search.               | §1, §3, §15                            |
| `01-quickstart.md`      | Single-process server + client over MemoryTransport; mirror of README quickstart, 40 lines, references `Example_quickstart` in `arcp/example_test.go`. | Newcomer.                                    | §4.3, §6.2, §7.1, §8.2                 |
| `02-concepts.md`        | One paragraph each: envelope, session, job, event, lease, delegation, resume, capability negotiation. Cross-links to features and `pkg.go.dev`. | Port-from-TS engineer.                       | §5, §6, §7, §8, §9, §10, §6.2          |
| `06-conformance.md`     | Section-by-section status mirror of `CONFORMANCE.md`. Generated from the conformance harness output (see `07-tests.md`). | pkg.go.dev cross-link target; compliance.    | All §4–§15                             |

### `03-features/` (one page per v1.1 capability)

Each page is independently navigable: a search hit for "heartbeat go
sdk" lands on `heartbeat.md` and that page alone explains how to turn
the feature on, what it does on the wire, and what error sentinels it
can raise. No prose like "as we saw in §X".

| File                  | Covers                                                                                    | Target reader                | Spec §s    | Negotiation flag       |
| --------------------- | ----------------------------------------------------------------------------------------- | ---------------------------- | ---------- | ---------------------- |
| `heartbeat.md`        | Ticker config, `session.ping`/`session.pong`, watchdog, `HEARTBEAT_LOST` close.           | Port-from-TS.                | §6.4, §12  | `heartbeat`            |
| `ack.md`              | `session.ack` coalescer, `last_processed_seq` highwater, `back_pressure` status emit.     | Port-from-TS.                | §6.5, §8.2 | `ack`                  |
| `list-jobs.md`        | `Client.ListJobs`, filters, opaque cursor pagination, same-principal auth default.        | Port-from-TS.                | §6.6       | `list_jobs`            |
| `subscribe.md`        | `Client.Subscribe`, cross-session attach, replay with `history:true`, subscriber-scoped `event_seq`, `PERMISSION_DENIED` on non-submitter cancel. | Port-from-TS.                | §7.6       | `subscribe`            |
| `agent-versions.md`   | `name@version` grammar, inventory shape, default resolution, `AGENT_VERSION_NOT_AVAILABLE`. | Port-from-TS.                | §7.5, §12  | `agent_versions`       |
| `lease-expires-at.md` | `lease_constraints.expires_at`, runtime watchdog, `LEASE_EXPIRED` at submit and mid-run.  | Port-from-TS.                | §9.5, §12  | `lease_expires_at`     |
| `cost-budget.md`      | `cost.budget` lease grammar, per-currency counters, debit on `cost.*` metrics, `BUDGET_EXHAUSTED` surfaced as `tool_result`, `cost.budget.remaining` debounced emit. | Port-from-TS.                | §9.6, §12  | `cost.budget`          |
| `progress.md`         | `progress { current, total?, units?, message? }` event kind, non-negative `current`.      | Port-from-TS.                | §8.2.1     | `progress`             |
| `result-chunk.md`     | `JobContext.StreamResult` writer, `result_id`/`chunk_seq`/`more`, terminating `job.result` carries `result_id` + `result_size`, MUST NOT mix inline result with chunks. | Port-from-TS.                | §8.4       | `result_chunk`         |

Every feature page has the same four sections: **Wire**, **Use**,
**Errors**, **See also**. The "Use" section links to its
`Example_xxx` in the `arcp` package on `pkg.go.dev`.

### `04-examples/`

Short pointer pages, one per `examples/<name>/` directory. Each page
is ≤30 lines: what the example demonstrates, the spec § it cites, the
run command (`go run ./examples/<name>/...`), and a link into the
source. The full implementations live in `examples/`, not in `docs/`.
The example set mirrors `../../typescript-sdk/README.md` (v1.0 core +
v1.1 features + host integrations) so cross-SDK readers find the same
scenario names.

### `05-reference/` (human-friendly mirror of `pkg.go.dev`)

| File                  | Mirrors                          | Target reader                                  |
| --------------------- | -------------------------------- | ---------------------------------------------- |
| `client.md`           | `arcp/client`                    | Reader who landed on `pkg.go.dev` and wants a narrative pass. |
| `server.md`           | `arcp/server`                    | Same.                                          |
| `transport.md`        | `arcp/transport`                 | Same.                                          |
| `messages.md`         | `arcp/messages`                  | Same.                                          |
| `errors.md`           | `arcp` errors + sentinels        | Same.                                          |
| `middleware-otel.md`  | `arcp/middleware/otel`           | Same.                                          |

Each reference page links to the godoc URL for every named symbol and
duplicates only signatures + the one-sentence summary. No prose
duplication of doc comments — if a page would copy more than two
sentences from godoc, link instead. Rationale: godoc is the source of
truth; the docs site is the navigation layer.

## Frontmatter schema

Identical across all SDKs. The shared site reads it verbatim; field
names match `../../typescript-sdk/docs/` (which will adopt this same
schema in its v1.1 docs revision — TS docs today have no frontmatter,
this is the cross-language convergence point).

```yaml
---
title: Heartbeat
sdk: go
spec_sections: [§6.4, §12]
order: 1
kind: feature
---
```

| Field           | Required | Notes                                                                                 |
| --------------- | -------- | ------------------------------------------------------------------------------------- |
| `title`         | yes      | Human title. Sentence case. Used in nav and `<title>`.                                |
| `sdk`           | yes      | Literal `go`. Set per-SDK so the site can render cross-SDK switchers.                 |
| `spec_sections` | yes      | Array of spec section refs as written in the spec (`§6.4`, `§12`). Drives the §-index. |
| `order`         | yes      | Integer. Sort key **within the parent directory**. Lower first; ties break on `title`. |
| `kind`          | yes      | One of: `overview`, `quickstart`, `concept`, `feature`, `example`, `reference`, `conformance`. Drives nav grouping. |
| `summary`       | no       | One-line teaser for search-result cards. If absent, the site uses the first paragraph. |
| `pkg_godoc`     | no       | Full `https://pkg.go.dev/...` URL the page mirrors. Lets the site render a "view on pkg.go.dev" affordance. |

`order` is **only** scoped to its directory; numbering does not need
to be globally unique. `03-features/heartbeat.md` having `order: 1`
and `04-examples/heartbeat.md` having `order: 1` is fine.

Field names match what the TS site will read. Renaming on either side
breaks both — treat as a frozen schema.

## Doc comments (`pkg.go.dev` contract)

Hard rules, enforced in CI by `revive` (see `03-libraries.md` for the
linter pick):

1. **Every exported symbol has a doc comment.** No exceptions for
   `Type`, `Func`, `Var`, `Const`, package-level. `revive`'s
   `exported` rule enforces this; `golint`'s same rule enforces the
   "begins with the name" form.
2. **The first sentence begins with the symbol name.** `// Submit
   sends job.submit and returns a JobHandle.` not `// Sends a job…`.
   Godoc parses the first sentence as the summary in the package index.
3. **Doc comments state preconditions, postconditions, ownership, and
   error contract — not what the code obviously shows.** The reader
   already sees the parameter types in the signature on `pkg.go.dev`.
   What they cannot see: who closes a returned channel, when a context
   cancel propagates, which sentinels can wrap into the returned error.
4. **Channel ownership is spelled out.** Functions returning `<-chan T`
   say in their comment "the returned channel is closed when the job
   reaches a terminal state or `ctx` is cancelled, whichever first."
   Without this, callers leak goroutines on subscribe / chunk reads
   (see `02-current-audit.md` high-risk items §6.6 and §7.6).
5. **Error contracts list sentinels by name.** `// Returns
   ErrLeaseExpired (§9.5), ErrBudgetExhausted (§9.6), or an error
   wrapping ErrInvalidRequest if the feature was not negotiated.`
   Callers grep doc comments for sentinel names; do not force them to
   read source.
6. **`pkg.go.dev` renders fenced ```go``` blocks.** Use them in
   package overview (`doc.go`) over ASCII diagrams. Diagrams live in
   the docs site only.

### `Example_xxx` functions

Place runnable examples in `example_test.go` adjacent to the package
they exercise. Each is a 20–30 line function that compiles under
`go test` and renders on `pkg.go.dev`'s "Examples" tab.

The set to ship for v1.1.0, one per feature page in `03-features/`:

| Example function          | Demonstrates                                          | Lives in                        |
| ------------------------- | ----------------------------------------------------- | ------------------------------- |
| `Example_quickstart`      | Server + client over MemoryTransport, one job.        | `arcp/example_test.go`          |
| `Example_submitAndStream` | `Client.Submit` + range over `Handle.Events`.         | `arcp/example_test.go`          |
| `Example_subscribe`       | Cross-session attach, replay with `history:true`.     | `arcp/client/example_test.go`   |
| `Example_listJobs`        | Paginated list with opaque cursor.                    | `arcp/client/example_test.go`   |
| `Example_heartbeat`       | Negotiating `heartbeat`, handling `HEARTBEAT_LOST`.   | `arcp/example_test.go`          |
| `Example_costBudget`      | Lease with `cost.budget`, `BUDGET_EXHAUSTED` surface. | `arcp/example_test.go`          |
| `Example_leaseExpiresAt`  | `lease_constraints.expires_at`, mid-run termination.  | `arcp/example_test.go`          |
| `Example_resultChunk`     | `JobContext.StreamResult` from a handler.             | `arcp/server/example_test.go`   |
| `Example_progress`        | Emitting `progress` events from a handler.            | `arcp/server/example_test.go`   |
| `Example_agentVersions`   | Registering `echo@1.0.0`, default-version resolution. | `arcp/server/example_test.go`   |

Doubling as compile-checked snippets: when `01-quickstart.md` or a
feature page says "here is how it looks", the body of the example is
the source of truth — the markdown either quotes it verbatim with a
godoc URL or it omits the code and links. No drift between docs and
the SDK because `go test ./...` fails on stale examples.

## `README.md` outline

For `go-sdk/README.md`. Target length ≤8 minute read.

### 1. Badge row

Three badges, no shields-fluff:

- License — Apache-2.0.
- Go version — `≥ 1.23.0` (per `03-libraries.md`).
- ARCP version — `v1.1`.

### 2. Pitch (two sentences)

> Go SDK for [ARCP v1.1](../spec/docs/draft-arcp-02.1.md), the wire
> protocol an agent uses to talk to the runtime that hosts it. Ships a
> client, a server, transports for WebSocket / stdio / in-memory, OTel
> middleware, and the `arcp` CLI.

No "welcome", no "the most…", no "let us walk you through".

### 3. Install

```sh
go get github.com/agentruntimecontrolprotocol/go-sdk@v1.1.0
```

Then the quickstart from §6 below, runnable with `go run`.

### 4. Packaging table

Sub-packages mirror TS sub-packages so cross-SDK readers find the
same shape. The Go convention is "one importable package per
directory under the module root" — flatter than TS's pnpm workspace
but the same partition.

| Go import path                               | TS equivalent                  | Use when                                                                       |
| -------------------------------------------- | ------------------------------ | ------------------------------------------------------------------------------ |
| `.../go-sdk` (root, package `arcp`)          | `@arcp/sdk` (meta)             | One import for the common case. Re-exports `arcp/client` and `arcp/server`.    |
| `.../go-sdk/client`                          | `@arcp/client`                 | Building a client that talks to a runtime. No server symbols.                  |
| `.../go-sdk/server`                          | `@arcp/runtime`                | Hosting agents. No client symbols.                                             |
| `.../go-sdk/transport`                       | `@arcp/core/transport`         | Custom transports, or pairing `MemoryTransport` in tests.                      |
| `.../go-sdk/messages`                        | `@arcp/core/messages`          | Direct envelope construction (rare; clients/servers do it for you).            |
| `.../go-sdk/middleware/nethttp`              | `@arcp/node`                   | Attaching the WS upgrade to an existing `*http.Server`.                        |
| `.../go-sdk/middleware/chi`                  | (no direct TS equivalent)      | Mounting on a `chi.Router`. Go-side convenience; chi is the most-used Go router. |
| `.../go-sdk/middleware/otel`                 | `@arcp/middleware-otel`        | Emitting OTel spans + W3C trace propagation per §11.                           |
| `.../go-sdk/cmd/arcp` (binary `arcp`)        | `@arcp/sdk` CLI                | `arcp serve`, `arcp submit`, `arcp replay`.                                    |

There is no `arcp/core` package — Go conventions prefer flat over a
shared "core" import name. Envelopes, errors, and messages live as
`arcp/messages`, `arcp/errors` (or as top-level `arcp` symbols), and
the `transport` package. The TS `core/` partition is a workspace
artifact, not a public API distinction.

### 5. Core concepts

One paragraph each, no code:

- **Envelope (§5)** — every wire message is `arcp:"1"` + `id` +
  `type` + `payload` + optional session/job/event/trace ids. Forward-
  compatible: unknown fields round-trip.
- **Session (§6)** — `session.hello` → `session.welcome` (or
  `session.error`); ends on `session.bye` or transport close.
- **Job (§7)** — `job.submit` → `job.accepted` → `job.event*` →
  terminal `job.result` ∣ `job.error`. One verb to start, one to stop.
- **Lease (§9)** — capability namespace → glob patterns; immutable
  at submit; runtime MAY shrink, MUST NOT expand. v1.1 adds
  `expires_at` and `cost.budget`.
- **Event (§8)** — one `job.event` envelope, `payload.kind` ∈ eight
  reserved values plus `x-vendor.*`. v1.1 adds `progress` and
  `result_chunk`.
- **Subscribe (§7.6)** — re-attach to a job from a different session,
  optionally replay buffered events. Subscribers cannot cancel.

Each paragraph ends with one cross-link: `→ docs/03-features/<x>.md`
and `→ pkg.go.dev/.../arcp#<Symbol>`.

### 6. Quickstart code block

A complete server + client over `MemoryTransport`, ≤40 lines. State
in the README: **the snippet is the body of `Example_quickstart` in
`arcp/example_test.go`** — copying from this README into a `.go` file
and running `go run` works because the same source is tested under
`go test`.

### 7. Compatibility table

| Go SDK version | ARCP version | Go version       |
| -------------- | ------------ | ---------------- |
| `v1.1.x`       | v1.1         | `≥ 1.23.0`       |
| `v1.0.x`       | (not shipped — see [`planning/v1.1/02-current-audit.md`](./planning/v1.1/02-current-audit.md)) | n/a |

Footnote that v1.1 is wire-compatible with v1.0 (additive per
§"Changes from v1.0"); a v1.0 client connecting to a v1.1 runtime
gets a v1.0 session and the runtime suppresses v1.1 features that
were not negotiated.

### 8. Conformance link

One line linking to `CONFORMANCE.md`, which is generated from the
conformance harness (see `07-tests.md`).

### 9. License

`[Apache-2.0](./LICENSE)`. One line.

## Voice rules (enforced)

- Terse. No marketing. No emojis. No "Welcome to…", "we'll", "let's".
- "we" = SDK maintainers. "you" = the user.
- Every code block compiles. Long ones quote an `Example_xxx`; short
  ones are tested as fenced-block tests where the harness supports it.
- Reject these words in any docs file or doc comment: leverage,
  robust, scalable, performant, powerful, modern, easy to use,
  developer-friendly, idiomatic-as-substitute, battle-tested. CI grep
  in the docs lint job (see `07-tests.md`) fails the build on any hit.
- Each section either cites a spec §, a TS docs path, or a specific
  Go convention. Sections that cite none get cut.

## What is NOT in docs

Stated up-front so contributors do not add these:

- **No tutorial chain.** Pages do not assume the reader read a prior
  page. Every page is independently navigable from a search result.
  Concepts that recur (envelope, session) get a one-sentence inline
  recap with a link, not a "see chapter 2" pointer.
- **No architecture-decision records in `docs/`.** ADR-shaped writing
  (why pick X over Y, what we considered) lives in `planning/`. The
  docs site reads `docs/`, not `planning/`. The split is enforced by
  the docs-site build only ingesting `docs/`.
- **No "migrating from v1.0" page in v1.1.0.** No v1.0 Go SDK ever
  shipped (see `02-current-audit.md` headline finding). Add the
  migration page in v1.1.1 only if v1.0.x is back-ported, which is not
  planned.
- **No copy of the spec.** Concept pages summarise and link; they do
  not re-state normative text. If a reader needs the MUST/SHOULD, they
  click through to `../spec/docs/draft-arcp-02.1.md`.
- **No per-page changelog.** `CHANGELOG.md` at the module root is the
  one place; per-page changelogs drift.

## Cross-references

- Spec: `../spec/docs/draft-arcp-02.1.md` §§6.4, 6.5, 6.6, 7.5, 7.6,
  8.2.1, 8.4, 9.5, 9.6, 12.
- `01-spec-delta.md` — the additions table is the source for the
  `03-features/` page list and the negotiation-flag column.
- `02-current-audit.md` — anti-salvage list keeps the docs honest;
  pages do not document deleted shapes (`stream.*`, `permission.*`,
  generic `subscribe`).
- `03-libraries.md` — Go version floor in the compatibility table,
  `revive` for the "begins with name" doc-comment rule.
- TS docs structure: `../../typescript-sdk/docs/` (guides/, packages/,
  conformance.md). The Go layout is a deliberate re-shape — flatter
  features/, no per-package guide page (the reference/ mirror of
  godoc replaces TS's `packages/`).
- TS README: `../../typescript-sdk/README.md` is the structural
  template for §§1–9 above.

# Idiomatic Go for Public SDKs — Definitive Guide

A prescriptive, opinionated style guide for public Go SDKs. Optimized for
Claude Code consumption. Every rule is a hard rule unless explicitly marked
as a preference. When in doubt, choose the option that is smaller, simpler,
and more boring.

---

## 1. Package Design

- One package = one cohesive concept. If you cannot describe the package in
  one sentence without "and", split it.
- Package names: lowercase, single word, no underscores, no plurals. The
  package name is part of every exported symbol — `client.New`, not
  `clients.NewClient`.
- Never name a package `util`, `common`, `helpers`, `shared`, `misc`, or
  `base`. These are smells, not packages.
- Avoid stutter: `http.Server`, not `http.HTTPServer`. The import path
  carries the namespace.
- Internal-only code lives under `internal/`. Default to `internal/` for
  anything that does not need to be exported.
- Top-level package (the import root) holds the primary `Client` type and
  user-facing entry points. Subpackages hold optional features, transports,
  or domain types.
- No circular dependencies. If you feel pressure to create one, extract a
  shared types package with zero behavior.

## 2. API Surface

- Exporting is a one-way door. Default every symbol to unexported. Promote
  to exported only when a user concretely needs it.
- Minimize the public API. Every exported symbol is a maintenance contract
  forever (semver).
- Accept interfaces, return concrete types. The caller composes; the SDK
  delivers.
- Public functions take `context.Context` as the first parameter. No
  exceptions for I/O, network, or anything that can block.
- Never expose third-party types in your public API. Wrap them. Your
  dependency choices are not the caller's problem.
- Never return `interface{}` / `any` from a public function. Be specific.
- Functional options for constructors with more than two optional knobs.
  Two or fewer? Use a config struct or positional args.
- Variadic options must be safe to omit and safe to reorder.

## 3. Error Handling

- Errors are values. Return them. Do not panic across a package boundary.
- Sentinel errors for stable, comparable conditions: `var ErrNotFound =
errors.New("resource not found")`. Document them.
- Custom error types when callers need structured fields (HTTP status,
  retry-after, request ID). Implement `Error() string` and provide
  accessors.
- Wrap with `fmt.Errorf("doing X: %w", err)` to preserve the chain. Never
  use `%v` or `%s` on an error you intend to be inspectable.
- Support `errors.Is` and `errors.As`. Test both.
- Never log and return — pick one. The caller decides logging.
- Never swallow errors. If you must ignore one, assign to `_` with a
  one-line comment explaining why.
- No `panic` in library code except for truly unrecoverable programmer
  errors (nil receiver on a required field at construction time). Document
  any panic.

## 4. Context & Cancellation

- First parameter, named `ctx`, type `context.Context`. Always.
- Never store a `context.Context` in a struct. Pass it through.
- Honor cancellation: check `ctx.Err()` before long loops, pass `ctx` to
  every downstream call.
- Never call `context.Background()` or `context.TODO()` inside library
  code. The caller owns the root context.
- Deadlines and timeouts are caller concerns. The SDK may apply per-call
  defaults via options, but never overrides a caller's deadline.

## 5. Concurrency

- Document the goroutine-safety of every exported type. "Safe for
  concurrent use" or "Not safe for concurrent use" — no ambiguity.
- Prefer immutable values over locks. Prefer channels for ownership
  transfer, mutexes for shared state.
- Never start a goroutine you cannot stop. Every spawned goroutine must
  have a clear shutdown path tied to `ctx` or an explicit `Close`.
- No goroutine leaks. Tests must verify.
- Avoid `sync.Map`. Use a regular `map` with a `sync.RWMutex` unless you
  have measured contention.

## 6. Interfaces & Types

- Define interfaces where they are consumed, not where they are
  implemented.
- Small interfaces. One to three methods. If you have more, you are
  describing a struct.
- Zero values should be useful. A user should be able to `var c Client`
  for simple cases, or the type should panic loudly with a clear message
  on misuse.
- No pointer receivers mixed with value receivers on the same type. Pick
  one. Default to pointer receivers for anything with state.
- Embed sparingly. Embedding leaks the inner type's full API surface.
- No generic abstractions for one caller. Wait for the third use before
  generalizing.

## 7. Constructors & Options

- `New` returns the primary type. `NewWithX` is a code smell — use
  options.
- Options pattern:
  ```go
  type Option func(*config)
  func WithTimeout(d time.Duration) Option { ... }
  func New(apiKey string, opts ...Option) (*Client, error) { ... }
  ```
- Required arguments are positional. Optional arguments are options.
- Validate in the constructor. Fail fast with a clear error. Never defer
  validation to first use.
- Return `(*T, error)` from any constructor that can fail.

## 8. Documentation

- Every exported symbol has a doc comment. No exceptions. `golint` /
  `revive` enforces this.
- Doc comments start with the symbol name: `// Client represents...`,
  `// New returns...`.
- Package-level doc in `doc.go` for any non-trivial package. Include an
  overview and at least one usage snippet.
- Every public type needs at least one runnable `Example` in `_test.go`.
  These are tests and they appear in pkg.go.dev.
- Document errors returned (sentinels, types). Document concurrency
  guarantees. Document deprecations with `// Deprecated:` and a migration
  path.

## 9. Testing

- Table-driven tests are the default. Each row has a `name`.
- Use the standard library `testing` package. No `testify`, no `ginkgo`,
  no DSLs in a public SDK. Dependencies are contagion.
- `httptest.Server` for HTTP transports. Never hit the real network in
  unit tests.
- Use `t.Parallel()` aggressively. Tests must be safe for parallel
  execution.
- Use `t.Helper()` in test helpers. Use `t.Cleanup()` for teardown.
- Race detector required in CI: `go test -race ./...`.
- Coverage is a signal, not a target. Aim for meaningful coverage of
  public API and error paths, not 100%.
- Fuzz tests for parsers, serializers, and any input-handling code.

## 10. Complexity Limits

These are hard ceilings enforced by linters. Refactor before exceeding.

- **Cyclomatic complexity per function: 10.** Use `gocyclo`.
- **Cognitive complexity per function: 15.** Use `gocognit`.
- **Function length: 50 lines.** Strong preference for under 30.
- **Function parameters: 5.** More than 5 means a struct.
- **Return values: 3.** More than 3 means a struct.
- **Nesting depth: 3.** Use early returns and guard clauses to flatten.
- **File length: 400 lines.** Hard ceiling. Split by responsibility.
- **Package size: aim for under 2000 lines total across files.**

Patterns to reduce complexity:

- Guard clauses at the top. Happy path is unindented and reads top to
  bottom.
- Extract helpers as soon as a function does two things.
- Replace flag-argument booleans with two functions.
- Replace switch-on-type with polymorphism (interface methods).
- Replace nested conditionals with early returns.
- A function with a comment block explaining "what this section does" is a
  function asking to be extracted.

## 11. Formatting & Style

- `gofumpt` on save. `gofmt` is the floor, `gofumpt` is the standard.
- **Line length: aspire to 80 characters. Hard ceiling 100.** Break long
  signatures, chained calls, and string literals. Use named consts for
  long URLs.
- Imports in three groups: stdlib, third-party, local. `goimports`
  enforces this.
- Names: short for short scope, descriptive for long scope. `i` in a tight
  loop is fine; `customerInvoiceLineItem` for a method receiver is not.
- No Hungarian notation. No type suffixes (`userMap`, `nameStr`).
- Receivers: one or two lowercase letters. Consistent across all methods
  on the type.
- Constants and enums: typed. Use `iota` with a named type. Provide a
  `String()` method.
- No `init()` functions in library code. They are invisible side effects.
- No package-level mutable state. Configuration flows through
  constructors.

## 12. Dependencies

- Standard library first. Always.
- Every dependency added to a public SDK is a transitive dependency for
  every user. Justify each one in the PR.
- Forbidden in public SDKs without explicit exemption: logging libraries
  (use `log/slog`), assertion libraries, ORM-like helpers, reflection
  helpers.
- Pin via `go.mod`. Run `govulncheck` in CI.

## 13. Versioning & Compatibility

- Semantic versioning. Strictly.
- v0.x is for pre-stable. Breaking changes allowed but document them.
- v1+: no breaking changes to exported API without a major version bump
  and a new import path (`/v2`).
- Additive changes only in minor versions. New methods on interfaces are
  breaking — define new interfaces instead.
- Deprecate before remove. Minimum one minor version of deprecation
  before removal at next major.

## 14. Tooling — Required CI Gates

```yaml
- go vet ./...
- gofumpt -l -d . # must produce no diff
- goimports -l -d . # must produce no diff
- golangci-lint run # config below
- go test -race -count=1 ./...
- govulncheck ./...
```

Minimum `.golangci.yml` linters:

```
errcheck, gosimple, govet, ineffassign, staticcheck, unused,
gocyclo, gocognit, gofumpt, goimports, revive, misspell,
unconvert, unparam, prealloc, bodyclose, contextcheck, errorlint,
forcetypeassert, gocritic, godot, noctx, nilerr, exhaustive
```

With:

```
gocyclo:    { min-complexity: 10 }
gocognit:   { min-complexity: 15 }
funlen:     { lines: 50, statements: 40 }
lll:        { line-length: 100 }
```

## 15. Logging & Observability

- Use `log/slog` from stdlib. No `logrus`, no `zap`, no `zerolog` in
  public SDK code.
- Accept an `*slog.Logger` via option. Default to `slog.Default()` or a
  discard handler — never write to stderr by default.
- Never log at info or above from library code without an opt-in. Debug
  is acceptable.
- Structured fields, not formatted strings. `slog.String("key", value)`.
- For tracing: accept `context.Context`, do not import OpenTelemetry
  directly. Let users wire it.

---

# Refactor Prompt for Claude Code

Use the following prompt verbatim (or as a base) to refactor an existing
Go codebase against this guide. The prompt is designed for autonomous
execution — no mid-task check-ins, no permission requests, see it
through.

```markdown
# Task: Refactor Go SDK to Idiomatic Standards

You are refactoring a public Go SDK to conform to the standards in
`GO_SDK_GUIDE.md` (located at the repo root). This is an autonomous task.
You will complete it in full before reporting back. Do not stop to ask
for permission. Do not ask clarifying questions unless you have hit a
hard blocker that cannot be resolved by reading the codebase. Make
decisions, document them in commit messages, and move on.

## Phase 0 — Investigation (do not write code yet)

1. Read `GO_SDK_GUIDE.md` in full. Internalize the rules.
2. Walk the entire codebase. Build a mental map: packages, exported
   surface, internal structure, dependency graph.
3. Run the current tooling. Capture baseline:
   - `go build ./...`
   - `go test -race ./...`
   - `golangci-lint run` (install if missing, use the config from the
     guide section 14)
   - `gocyclo -over 10 .` and `gocognit -over 15 .`
   - File line counts (`wc -l` over `**/*.go`)
   - Function line counts (use `funlen` linter output)
4. Produce a written refactor plan as `REFACTOR_PLAN.md` in the repo
   root. Include:
   - Files exceeding 400 lines (with proposed splits)
   - Functions exceeding 50 lines or complexity ceilings (with proposed
     extractions)
   - Lines exceeding 100 chars (with strategy)
   - Public API surface deltas required (additions, deprecations,
     removals)
   - Dependencies to remove or replace
   - Missing docs, missing examples, missing concurrency annotations
   - Test gaps (race-unsafe tests, missing table-driven conversions)
   - Ordering: dependency-free changes first, breaking changes last
5. Commit the plan. Then begin execution.

## Phase 1 — Mechanical Pass (no behavior changes)

Execute in this order. Commit after each step.

1. Run `gofumpt -w .` and `goimports -w .` across the entire repo.
2. Enforce 100-char hard line limit, aspire to 80. Break long signatures,
   extract const URLs, split chained calls. Do not introduce new
   variables purely for shortening — refactor structurally.
3. Reorganize imports into stdlib / third-party / local groups.
4. Rename packages that violate naming rules (no `util`, `common`,
   `helpers`, `shared`, `misc`, `base`, no plurals, no underscores).
   Update all imports.
5. Move non-public symbols into `internal/`. Anything not explicitly
   needed by external callers gets demoted.
6. Remove all `init()` functions from library code. Replace with
   explicit constructor logic.
7. Remove all package-level mutable state. Migrate to constructor
   configuration.
8. Run the test suite. It must still pass at every commit.

## Phase 2 — Structural Pass (file & function sizing)

1. Split every file over 400 lines along responsibility boundaries.
   Group by cohesion, not alphabetical accident.
2. Refactor every function exceeding 50 lines or complexity ceilings.
   Techniques:
   - Extract helpers (private, same package)
   - Apply guard clauses to flatten nesting
   - Replace boolean parameters with named functions
   - Replace type-switch on interface with method dispatch
3. Functions with more than 5 parameters get a parameter struct.
   Functions with more than 3 return values get a result struct.
4. Each commit in this phase must keep tests green and reduce at least
   one complexity metric.

## Phase 3 — API Hardening

1. Audit every exported symbol. If it is not documented or not used
   externally, demote to unexported. Document everything that survives.
2. Ensure every public function that does I/O or can block takes
   `context.Context` as its first parameter. Add it where missing.
   Treat this as a breaking change; coordinate with the version bump in
   Phase 5.
3. Convert ad-hoc errors to a consistent strategy:
   - Sentinel `var Err...` for stable conditions
   - Typed errors for structured info
   - Wrap all internal errors with `fmt.Errorf("...: %w", err)`
   - Verify `errors.Is` / `errors.As` paths work with tests
4. Replace all `panic` calls in library code with returned errors,
   except documented constructor preconditions.
5. Replace any returned `interface{}` / `any` with concrete types.
6. Convert constructors with more than two optional arguments to the
   functional options pattern.
7. Annotate every exported type with goroutine-safety documentation.

## Phase 4 — Dependencies & Observability

1. Remove every third-party dependency that has a stdlib equivalent.
   Specifically purge: `testify`, `logrus`, `zap`, `zerolog`, `gorilla`
   helpers, ORM-like libraries.
2. Migrate logging to `log/slog`. Accept `*slog.Logger` via option,
   default to a discard handler.
3. Remove any direct OpenTelemetry imports from library code. Document
   the integration pattern for users instead.
4. Run `go mod tidy`. Run `govulncheck ./...`. Address every finding.

## Phase 5 — Tests & Examples

1. Convert all tests to table-driven where multiple cases exist. Every
   row needs a `name`.
2. Add `t.Parallel()` to every test that is safe. Add `t.Helper()` to
   every helper. Use `t.Cleanup()` for teardown.
3. Replace any real network calls in tests with `httptest.Server`.
4. Add at least one `Example` function per exported type. These must
   compile and run via `go test`.
5. Add fuzz tests for any parser, serializer, or input handler.
6. Verify `go test -race -count=1 ./...` is clean.

## Phase 6 — CI & Tooling

1. Add or update `.golangci.yml` to match the guide's section 14
   linter set with the specified thresholds.
2. Add or update CI (`.github/workflows/` or equivalent) to run the
   full gate set: `go vet`, `gofumpt -l -d .`, `goimports -l -d .`,
   `golangci-lint run`, `go test -race -count=1 ./...`,
   `govulncheck ./...`.
3. Add a `Makefile` target `make check` that runs the same set
   locally.

## Phase 7 — Versioning

1. If you introduced breaking API changes (new `ctx` parameters,
   removed exports, renamed packages), prepare a `/v2` directory or
   plan the major version bump in `CHANGELOG.md`.
2. Mark removed-but-shimmed symbols with `// Deprecated:` and a
   migration note.
3. Update `README.md` with the new usage patterns. Keep examples
   minimal and runnable.

## Completion Criteria — Hard Gates

You are not done until all of the following are true. Verify each
explicitly before reporting completion. Do not declare success on any
gate you have not run.

- [ ] `go build ./...` succeeds with no warnings
- [ ] `go test -race -count=1 ./...` passes
- [ ] `golangci-lint run` produces zero findings
- [ ] `gofumpt -l .` produces no output
- [ ] `goimports -l .` produces no output
- [ ] `govulncheck ./...` reports no vulnerabilities
- [ ] No file exceeds 400 lines
- [ ] No function exceeds 50 lines
- [ ] No function exceeds cyclomatic complexity 10
- [ ] No function exceeds cognitive complexity 15
- [ ] No line exceeds 100 characters (grep for lines over 100 to
      verify)
- [ ] Every exported symbol has a doc comment
- [ ] Every exported type has goroutine-safety documentation
- [ ] Every exported type has at least one `Example` test
- [ ] No `init()` functions in library code
- [ ] No package-level mutable state
- [ ] No `panic` calls outside documented constructor preconditions
- [ ] No `interface{}` / `any` returned from exported functions
- [ ] No banned dependencies present in `go.mod`
- [ ] `REFACTOR_PLAN.md` is updated to reflect actual changes made
- [ ] `CHANGELOG.md` documents every breaking change

## Working Style

- Make decisions. Do not stall on "this could go either way" — pick the
  smaller, simpler, more boring option and move on.
- Commit frequently with focused messages. One concept per commit.
- Tests must stay green at every commit. If a refactor breaks tests,
  fix the tests in the same commit only if the test was wrong; otherwise
  revert and try a different approach.
- Spawn parallel sub-agents for independent file refactors when the
  work fans out (e.g. Phase 2 splits across many files). Each sub-agent
  owns a non-overlapping set of files and reports back with a diff
  summary.
- If you hit a genuine architectural ambiguity that cannot be resolved
  by reading the code and the guide, document the decision you made in
  `REFACTOR_PLAN.md` with a one-paragraph rationale and proceed. Do not
  stop.
- When complete, produce a final report: diff summary, metrics before
  vs after (file count, line count, complexity ceilings, dependency
  count, coverage), and any deferred items with rationale.
```

---

## Appendix — Quick Reference Card

```
PACKAGE      → one concept, no util/common/helpers
EXPORT       → default unexported, justify every export
ERRORS       → values, wrap with %w, document sentinels
CONTEXT      → first param, never stored, never Background() in lib
CONCURRENCY  → document safety, no leaked goroutines
INTERFACES   → small, defined at consumer, accept in / concrete out
CONSTRUCTORS → New + functional options
DOCS         → every exported symbol, examples for every type
TESTS        → stdlib only, table-driven, parallel, race-clean
LIMITS       → 10 cyclo / 15 cog / 50 lines fn / 400 lines file
LINES        → aspire 80, hard 100
DEPS         → stdlib first, justify every addition
LOGGING      → log/slog only, opt-in, structured
VERSIONING   → strict semver, /v2 for breaking
```

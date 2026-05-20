---
title: Concepts
sdk: go
spec_sections: [§5, §6, §7, §8, §9, §10]
order: 3
kind: concept
---

# Concepts

- **Envelope (§5)** — every message is `{arcp: "1.1", id, type, session_id?, job_id?, trace_id?, event_seq?, payload}`. Unknown top-level fields round-trip.
- **Session (§6)** — `session.hello` → `session.welcome` (or `session.error`). Either peer ends the session with `session.bye` or transport close.
- **Job (§7)** — `job.submit` → `job.accepted` → `job.event*` → terminal `job.result` ∣ `job.error`. The submitting session is the only session permitted to cancel.
- **Lease (§9)** — capability namespace → glob patterns; immutable for the job's lifetime. Optional `expires_at` and `cost.budget` bound time and cost.
- **Event (§8)** — one `job.event` envelope. `payload.kind` is one of `log`, `thought`, `tool_call`, `tool_result`, `status`, `metric`, `artifact_ref`, `delegate`, `progress`, `result_chunk` — or an `x-vendor.*` namespaced extension.
- **Subscribe (§7.6)** — attach to a job from a different session. Subscribers may replay buffered history but cannot cancel.
- **Delegate (§10)** — a child job whose lease is a subset of its parent's. Budget and `expires_at` are subset-checked at delegation time.

# Job events (§8)

Agents emit events through `*server.JobContext`. Every emitter
allocates a session-scoped monotonic `event_seq` and fans the
envelope out to any attached subscribers in addition to the
submitter.

| Emitter | Event kind | Notes |
| --- | --- | --- |
| `jc.Log(level, msg, attrs...)` | `log` | `level` is an `slog.Level`. |
| `jc.Thought(text)` | `thought` | Free-form reasoning text. |
| `jc.ToolCall(tool, args)` | `tool_call` | Returns the generated `call_id` for the matching result. |
| `jc.ToolResult(callID, result)` | `tool_result` | Success body. |
| `jc.ToolError(callID, err)` | `tool_result` | Failure body; `code`/`message`/`retryable` derived from `err`. |
| `jc.Status(phase, message)` | `status` | Free-form phase strings; spec reserves `credential_rotated`. |
| `jc.Metric(name, value, unit, dims)` | `metric` | Names starting with `cost.` debit the matching budget currency (see below). |
| `jc.ArtifactRef(uri, ct, size, sha256)` | `artifact_ref` | Out-of-band artifact pointer. |
| `jc.Progress(current, total, units, message)` | `progress` | `total=0` means indeterminate. |
| `jc.RotateCredential(id, newValue)` | `status` | Emits `phase: "credential_rotated"` and revokes the prior upstream credential. Requires a provisioner. |
| `jc.StreamResult(encoding)` | `result_chunk` | Returns an `io.WriteCloser`; see streaming below. |

```go
jc.Status("running", "starting")
jc.Progress(3, 10, "files", "")
callID := jc.ToolCall("search.web", map[string]string{"q": "ARCP"})
jc.ToolResult(callID, map[string]int{"hits": 4})
jc.Metric("cost.search", 0.42, "USD", nil)
```

Clients can read live events from a submitted handle:

```go
for ev := range h.Events() {
    switch ev.Kind {
    case messages.KindProgress:
        var p messages.ProgressBody
        _ = json.Unmarshal(ev.Body, &p)
    }
}
```

Use `messages.DecodeEventBody(&ev)` if you'd rather receive a typed
body struct without writing the per-kind switch yourself.

## Cost metrics and budget remaining

When `Metric` is called with a name starting with `cost.` and a unit
matching a budgeted currency, the runtime debits the counter. After
each successful debit the runtime emits a follow-up
`cost.budget.remaining` metric whenever the change exceeds 5% of the
initial budget or the remaining balance reaches zero, so callers can
observe drawdown without polling.

## Streaming results

`StreamResult` opens a writer that emits `result_chunk` events. The
default encoding is `utf8`; pass `"base64"` for binary payloads. The
runtime caps a single stream at `Options.MaxResultBytes` (default
32 MiB) and returns `INTERNAL_ERROR` if a write would exceed it.
Call the writer's `Close` to send the terminal chunk with
`more: false`; the runtime then emits the final `job.result` with
the `result_id` and `result_size` once the `AgentFunc` returns.
`StreamResult` may only be called once per job.

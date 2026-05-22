# Job events (§8)

Agents emit events through `*server.JobContext`.

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

`StreamResult` emits `result_chunk` events and returns a writer. Close
the writer before the agent returns so the runtime can emit the final
`job.result` with result metadata.

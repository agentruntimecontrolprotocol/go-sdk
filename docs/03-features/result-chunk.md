---
title: Result Chunk
sdk: go
spec_sections: [§8.4]
order: 9
kind: feature
---

# Result Chunk

**Negotiation flag:** `result_chunk`. `JobContext.StreamResult`
returns an `io.WriteCloser`; each `Write` emits one `result_chunk`
event. The runtime emits the terminating `job.result` with
`result_id` and `result_size` once the agent function returns.

```go
w, _ := jc.StreamResult("utf8")
defer w.Close()
_, _ = w.Write(big)
```

The client reassembles via `JobHandle.CollectChunks`.

## See also

- [examples/result-chunk](../../examples/result-chunk)

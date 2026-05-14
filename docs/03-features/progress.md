---
title: Progress
sdk: go
spec_sections: [§8.2.1]
order: 8
kind: feature
---

# Progress

**Negotiation flag:** `progress`. Structured progress events with
optional `total`, `units`, and `message`.

```go
jc.Progress(current, total, "files", "indexing")
```

## See also

- [examples/progress](../../examples/progress)

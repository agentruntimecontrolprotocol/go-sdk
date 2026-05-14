---
title: Subscribe
sdk: go
spec_sections: [§7.6]
order: 4
kind: feature
---

# Subscribe

**Negotiation flag:** `subscribe`. Attach to a job submitted in
another session. Subscribers observe events under their own
session-scoped event_seq space; they cannot cancel.

```go
sub, _ := cli.Subscribe(ctx, jobID, client.SubscribeOptions{History: true})
for ev := range sub.Events() { ... }
```

## See also

- [examples/subscribe](../../examples/subscribe)

---
title: Lease Expiration
sdk: go
spec_sections: [§9.5, §12]
order: 6
kind: feature
---

# Lease Expiration

**Negotiation flag:** `lease_expires_at`. Submit with
`LeaseConstraints.ExpiresAt`; the runtime watchdog terminates the job
with `LEASE_EXPIRED` if the deadline passes. The watchdog fires on
the configured `Options.Clock`.

```go
exp := time.Now().Add(5 * time.Minute)
cli.Submit(ctx, client.SubmitRequest{
    LeaseConstraints: &messages.LeaseConstraints{ExpiresAt: &exp},
})
```

## See also

- [examples/lease-expires-at](../../examples/lease-expires-at)

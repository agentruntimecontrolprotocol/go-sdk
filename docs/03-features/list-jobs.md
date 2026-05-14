---
title: Job Listing
sdk: go
spec_sections: [§6.6]
order: 3
kind: feature
---

# Job Listing

**Negotiation flag:** `list_jobs`. `Client.ListJobs` returns a
read-only inventory filtered by status / agent / created_at, with
opaque cursor pagination keyed by `(created_at, job_id)`.

```go
resp, err := cli.ListJobs(ctx, client.ListJobsRequest{
    Filter: messages.ListJobsFilter{Status: []string{"running"}},
    Limit:  10,
})
```

## See also

- [examples/list-jobs](../../examples/list-jobs)

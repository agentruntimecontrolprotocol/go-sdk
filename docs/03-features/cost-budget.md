---
title: Cost Budget
sdk: go
spec_sections: [§9.6, §12]
order: 7
kind: feature
---

# Cost Budget

**Negotiation flag:** `cost.budget`. The lease's `cost.budget`
patterns initialize per-currency counters. Agent `metric` events with
`name=cost.*` and matching `unit` debit the counter; depletion
surfaces `BUDGET_EXHAUSTED` preferentially as a `tool_result` body.

```go
LeaseRequest: arcp.Lease{
    arcp.CapToolCall:   {"search.*"},
    arcp.CapCostBudget: {"USD:1.00"},
}
```

## See also

- [examples/cost-budget](../../examples/cost-budget)

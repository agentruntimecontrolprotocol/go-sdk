# Errors (§12)

ARCP errors are structured values with a canonical code, message,
retry hint, optional details, and optional cause.

```go
return nil, arcp.ErrPermissionDenied.WithMessage("model not in lease")
```

At the protocol boundary, the runtime uses `arcp.Code(err)` and
`arcp.IsRetryable(err)` to populate `job.error` and `session.error`.
Use `errors.Is(err, arcp.ErrBudgetExhausted)` to test for a specific
code through wrapping.

Provisioners should return `credentials.BudgetExhausted` when an
upstream credential cap is depleted so callers see `BUDGET_EXHAUSTED`.

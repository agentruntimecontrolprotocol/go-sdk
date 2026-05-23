# Errors (§12)

ARCP errors are structured values with a canonical code, message,
retry hint, optional details, and optional cause.

```go
return nil, arcp.ErrPermissionDenied.WithMessage("model not in lease")
return nil, arcp.Newf(arcp.CodeInternalError, "marshal: %v", err)
```

## The fifteen canonical codes

| Code | Sentinel | Default retryable |
| --- | --- | --- |
| `PERMISSION_DENIED` | `ErrPermissionDenied` | false |
| `LEASE_SUBSET_VIOLATION` | `ErrLeaseSubsetViolation` | false |
| `JOB_NOT_FOUND` | `ErrJobNotFound` | false |
| `DUPLICATE_KEY` | `ErrDuplicateKey` | false |
| `AGENT_NOT_AVAILABLE` | `ErrAgentNotAvailable` | false |
| `AGENT_VERSION_NOT_AVAILABLE` | `ErrAgentVersionNotAvailable` | false |
| `CANCELLED` | `ErrCancelled` | false |
| `TIMEOUT` | `ErrTimeout` | false |
| `RESUME_WINDOW_EXPIRED` | `ErrResumeWindowExpired` | false |
| `HEARTBEAT_LOST` | `ErrHeartbeatLost` | true |
| `LEASE_EXPIRED` | `ErrLeaseExpired` | false |
| `BUDGET_EXHAUSTED` | `ErrBudgetExhausted` | false |
| `INVALID_REQUEST` | `ErrInvalidRequest` | false |
| `UNAUTHENTICATED` | `ErrUnauthenticated` | false |
| `INTERNAL_ERROR` | `ErrInternalError` | true |

## Helpers

- `arcp.Code(err)` walks the error chain and returns the first
  embedded `ErrorCode`. Non-arcp errors return `CodeInternalError`.
- `arcp.IsRetryable(err)` returns the embedded `Retryable` flag for
  arcp errors. Non-arcp errors return `true` by default — a
  conservative choice so generic transport-level errors don't become
  fatal.
- `arcp.Newf(code, format, args...)` constructs an `*Error` with a
  formatted message and the default retryable flag for the code
  (`true` for `CodeInternalError` and `CodeHeartbeatLost`, `false`
  otherwise).
- `(*Error).WithCause(err)`, `(*Error).WithMessage(msg)`,
  `(*Error).WithDetails(map)` return copies of the sentinel with the
  field overridden.
- `errors.Is(err, arcp.ErrBudgetExhausted)` matches by code through
  wrapping (the `*Error.Is` method compares by `Code`).

## At the protocol boundary

The runtime uses `arcp.Code(err)` and `arcp.IsRetryable(err)` to
populate `job.error` and `session.error`. Returning a wrapped
sentinel from an `AgentFunc` is therefore the simplest way to surface
the right code:

```go
return nil, arcp.ErrPermissionDenied.WithMessage("model not in lease").WithCause(err)
```

Provisioners should return `credentials.BudgetExhausted` when an
upstream credential cap is depleted; the variable is aliased to
`arcp.ErrBudgetExhausted`, so callers see `BUDGET_EXHAUSTED` either
way.

---
title: errors
sdk: go
spec_sections: [§12]
order: 5
kind: reference
pkg_godoc: https://pkg.go.dev/github.com/agentruntimecontrolprotocol/go-sdk
---

# Reference: errors

The fifteen canonical codes plus sentinels:

| Sentinel                          | Code                            |
| --------------------------------- | ------------------------------- |
| `ErrPermissionDenied`             | `PERMISSION_DENIED`             |
| `ErrLeaseSubsetViolation`         | `LEASE_SUBSET_VIOLATION`        |
| `ErrJobNotFound`                  | `JOB_NOT_FOUND`                 |
| `ErrDuplicateKey`                 | `DUPLICATE_KEY`                 |
| `ErrAgentNotAvailable`            | `AGENT_NOT_AVAILABLE`           |
| `ErrAgentVersionNotAvailable`     | `AGENT_VERSION_NOT_AVAILABLE`   |
| `ErrCancelled`                    | `CANCELLED`                     |
| `ErrTimeout`                      | `TIMEOUT`                       |
| `ErrResumeWindowExpired`          | `RESUME_WINDOW_EXPIRED`         |
| `ErrHeartbeatLost`                | `HEARTBEAT_LOST`                |
| `ErrLeaseExpired`                 | `LEASE_EXPIRED`                 |
| `ErrBudgetExhausted`              | `BUDGET_EXHAUSTED`              |
| `ErrInvalidRequest`               | `INVALID_REQUEST`               |
| `ErrUnauthenticated`              | `UNAUTHENTICATED`               |
| `ErrInternalError`                | `INTERNAL_ERROR`                |

Helpers: `Code(err) ErrorCode`, `IsRetryable(err) bool`. Errors wrap
via `%w`; `errors.Is(err, ErrLeaseExpired)` matches by code.

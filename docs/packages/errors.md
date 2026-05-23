# Root package (`arcp`)

Import path: `github.com/agentruntimecontrolprotocol/go-sdk`

The root package exposes protocol primitives: envelopes, IDs, error
codes, feature flags, and lease types. It has no dependencies on the
client or server packages.

## Versioning

| Symbol | Value |
| --- | --- |
| `ProtocolVersion` | Wire-format version string (currently `"1.1"`). |
| `SDKVersion` | The Go module's semver string used as the default `ClientVersion`. |

## Envelope

| API | Purpose |
| --- | --- |
| `Envelope` | The wire frame — see [packages/messages](./messages.md) for the typed payloads. |
| `NewEnvelope(typ, payload)` | Build a typed envelope and marshal a payload. |
| `MarshalPayload(v)` | Encode any value to `json.RawMessage`; passes through `nil`, `[]byte`, and `json.RawMessage` unchanged. |
| `Envelope.DecodePayload(v)` | Decode the envelope payload into a struct. |
| `Envelope.Validate()` | Reject empty `ARCP`, `ID`, or `Type`. |
| `MessageType` interface | Implemented by typed payload structs registered in `messages/`. |
| `RegisterMessageType`, `NewPayloadForType`, `RegisteredTypes` | Dispatch table for routing envelopes by their `type` token. |

## IDs

The runtime mints ULIDs for everything; helpers cover every reserved
slot so callers can stay on the same id space:

| API | Slot |
| --- | --- |
| `NewEnvelopeID` | `envelope.id` |
| `NewSessionID` | `session_id` |
| `NewJobID` | `job_id` |
| `NewResultID` | `result_id` (for streamed results) |
| `NewPingNonce` | `session.ping.nonce` |
| `NewCallID` | tool call correlation |
| `NewTraceID` | 32-hex W3C trace id |
| `NewULID` | generic ULID, used by `resume_token` |

## Errors

| API | Purpose |
| --- | --- |
| `Error`, `ErrorCode` | Structured protocol errors. |
| `Error.WithCause`, `WithMessage`, `WithDetails` | Copy-and-override builders. |
| `Error.Is`, `Error.Unwrap` | Wire `errors.Is` / `errors.As` against sentinel codes and causes. |
| `Code(err)` | Extract the first `ErrorCode` from the error chain (defaults to `CodeInternalError`). |
| `IsRetryable(err)` | Read the `Retryable` flag (defaults to `true` for non-arcp errors). |
| `Newf(code, format, args...)` | Build an `*Error` with a formatted message and the default retryable flag for the code. |

The fifteen canonical codes and their sentinels are enumerated in
[guides/errors.md](../guides/errors.md).

## Features

| API | Purpose |
| --- | --- |
| `Features` | Default negotiable feature list. |
| `IntersectFeatures(a, b)` | Return the intersection of two feature lists. |
| `HasFeature(list, name)` | Predicate. |

## Leases

| API | Purpose |
| --- | --- |
| `Lease`, `Capability` | Lease maps and the reserved-capability constants (`CapFSRead`, `CapFSWrite`, `CapNetFetch`, `CapToolCall`, `CapAgentDelegate`, `CapModelUse`, `CapCostBudget`). |
| `Lease.Clone()` | Deep copy. |
| `BudgetAmount`, `ParseBudgetAmount`, `Currency` | `cost.budget` grammar (`CUR:decimal`). |
| `IsLeaseSubset(parent, child, parentRemaining, parentExpiry, childExpiry)` | Public subset check; non-nil return is always `*Error` with code `LEASE_SUBSET_VIOLATION` (see [Delegation](../guides/delegation.md)). |

Use sentinel errors with `errors.Is`:

```go
if errors.Is(err, arcp.ErrBudgetExhausted) {
    // upstream or runtime budget cap was reached
}
```

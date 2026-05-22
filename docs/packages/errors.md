# Root package and errors

Import path: `github.com/agentruntimecontrolprotocol/go-sdk`

The root package exposes protocol primitives: envelopes, IDs, error
codes, feature flags, and lease types.

| API | Purpose |
| --- | --- |
| `NewEnvelope` | Build a typed envelope and marshal a payload. |
| `Envelope.DecodePayload` | Decode an envelope payload into a message struct. |
| `Error`, `ErrorCode` | Structured protocol errors. |
| `Code`, `IsRetryable` | Boundary helpers for arbitrary errors. |
| `Lease`, `Capability` | Lease maps and reserved capability constants. |
| `Features` | Default negotiable feature list. |

Use sentinel errors with `errors.Is`:

```go
if errors.Is(err, arcp.ErrBudgetExhausted) {
	// upstream or runtime budget cap was reached
}
```

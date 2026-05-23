# Package `credentials`

Import path: `github.com/agentruntimecontrolprotocol/go-sdk/credentials`

`credentials` defines the provisioner surface used by runtimes that
support spec §9.8 provisioned credentials.

## Interface

```go
type Provisioner interface {
    Issue(ctx context.Context, req IssueRequest) ([]messages.Credential, error)
    Revoke(ctx context.Context, credentialID string) error
}

type IssueRequest struct {
    JobID       string
    Principal   string
    Agent       string
    Lease       arcp.Lease
    Budget      map[arcp.Currency]float64
    ExpiresAt   *time.Time
    ParentJobID string
}
```

| Symbol | Purpose |
| --- | --- |
| `Provisioner` | Issue and revoke lease-bound credentials. |
| `IssueRequest` | The job context the runtime supplies to `Issue`. |
| `NewMemory(prefix string) *Memory` | In-memory provisioner used by tests and examples. Issued credential IDs start with `prefix`. |
| `Memory.Outstanding()`, `Memory.Issued()`, `Memory.Revoked()` | Snapshot accessors for tests. |
| `BudgetExhausted` | Alias for `arcp.ErrBudgetExhausted`; return when an upstream per-credential cap is depleted. |
| `ErrNoRevocation` | Constructed `*arcp.Error` (`INTERNAL_ERROR`) signalling that the provisioner cannot expose a durable revocation path. |

Configure a provisioner with `server.Options.Provisioner`. The server
advertises `provisioned_credentials` **and** `model.use` only when a
provisioner is present; both are removed from the advertised set when
`Provisioner` is nil.

The runtime issues credentials inside the `job.accepted` envelope and
revokes them (with bounded retry/backoff) on every terminal state.
`JobContext.RotateCredential` mid-job updates the stored value and
emits a reserved `status` event with phase `credential_rotated`.

See [guides/leases.md](../guides/leases.md) and the
[provisioned-credentials example](../../examples/provisioned-credentials/)
for end-to-end usage.

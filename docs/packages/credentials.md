# Package `credentials`

Import path: `github.com/agentruntimecontrolprotocol/go-sdk/credentials`

`credentials` defines the provisioner interface used by runtimes that
support spec §9.8 provisioned credentials.

| API | Purpose |
| --- | --- |
| `Provisioner` | Issues lease-bound credentials and revokes them on job termination. |
| `IssueRequest` | Finalized job lease, principal, agent, budget, and expiry context. |
| `NewMemory` | Deterministic in-memory provisioner for tests and examples. |
| `BudgetExhausted` | Sentinel that maps upstream spend-cap failures to `BUDGET_EXHAUSTED`. |

Configure a provisioner with `server.Options.Provisioner`. The server
advertises `provisioned_credentials` only when a provisioner is present.

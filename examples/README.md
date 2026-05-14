# Examples

Each subdirectory is a self-contained scenario. The `server` and
`client` binaries are run separately; the smoke runner at
`internal/cmd/examplesmoke` exercises every example end-to-end.

| Directory             | Spec §        | Port  |
| --------------------- | ------------- | ----- |
| `submit-and-stream`   | §7.1, §8.2    | 7811  |
| `delegate`            | §10           | 7812  |
| `resume`              | §6.3          | 7813  |
| `idempotent-retry`    | §7.2          | 7814  |
| `lease-violation`     | §9.3          | 7815  |
| `cancel`              | §7.4          | 7816  |
| `stdio`               | §4.2          | n/a   |
| `vendor-extensions`   | §15           | 7818  |
| `custom-auth`         | §6.1          | 7819  |
| `heartbeat`           | §6.4          | 7820  |
| `ack-backpressure`    | §6.5          | 7821  |
| `list-jobs`           | §6.6          | 7822  |
| `subscribe`           | §7.6          | 7823  |
| `agent-versions`      | §7.5          | 7824  |
| `lease-expires-at`    | §9.5          | 7825  |
| `cost-budget`         | §9.6          | 7826  |
| `progress`            | §8.2.1        | 7827  |
| `result-chunk`        | §8.4          | 7828  |
| `tracing`             | §11           | 7829  |
| `nethttp-routes`      | §4.1          | 7830  |
| `chi-routes`          | §4.1          | 7831  |

Run an example:

```sh
go run ./examples/submit-and-stream/server &
go run ./examples/submit-and-stream/client
```

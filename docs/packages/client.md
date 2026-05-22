# Package `client`

Import path: `github.com/agentruntimecontrolprotocol/go-sdk/client`

`client` owns the caller side of a session: handshake, feature
negotiation, job submission, cancellation, subscriptions, list jobs,
ack, and result chunk collection.

Key types:

| Type | Purpose |
| --- | --- |
| `Client` | Connected ARCP session. |
| `Options` | Client identity, token, features, logger, auto-ack settings. |
| `SubmitRequest` | Agent, input, lease, idempotency key, timeout, trace ID. |
| `JobHandle` | Accepted job state, events, chunks, cancel, wait. |
| `Subscription` | Live observer for an existing job. |

Common flow:

```go
cli, err := client.Connect(ctx, t, client.Options{Token: token})
h, err := cli.Submit(ctx, client.SubmitRequest{Agent: "echo", Input: input})
res, err := h.Wait(ctx)
```

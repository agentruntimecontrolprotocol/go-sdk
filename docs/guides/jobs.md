# Jobs (§7)

Submit work through the client:

```go
h, err := cli.Submit(ctx, client.SubmitRequest{
	Agent: "researcher@1.2.0",
	Input: map[string]string{"topic": "leases"},
})
```

The runtime resolves the agent, creates a job, sends `job.accepted`,
then invokes the registered `server.AgentFunc`.

```go
srv.RegisterAgent("researcher", func(ctx context.Context, input json.RawMessage, jc *server.JobContext) (any, error) {
	return map[string]bool{"ok": true}, nil
})
```

Returning a value emits `job.result`. Returning an error emits
`job.error` with the ARCP code from `arcp.Code(err)`.

## Cancellation

Only the submitting session can cancel:

```go
err := h.Cancel(ctx, "user requested stop")
```

Agents should watch `ctx.Done()` and return promptly. The runtime emits
`CANCELLED` when cancellation wins the terminal state.

## Budgets

`cost.budget` entries live in the lease request:

```go
LeaseRequest: arcp.Lease{
	arcp.CapToolCall:   {"search.*"},
	arcp.CapCostBudget: {"USD:1.00"},
}
```

Agents emit cost metrics through `JobContext.Metric`; the runtime debits
matching budget currencies and reports `BUDGET_EXHAUSTED` on later
lease checks when a counter reaches zero.

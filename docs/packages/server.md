# Package `server`

Import path: `github.com/agentruntimecontrolprotocol/go-sdk/server`

`server` hosts ARCP agents and runtime sessions.

Key types:

| Type | Purpose |
| --- | --- |
| `Server` | Runtime instance; accepts transports and owns jobs. |
| `Options` | Runtime identity, auth verifier, feature set, logger, clock, provisioner. |
| `AgentFunc` | Function invoked for each accepted job. |
| `JobContext` | Agent-facing event, lease, budget, streaming, and credential API. |

Register agents before accepting sessions:

```go
srv := server.New(server.Options{Name: "runtime"})
srv.RegisterAgent("echo", func(ctx context.Context, input json.RawMessage, jc *server.JobContext) (any, error) {
	return map[string]json.RawMessage{"echo": input}, nil
})
```

Use `Options.Provisioner` to enable `provisioned_credentials`.

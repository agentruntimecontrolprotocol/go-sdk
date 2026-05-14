---
title: Quickstart
sdk: go
spec_sections: [§4.3, §6.2, §7.1, §8.2]
order: 2
kind: quickstart
---

# Quickstart

A single-process server + client over `MemoryTransport`:

```go
srv := server.New(server.Options{})
srv.RegisterAgent("echo", func(ctx context.Context, input json.RawMessage, jc *server.JobContext) (any, error) {
    return map[string]json.RawMessage{"echo": input}, nil
})

a, b := transport.NewMemoryPair()
ctx := context.Background()
go srv.Accept(ctx, b)
cli, _ := client.Connect(ctx, a, client.Options{Token: "demo"})
defer cli.Close(ctx)

h, _ := cli.Submit(ctx, client.SubmitRequest{
    Agent: "echo",
    Input: map[string]string{"hi": "there"},
})
res, _ := h.Wait(ctx)
fmt.Println(string(res.Output))
```

For a WebSocket-mounted server, see
[examples/nethttp-routes](../examples/nethttp-routes).

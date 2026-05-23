# Getting started

Install the module:

```sh
go get github.com/agentruntimecontrolprotocol/go-sdk@latest
```

Create an in-memory runtime and client:

```go
srv := server.New(server.Options{Name: "demo-runtime"})
srv.RegisterAgent("echo", func(ctx context.Context, input json.RawMessage, jc *server.JobContext) (any, error) {
	return map[string]json.RawMessage{"echo": input}, nil
})

a, b := transport.NewMemoryPair()
ctx := context.Background()
go srv.Accept(ctx, b)

cli, err := client.Connect(ctx, a, client.Options{Token: "demo"})
if err != nil {
	log.Fatal(err)
}
defer cli.Close(ctx)

h, err := cli.Submit(ctx, client.SubmitRequest{
	Agent: "echo",
	Input: map[string]string{"text": "hello"},
})
if err != nil {
	log.Fatal(err)
}
res, err := h.Wait(ctx)
if err != nil {
	log.Fatal(err)
}
log.Println(string(res.Output))
```

Use WebSocket for process boundaries by mounting
`middleware/nethttp.NewHandler` or `middleware/chi.Mount`, then
dialing with `transport.DialWebSocket`. The chi sub-package exposes
`Mount(router, srv, opts) *nethttp.Handler` instead of a separate
constructor, so the same `Handler.Shutdown` is available either way.

# Package `middleware/nethttp`

Import path: `github.com/agentruntimecontrolprotocol/go-sdk/middleware/nethttp`

`middleware/nethttp` mounts a `server.Server` on the standard library
HTTP stack.

```go
srv := server.New(server.Options{})
http.Handle("/arcp", nethttp.NewHandler(srv, nethttp.Options{}))
log.Fatal(http.ListenAndServe(":7826", nil))
```

Use it when your application already owns an `*http.Server` or
`http.ServeMux`.

# Package `middleware/chi`

Import path: `github.com/agentruntimecontrolprotocol/go-sdk/middleware/chi`

`middleware/chi` adapts a runtime to a `chi.Router`.

```go
r := chi.NewRouter()
r.Handle("/arcp", arcpchi.NewHandler(srv, arcpchi.Options{}))
```

Use it for Go services already organized around chi routes and
middleware.

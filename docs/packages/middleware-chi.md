# Package `middleware/chi`

Import path: `github.com/agentruntimecontrolprotocol/go-sdk/middleware/chi`

`middleware/chi` adapts a runtime to a `chi.Router` via a single
`Mount` helper. There is no separate `NewHandler`; `Mount` wraps
`middleware/nethttp.NewHandler` internally and binds it to the
router.

```go
import (
    arcpchi "github.com/agentruntimecontrolprotocol/go-sdk/middleware/chi"
    "github.com/agentruntimecontrolprotocol/go-sdk/server"
    "github.com/go-chi/chi/v5"
)

srv := server.New(server.Options{})
r := chi.NewRouter()
h := arcpchi.Mount(r, srv, arcpchi.Options{}) // h is *nethttp.Handler
```

## API

| Symbol | Purpose |
| --- | --- |
| `Options` | Type alias for `nethttp.Options` — see [middleware-nethttp](./middleware-nethttp.md) for the field list. |
| `Mount(r chi.Router, srv *server.Server, opts Options) *nethttp.Handler` | Attach the handler at `opts.Path` (default `"/arcp"`). The returned handle is usable for `Shutdown`. |

Use it for Go services already organized around chi routes and
middleware. Mount more than one runtime on the same router by giving
each a distinct `Options.Path`.

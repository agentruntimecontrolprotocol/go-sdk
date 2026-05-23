# Package `middleware/nethttp`

Import path: `github.com/agentruntimecontrolprotocol/go-sdk/middleware/nethttp`

`middleware/nethttp` mounts a `server.Server` on the standard library
HTTP stack via WebSocket upgrade.

```go
import (
    arcpnethttp "github.com/agentruntimecontrolprotocol/go-sdk/middleware/nethttp"
    "github.com/agentruntimecontrolprotocol/go-sdk/server"
)

srv := server.New(server.Options{})
http.Handle("/arcp", arcpnethttp.NewHandler(srv, arcpnethttp.Options{}))
log.Fatal(http.ListenAndServe(":7777", nil))
```

Use it when your application already owns an `*http.Server` or
`http.ServeMux`.

## API

| Symbol | Purpose |
| --- | --- |
| `NewHandler(srv *server.Server, opts Options) *Handler` | Build the handler. |
| `Handler.ServeHTTP` | The `http.Handler` implementation. |
| `Handler.Shutdown(ctx)` | Close every active WebSocket with status 1001 (Going Away) and wait until ctx expires. |

### `Options`

| Field | Default | Notes |
| --- | --- | --- |
| `Path` | `/arcp` | Request path served by the handler; other paths return 404. |
| `AllowedHosts` | `[localhost, 127.0.0.1, [::1]]` | DNS-rebind protection per spec §14. Requests with other `Host` headers return HTTP 421. |
| `ReadLimit` | `1 MiB` | Inbound WebSocket frame size cap. |
| `Subprotocols` | none | Forwarded to `websocket.Accept`. |
| `Origins` | nil | Allowed `Origin` patterns for browser clients; nil disables CORS. |
| `PingInterval` | `0` (disabled) | When > 0, sends WebSocket-layer pings at this cadence to keep idle connections alive through NAT/load balancer timeouts. Independent of the ARCP `session.ping` heartbeat. |

`NewHandler` only accepts `GET` (the WebSocket upgrade); other
methods return `405`. For graceful shutdown call
`server.Server.Close()` to terminate sessions, then
`Handler.Shutdown(ctx)` to close every still-open WebSocket.

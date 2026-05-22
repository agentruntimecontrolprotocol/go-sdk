# Transports

ARCP is transport neutral. This SDK ships three implementations of
`transport.Transport`.

| Transport | Use when |
| --- | --- |
| WebSocket | Client and runtime are separate processes or hosts. |
| stdio | A supervisor launches an agent/runtime process and speaks NDJSON. |
| in-memory | Tests, examples, and same-process embedding. |

## WebSocket

Mount a runtime with `middleware/nethttp` or `middleware/chi`:

```go
srv := server.New(server.Options{})
http.Handle("/arcp", nethttp.NewHandler(srv, nethttp.Options{}))
```

Dial with:

```go
t, err := transport.DialWebSocket(ctx, "ws://localhost:7826/arcp", transport.WebSocketOptions{})
```

## stdio

`transport.NewStdioTransport` wraps an `io.Reader` and `io.Writer` using one JSON
envelope per line. It is useful for command runners and sandboxes.

## in-memory

`transport.NewMemoryPair()` returns connected endpoints. The integration
tests use it to exercise the full client/server protocol without a
network listener.

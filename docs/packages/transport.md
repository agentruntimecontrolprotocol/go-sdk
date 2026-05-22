# Package `transport`

Import path: `github.com/agentruntimecontrolprotocol/go-sdk/transport`

`transport.Transport` is the envelope send/receive abstraction used by
both client and server.

Implementations:

| Function | Purpose |
| --- | --- |
| `DialWebSocket` | Client-side WebSocket transport. |
| `NewMemoryPair` | Connected in-process transports for tests and embedding. |
| `NewStdioTransport` | NDJSON envelope transport over reader/writer pairs. |

Custom transports only need to implement `Send`, `Recv`, and `Close`.

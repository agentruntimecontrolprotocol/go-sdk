---
title: transport package
sdk: go
spec_sections: [§4]
order: 3
kind: reference
pkg_godoc: https://pkg.go.dev/github.com/agentruntimecontrolprotocol/go-sdk/transport
---

# Reference: transport

- `Transport` interface — `Send / Recv / Close`.
- `NewMemoryPair() (Transport, Transport)`
- `DialWebSocket(ctx, url, opts)`
- `NewWebSocket(*websocket.Conn) Transport`
- `NewStdioTransport(in, out) Transport`

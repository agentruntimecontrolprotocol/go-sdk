---
title: client package
sdk: go
spec_sections: [§6, §7]
order: 1
kind: reference
pkg_godoc: https://pkg.go.dev/github.com/agentruntimecontrolprotocol/go-sdk/client
---

# Reference: client

The `client` package contains:

- `Connect(ctx, transport, Options) (*Client, error)`
- `Client.Submit / ListJobs / Subscribe / Ack / Close`
- `JobHandle.Events / Chunks / Wait / Cancel / CollectChunks`
- `Subscription.Events / Close / Err`

See [pkg.go.dev](https://pkg.go.dev/github.com/agentruntimecontrolprotocol/go-sdk/client) for full signatures.

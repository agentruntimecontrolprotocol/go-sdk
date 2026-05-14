---
title: server package
sdk: go
spec_sections: [§6, §7, §8, §9]
order: 2
kind: reference
pkg_godoc: https://pkg.go.dev/github.com/agentruntimecontrolprotocol/go-sdk/server
---

# Reference: server

`server.Server` hosts agents. Key surface:

- `New(Options) *Server`
- `Server.RegisterAgent / RegisterAgentVersion / SetDefaultAgentVersion`
- `Server.Accept(ctx, transport)`
- `AgentFunc(ctx, input, *JobContext) (any, error)`
- `JobContext.Log / Thought / ToolCall / ToolResult / Status / Metric / Progress / StreamResult / ValidateLeaseOp / Budget / Lease`

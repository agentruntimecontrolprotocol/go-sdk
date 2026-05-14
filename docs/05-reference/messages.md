---
title: messages package
sdk: go
spec_sections: [§5, §6, §7, §8]
order: 4
kind: reference
pkg_godoc: https://pkg.go.dev/github.com/agentruntimecontrolprotocol/go-sdk/messages
---

# Reference: messages

Typed payload structs and wire-type constants. Importing the package
registers every payload against the root envelope registry.

Key types:

- `SessionHello / SessionWelcome / SessionError / SessionBye`
- `SessionPing / SessionPong / SessionAck`
- `SessionListJobs / SessionJobs / JobInfo / ListJobsFilter`
- `JobSubmit / JobAccepted / JobEvent / JobResult / JobError / JobCancel`
- `JobSubscribe / JobSubscribed / JobUnsubscribe`
- Body shapes: `LogBody / ThoughtBody / ToolCallBody / ToolResultBody / StatusBody / MetricBody / ArtifactRefBody / DelegateBody / ProgressBody / ResultChunkBody`
- `ParseAgentRef(s) (AgentRef, error)`

---
title: Overview
sdk: go
spec_sections: [§1, §3]
order: 1
kind: overview
---

# Overview

ARCP is a transport-agnostic wire protocol for submitting, observing,
and controlling long-running AI agent jobs. The Go SDK ships:

- a typed envelope and message registry under the root package;
- a client and a server in `client/` and `server/`;
- three transports (memory, WebSocket, stdio);
- host-adapter middleware for `net/http`, `chi`, and OTel;
- an `arcp` CLI.

The full wire surface is documented in
[../spec/docs/draft-arcp-02.1.md](../../spec/docs/draft-arcp-02.1.md).

## Package map

| Import                                    | Purpose                                           |
| ----------------------------------------- | ------------------------------------------------- |
| `.../go-sdk`                              | Envelope + errors + ids + lease + feature consts. |
| `.../go-sdk/messages`                     | Typed payload structs registered against envelope.|
| `.../go-sdk/transport`                    | Transport interface + memory/ws/stdio impls.      |
| `.../go-sdk/auth`                         | Bearer verifier interface.                        |
| `.../go-sdk/client`                       | Connect, Submit, Subscribe, ListJobs, Ack.        |
| `.../go-sdk/server`                       | Hosts agents; runs sessions and jobs.             |
| `.../go-sdk/middleware/{nethttp,chi,otel}`| Host-adapter sub-packages.                        |
| `.../go-sdk/cmd/arcp`                     | CLI binary.                                       |

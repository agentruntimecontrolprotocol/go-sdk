# Changelog

## v1.0.0

Initial release.

- Envelope, fifteen-code error taxonomy, feature negotiation.
- Session lifecycle: hello / welcome / bye, heartbeats, ack, list_jobs.
- Job lifecycle: submit / accepted / event* / result | error / cancel.
- Subscribe across sessions with optional replay.
- Agent versioning (`name@version`), inventory, default resolution.
- Lease enforcement with optional `expires_at` and `cost.budget`.
- Event kinds: log, thought, tool_call, tool_result, status, metric,
  artifact_ref, delegate, progress, result_chunk.
- Streamed results via `JobContext.StreamResult`.
- Transports: in-memory, WebSocket (coder/websocket), stdio NDJSON.
- Middleware: nethttp, chi, otel.
- `arcp` CLI with `serve` and `submit` subcommands.

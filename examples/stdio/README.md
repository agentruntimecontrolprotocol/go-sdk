# stdio

Spec §4.2. Client spawns the agent as a subprocess and wraps its
stdio pipes via `transport.NewStdioTransport` for NDJSON framing.

# CLI

The SDK includes an `arcp` binary under `cmd/arcp`.

```sh
go run ./cmd/arcp --help
```

Use the CLI for local smoke tests and scripted protocol exercises. For
production embedding, prefer the `client`, `server`, and `transport`
packages directly so your application owns logging, auth, lifecycle,
and routing.

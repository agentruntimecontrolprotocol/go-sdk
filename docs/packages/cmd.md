# Package `cmd/arcp`

Import path: `github.com/agentruntimecontrolprotocol/go-sdk/cmd/arcp`

The `arcp` command is a sample CLI for local smoke tests and manual
protocol exercises. It registers a single `echo` agent on `serve`
and submits one job on `submit`; applications normally import
`client`, `server`, and `transport` directly rather than shelling out
to the CLI.

```sh
go install github.com/agentruntimecontrolprotocol/go-sdk/cmd/arcp@latest
arcp serve -h
arcp submit -h
```

See [cli.md](../cli.md) for the subcommand and flag reference.

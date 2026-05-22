# Conformance

The authoritative matrix is [`../CONFORMANCE.md`](../CONFORMANCE.md).
The executable checks live in [`../tests/conformance`](../tests/conformance/).

Run:

```sh
go test ./tests/conformance/...
```

Set `ARCP_CONFORMANCE_OUT=/path/to/conformance.json` to emit the
machine-readable summary.

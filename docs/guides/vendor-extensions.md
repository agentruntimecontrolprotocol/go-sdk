# Vendor extensions (§15)

Unknown envelope fields round-trip through `arcp.Envelope.Extra`.
Unknown event kinds and feature flags are ignored unless both sides
opt into them.

Use `x-vendor.<name>.<capability>` for custom lease namespaces:

```go
lease := arcp.Lease{
	"x-vendor.acme.email.send": {"tenant-a/*"},
}
```

Keep extension payloads JSON-compatible and document whether they are
session-scoped, job-scoped, or event-only. Do not use unprefixed names
for non-standard capabilities or event kinds.

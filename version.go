package arcp

// ProtocolVersion is the ARCP protocol revision implemented by this package.
// It is the value placed in the envelope's `arcp` field (RFC §6.1.1).
const ProtocolVersion = "1.0"

// ImplVersion is the semantic version of this Go implementation. It is
// distinct from ProtocolVersion: protocol revisions are coordinated across
// implementations, while ImplVersion tracks bug fixes and additions to this
// codebase. Used in `client.kind`/`runtime.kind` identity blocks (RFC §8.2,
// §8.3).
const ImplVersion = "0.1.0"

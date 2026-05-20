package arcp

// ProtocolVersion is the literal value carried in every envelope's
// "arcp" field.
const ProtocolVersion = "1.1"

// SDKVersion is the version of this Go SDK.
const SDKVersion = "1.0.0"

// Features is the canonical list of negotiable feature flags advertised
// in session.hello and session.welcome capabilities.
var Features = []string{
	"heartbeat",
	"ack",
	"list_jobs",
	"subscribe",
	"agent_versions",
	"lease_expires_at",
	"cost.budget",
	"progress",
	"result_chunk",
}

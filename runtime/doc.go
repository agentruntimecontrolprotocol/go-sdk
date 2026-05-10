// Package runtime is the server-side ARCP runtime: it accepts a
// transport.Transport, drives session handshakes (RFC §8), manages jobs
// (§10), streams (§11), human-in-the-loop interactions (§12),
// permissions and leases (§15), subscriptions (§13), and artifacts
// (§16). The Runtime is the top-level type; everything else is an
// implementation detail it owns.
package runtime

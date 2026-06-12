package lease

import (
	"time"

	arcp "github.com/agentruntimecontrolprotocol/go-sdk"
)

// IsSubset reports whether child is a subset of parent: every child
// capability appears in parent and every child pattern's language is
// covered by a parent pattern; budget patterns are compared numerically
// against parentRemaining.
//
// The subset and glob semantics live canonically in arcp.IsLeaseSubset
// / internal/glob; this is a thin wrapper so the runtime and the public
// helper cannot drift (#61).
func IsSubset(parent, child arcp.Lease, parentRemaining map[arcp.Currency]float64, parentExpiry, childExpiry *time.Time) error {
	return arcp.IsLeaseSubset(parent, child, parentRemaining, parentExpiry, childExpiry)
}

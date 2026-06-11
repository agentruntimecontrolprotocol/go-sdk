package lease

import (
	"time"

	arcp "github.com/agentruntimecontrolprotocol/go-sdk"
	"github.com/agentruntimecontrolprotocol/go-sdk/internal/glob"
)

// IsSubset reports whether child is a subset of parent: every child
// capability appears in parent and every child pattern is matched by
// at least one parent pattern. Budget patterns are compared
// numerically against parentRemaining.
func IsSubset(parent, child arcp.Lease, parentRemaining map[arcp.Currency]float64, parentExpiry, childExpiry *time.Time) error {
	for cap, patterns := range child {
		if cap == arcp.CapCostBudget {
			if err := checkBudgetSubset(patterns, parentRemaining); err != nil {
				return err
			}
			continue
		}
		// Non-budget capabilities, including model.use, flow through
		// the generic glob subset check unchanged.
		parentPatterns, ok := parent[cap]
		if !ok {
			return arcp.ErrLeaseSubsetViolation.WithMessage("child lease has capability " + string(cap) + " missing from parent")
		}
		for _, cp := range patterns {
			if !anyCovers(parentPatterns, cp) {
				return arcp.ErrLeaseSubsetViolation.WithMessage("child pattern " + cp + " not covered by parent " + string(cap))
			}
		}
	}
	if childExpiry != nil && parentExpiry != nil && childExpiry.After(*parentExpiry) {
		return arcp.ErrLeaseSubsetViolation.WithMessage("child expires_at exceeds parent")
	}
	return nil
}

// anyCovers reports whether at least one parent pattern's language
// includes every target matchable by the child pattern. It uses
// pattern-inclusion (glob.Covers), not glob matching, so a child whose
// own wildcards widen authority beyond the parent is rejected per
// §9.4.
func anyCovers(parents []string, child string) bool {
	for _, p := range parents {
		if glob.Covers(p, child) {
			return true
		}
	}
	return false
}

func checkBudgetSubset(child []string, parentRemaining map[arcp.Currency]float64) error {
	for _, pat := range child {
		amt, err := arcp.ParseBudgetAmount(pat)
		if err != nil {
			return err
		}
		remaining, ok := parentRemaining[amt.Currency]
		if !ok {
			return arcp.ErrLeaseSubsetViolation.WithMessage("child budget currency " + string(amt.Currency) + " not in parent")
		}
		if amt.Value > remaining {
			return arcp.ErrLeaseSubsetViolation.WithMessage("child budget " + pat + " exceeds parent remaining")
		}
	}
	return nil
}

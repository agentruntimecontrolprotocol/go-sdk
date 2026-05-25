package lease_test

import (
	"errors"
	"testing"

	arcp "github.com/agentruntimecontrolprotocol/go-sdk"
	"github.com/agentruntimecontrolprotocol/go-sdk/internal/lease"
)

func TestIsSubsetModelUse(t *testing.T) {
	parent := arcp.Lease{arcp.CapModelUse: {"tier-fast/*"}}
	child := arcp.Lease{arcp.CapModelUse: {"tier-fast/gpt-4o-mini"}}
	if err := lease.IsSubset(parent, child, nil, nil, nil); err != nil {
		t.Fatalf("expected subset, got %v", err)
	}
}

func TestIsSubsetModelUseExpanded(t *testing.T) {
	parent := arcp.Lease{arcp.CapModelUse: {"tier-fast/gpt-4o-mini"}}
	child := arcp.Lease{arcp.CapModelUse: {"tier-fast/*"}}
	if err := lease.IsSubset(parent, child, nil, nil, nil); !errors.Is(err, arcp.ErrLeaseSubsetViolation) {
		t.Fatalf("want LEASE_SUBSET_VIOLATION, got %v", err)
	}
}

func TestIsSubsetBudgetOK(t *testing.T) {
	parent := arcp.Lease{arcp.CapCostBudget: {"USD:10.00"}}
	child := arcp.Lease{arcp.CapCostBudget: {"USD:1.00"}}
	parentRem := map[arcp.Currency]float64{"USD": 10}
	if err := lease.IsSubset(parent, child, parentRem, nil, nil); err != nil {
		t.Fatalf("want subset OK, got %v", err)
	}
}

func TestIsSubsetBudgetExceeds(t *testing.T) {
	parent := arcp.Lease{arcp.CapCostBudget: {"USD:10.00"}}
	child := arcp.Lease{arcp.CapCostBudget: {"USD:20.00"}}
	parentRem := map[arcp.Currency]float64{"USD": 10}
	if err := lease.IsSubset(parent, child, parentRem, nil, nil); !errors.Is(err, arcp.ErrLeaseSubsetViolation) {
		t.Fatalf("want LEASE_SUBSET_VIOLATION, got %v", err)
	}
}

func TestIsSubsetBudgetMissingCurrency(t *testing.T) {
	parent := arcp.Lease{arcp.CapCostBudget: {"USD:10.00"}}
	child := arcp.Lease{arcp.CapCostBudget: {"EUR:1.00"}}
	parentRem := map[arcp.Currency]float64{"USD": 10}
	if err := lease.IsSubset(parent, child, parentRem, nil, nil); !errors.Is(err, arcp.ErrLeaseSubsetViolation) {
		t.Fatalf("want LEASE_SUBSET_VIOLATION, got %v", err)
	}
}

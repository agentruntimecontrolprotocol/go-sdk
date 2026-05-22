package lease_test

import (
	"errors"
	"sync"
	"testing"
	"time"

	arcp "github.com/agentruntimecontrolprotocol/go-sdk"
	"github.com/agentruntimecontrolprotocol/go-sdk/internal/lease"
)

func TestValidateOpPermission(t *testing.T) {
	st := lease.NewState(arcp.Lease{
		arcp.CapFSRead: {"/workspace/**"},
	}, nil)
	now := time.Now()
	if err := st.ValidateOp(now, arcp.CapFSRead, "/workspace/foo/bar.go"); err != nil {
		t.Fatal(err)
	}
	if err := st.ValidateOp(now, arcp.CapFSWrite, "/workspace/foo"); err == nil {
		t.Fatal("expected permission denied for fs.write")
	}
}

func TestValidateOpModelUse(t *testing.T) {
	st := lease.NewState(arcp.Lease{
		arcp.CapModelUse: {"tier-fast/*"},
	}, nil)
	if err := st.ValidateOp(time.Now(), arcp.CapModelUse, "tier-fast/gpt-4o-mini"); err != nil {
		t.Fatal(err)
	}
}

func TestValidateOpModelUseDenied(t *testing.T) {
	st := lease.NewState(arcp.Lease{
		arcp.CapModelUse: {"tier-fast/*"},
	}, nil)
	if err := st.ValidateOp(time.Now(), arcp.CapModelUse, "tier-deep/gpt-4o"); !errors.Is(err, arcp.ErrPermissionDenied) {
		t.Fatalf("want PERMISSION_DENIED, got %v", err)
	}
}

func TestExpiresAt(t *testing.T) {
	exp := time.Now().Add(50 * time.Millisecond)
	st := lease.NewState(arcp.Lease{
		arcp.CapFSRead: {"/x/**"},
	}, &exp)
	if err := st.ValidateOp(time.Now(), arcp.CapFSRead, "/x/y"); err != nil {
		t.Fatal(err)
	}
	if err := st.ValidateOp(exp.Add(time.Second), arcp.CapFSRead, "/x/y"); !errors.Is(err, arcp.ErrLeaseExpired) {
		t.Fatalf("want LEASE_EXPIRED, got %v", err)
	}
}

func TestBudgetAtomicity(t *testing.T) {
	st := lease.NewState(arcp.Lease{
		arcp.CapToolCall:   {"search.*"},
		arcp.CapCostBudget: {"USD:1.00"},
	}, nil)
	var success int
	var failure int
	var mu sync.Mutex
	var wg sync.WaitGroup
	wg.Add(64)
	for i := 0; i < 64; i++ {
		go func() {
			defer wg.Done()
			err := st.ValidateOp(time.Now(), arcp.CapToolCall, "search.web")
			if err != nil && !errors.Is(err, arcp.ErrBudgetExhausted) {
				return
			}
			if err == nil {
				if _, err := st.Debit("USD", 0.50); err != nil {
					return
				}
				mu.Lock()
				success++
				mu.Unlock()
			} else {
				mu.Lock()
				failure++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if success > 2 {
		t.Fatalf("budget allowed %d charges, want ≤ 2", success)
	}
}

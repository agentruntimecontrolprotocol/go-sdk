package lease_test

import (
	"errors"
	"sync"
	"sync/atomic"
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
	const charge = 0.50
	const expected = 2 // 1.00 / 0.50
	var success int64
	var exhausted int64
	var wg sync.WaitGroup
	const goroutines = 128
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			_, err := st.ValidateAndDebit(time.Now(), arcp.CapToolCall, "search.web", arcp.BudgetAmount{Currency: "USD", Value: charge})
			switch {
			case err == nil:
				atomic.AddInt64(&success, 1)
			case errors.Is(err, arcp.ErrBudgetExhausted):
				atomic.AddInt64(&exhausted, 1)
			default:
				t.Errorf("unexpected error: %v", err)
			}
		}()
	}
	wg.Wait()
	if got := atomic.LoadInt64(&success); got != expected {
		t.Fatalf("budget allowed %d charges, want exactly %d", got, expected)
	}
	if got := atomic.LoadInt64(&exhausted); got != goroutines-expected {
		t.Fatalf("got %d budget-exhausted rejections, want %d", got, goroutines-expected)
	}
	// And no debit drove counters negative.
	if rem := st.Budget()["USD"]; rem != 0 {
		t.Fatalf("remaining budget is %v, want 0", rem)
	}
}

func TestDebitRejectsOverspend(t *testing.T) {
	st := lease.NewState(arcp.Lease{
		arcp.CapCostBudget: {"USD:1.00"},
	}, nil)
	if _, err := st.Debit("USD", 0.60); err != nil {
		t.Fatalf("first debit: %v", err)
	}
	if _, err := st.Debit("USD", 0.60); !errors.Is(err, arcp.ErrBudgetExhausted) {
		t.Fatalf("want ErrBudgetExhausted, got %v", err)
	}
	if rem := st.Budget()["USD"]; rem <= 0 {
		t.Fatalf("rejected debit drove counter to %v, want positive", rem)
	}
}

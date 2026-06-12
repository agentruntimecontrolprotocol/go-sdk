// Package lease implements the lease enforcement primitives: glob
// match, target canonicalization, expires_at and budget checks.
package lease

import (
	"sync"
	"time"

	arcp "github.com/agentruntimecontrolprotocol/go-sdk"
	"github.com/agentruntimecontrolprotocol/go-sdk/internal/glob"
)

// State holds the runtime-mutable lease state for one job: the
// immutable lease, the optional expires_at deadline, and per-currency
// budget counters.
type State struct {
	mu          sync.Mutex
	lease       arcp.Lease
	expiresAt   *time.Time
	counters    map[arcp.Currency]float64
	initialized map[arcp.Currency]float64
}

// NewState builds a State from the lease grant and accept time.
func NewState(lease arcp.Lease, expiresAt *time.Time) *State {
	s := &State{
		lease:       lease.Clone(),
		expiresAt:   cloneTime(expiresAt),
		counters:    map[arcp.Currency]float64{},
		initialized: map[arcp.Currency]float64{},
	}
	for _, pat := range lease[arcp.CapCostBudget] {
		amt, err := arcp.ParseBudgetAmount(pat)
		if err != nil {
			continue
		}
		s.counters[amt.Currency] += amt.Value
		s.initialized[amt.Currency] = s.counters[amt.Currency]
	}
	return s
}

// Lease returns the immutable lease grant.
func (s *State) Lease() arcp.Lease {
	if s == nil {
		return nil
	}
	return s.lease.Clone()
}

// ExpiresAt returns the lease expiry, if any.
func (s *State) ExpiresAt() *time.Time {
	if s == nil {
		return nil
	}
	return cloneTime(s.expiresAt)
}

// Budget returns a snapshot of remaining per-currency budget.
func (s *State) Budget() map[arcp.Currency]float64 {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make(map[arcp.Currency]float64, len(s.counters))
	for k, v := range s.counters {
		out[k] = v
	}
	return out
}

// Initial returns the per-currency starting budget.
func (s *State) Initial() map[arcp.Currency]float64 {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make(map[arcp.Currency]float64, len(s.initialized))
	for k, v := range s.initialized {
		out[k] = v
	}
	return out
}

// HasBudget reports whether the lease has any cost.budget counters.
func (s *State) HasBudget() bool {
	if s == nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.initialized) > 0
}

// ValidateOp checks that op (capability, target) is allowed under the
// lease at time now. Returns nil on success or a structured arcp.Error.
//
// Budget gate: if the lease carries any budget counters, the operation
// is rejected with BUDGET_EXHAUSTED when ANY currency counter is <= 0,
// not only the currency relevant to this op. This matches §9.6 ("ops
// MUST fail once any counter is <= 0"); see also ValidateAndDebit.
func (s *State) ValidateOp(now time.Time, capability arcp.Capability, target string) error {
	if s == nil {
		return arcp.ErrPermissionDenied.WithMessage("no lease in scope")
	}
	if s.expiresAt != nil && !now.Before(*s.expiresAt) {
		return arcp.ErrLeaseExpired.WithMessage("lease expired at " + s.expiresAt.Format(time.RFC3339))
	}
	if !s.matches(capability, target) {
		return arcp.ErrPermissionDenied.WithMessage("operation not permitted by lease")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.initialized) > 0 {
		for cur, v := range s.counters {
			if v <= 0 {
				return arcp.ErrBudgetExhausted.WithMessage("budget exhausted for " + string(cur))
			}
		}
	}
	return nil
}

// ValidateAndDebit atomically checks capability, target, expiry, and
// (when debit.Value > 0) reserves the budget under one lock. It returns
// the remaining balance for debit.Currency on success. If applying the
// debit would drive the counter negative, the counter is unchanged and
// the call returns ErrBudgetExhausted. A zero-valued debit (Value == 0
// or empty Currency) is treated as a pure validation.
//
// Callers that perform budgeted work — for example an agent invoking a
// metered tool — should prefer this over the ValidateOp+Debit pair to
// avoid the time-of-check / time-of-use window where many goroutines
// can pass validation before the first debit reduces the shared
// counter.
func (s *State) ValidateAndDebit(now time.Time, capability arcp.Capability, target string, debit arcp.BudgetAmount) (float64, error) {
	if s == nil {
		return 0, arcp.ErrPermissionDenied.WithMessage("no lease in scope")
	}
	if debit.Value < 0 {
		return 0, arcp.ErrInvalidRequest.WithMessage("debit value must be non-negative")
	}
	if s.expiresAt != nil && !now.Before(*s.expiresAt) {
		return 0, arcp.ErrLeaseExpired.WithMessage("lease expired at " + s.expiresAt.Format(time.RFC3339))
	}
	if !s.matches(capability, target) {
		return 0, arcp.ErrPermissionDenied.WithMessage("operation not permitted by lease")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.initialized) > 0 {
		for cur, v := range s.counters {
			if v <= 0 {
				return 0, arcp.ErrBudgetExhausted.WithMessage("budget exhausted for " + string(cur))
			}
		}
	}
	if debit.Value == 0 || debit.Currency == "" {
		// Pure validation; nothing to debit.
		return s.counters[debit.Currency], nil
	}
	if _, ok := s.initialized[debit.Currency]; !ok {
		// Currency is unbudgeted; debits against it are no-ops.
		return 0, nil
	}
	if s.counters[debit.Currency]-debit.Value < 0 {
		return s.counters[debit.Currency], arcp.ErrBudgetExhausted.WithMessage("budget exhausted for " + string(debit.Currency))
	}
	s.counters[debit.Currency] -= debit.Value
	return s.counters[debit.Currency], nil
}

// Debit subtracts v from the named currency counter. Returns the
// remaining balance and any error. If the debit would drive the
// counter negative, the counter is unchanged and an
// ErrBudgetExhausted is returned.
//
// New callers should prefer ValidateAndDebit so the check-then-debit
// pair is atomic with the underlying capability validation.
func (s *State) Debit(cur arcp.Currency, v float64) (float64, error) {
	if v < 0 {
		return 0, arcp.ErrInvalidRequest.WithMessage("debit value must be non-negative")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.initialized[cur]; !ok {
		return 0, nil
	}
	if s.counters[cur]-v < 0 {
		return s.counters[cur], arcp.ErrBudgetExhausted.WithMessage("budget exhausted for " + string(cur))
	}
	s.counters[cur] -= v
	return s.counters[cur], nil
}

// Report applies an already-incurred cost against the named currency
// counter, per the §9.6 metric-reporting path. Unlike Debit and
// ValidateAndDebit (which authorize work before it happens and refuse
// to cross zero), the cost reported here has already been spent, so the
// counter MUST be decremented even when that drives it to or below
// zero. Once a counter reaches <= 0, the next lease-validated operation
// fails with BUDGET_EXHAUSTED.
//
// Unbudgeted currencies are ignored. The returned bool reports whether
// the currency was budgeted; the float is the resulting (possibly
// non-positive) remaining balance.
func (s *State) Report(cur arcp.Currency, v float64) (float64, bool) {
	if v < 0 {
		return 0, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.initialized[cur]; !ok {
		return 0, false
	}
	s.counters[cur] -= v
	return s.counters[cur], true
}

func (s *State) matches(cap arcp.Capability, target string) bool {
	patterns, ok := s.lease[cap]
	if !ok {
		return false
	}
	canon := canonicalizeTarget(cap, target)
	for _, p := range patterns {
		if glob.Match(p, canon) {
			return true
		}
	}
	return false
}

func canonicalizeTarget(cap arcp.Capability, target string) string {
	switch cap {
	case arcp.CapFSRead, arcp.CapFSWrite:
		return CanonicalizePath(target)
	case arcp.CapNetFetch:
		if u, err := CanonicalizeURL(target); err == nil {
			return u
		}
	case arcp.CapModelUse:
		return target
	}
	return target
}

func cloneTime(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	c := *t
	return &c
}

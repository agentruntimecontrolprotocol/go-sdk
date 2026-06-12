package arcp

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/agentruntimecontrolprotocol/go-sdk/internal/glob"
)

// Capability is the lease namespace identifier. The constants below
// enumerate spec-reserved namespaces; vendors may add their own
// string-valued capabilities.
type Capability string

// Reserved capability namespaces.
const (
	CapFSRead        Capability = "fs.read"
	CapFSWrite       Capability = "fs.write"
	CapNetFetch      Capability = "net.fetch"
	CapToolCall      Capability = "tool.call"
	CapAgentDelegate Capability = "agent.delegate"
	CapModelUse      Capability = "model.use"
	CapCostBudget    Capability = "cost.budget"
)

// Lease maps capability namespaces to pattern lists. The lease is
// immutable for a job's lifetime; budget counters are runtime state
// derived from the lease, not part of it.
type Lease map[Capability][]string

// Clone returns a deep copy of l.
func (l Lease) Clone() Lease {
	if l == nil {
		return nil
	}
	out := make(Lease, len(l))
	for k, v := range l {
		cp := make([]string, len(v))
		copy(cp, v)
		out[k] = cp
	}
	return out
}

// Currency is an ISO 4217 currency code, or the protocol-reserved
// "credits" string, or a runtime-defined identifier.
type Currency string

// BudgetAmount is one entry in a cost.budget pattern list, of the form
// "CUR:amount" (for example "USD:5").
type BudgetAmount struct {
	Currency Currency
	Value    float64
}

// String returns the canonical "CUR:value" representation. The value
// uses the shortest decimal that round-trips, so BudgetAmount{"USD", 5}
// renders as "USD:5" (not "USD:5.00") and {"USD", 5.5} as "USD:5.5".
func (b BudgetAmount) String() string {
	return fmt.Sprintf("%s:%s", b.Currency, strconv.FormatFloat(b.Value, 'f', -1, 64))
}

// ParseBudgetAmount parses a cost.budget pattern entry per the spec
// grammar: amount ::= currency ":" decimal. Negative values are
// rejected.
func ParseBudgetAmount(s string) (BudgetAmount, error) {
	i := strings.Index(s, ":")
	if i <= 0 || i == len(s)-1 {
		return BudgetAmount{}, ErrInvalidRequest.WithMessage("budget amount must be CUR:decimal")
	}
	cur := s[:i]
	dec := s[i+1:]
	if !isCurrency(cur) {
		return BudgetAmount{}, ErrInvalidRequest.WithMessage("budget currency must be uppercase A-Z or \"credits\"")
	}
	v, err := strconv.ParseFloat(dec, 64)
	if err != nil {
		return BudgetAmount{}, ErrInvalidRequest.WithMessage("budget value not a decimal: " + err.Error())
	}
	if v < 0 {
		return BudgetAmount{}, ErrInvalidRequest.WithMessage("budget value must be non-negative")
	}
	return BudgetAmount{Currency: Currency(cur), Value: v}, nil
}

func isCurrency(s string) bool {
	if s == "credits" {
		return true
	}
	if len(s) < 1 {
		return false
	}
	for _, r := range s {
		if r < 'A' || r > 'Z' {
			// Vendor-defined currencies may use any printable form.
			// ISO-4217-shaped uppercase is accepted by default;
			// deployments may widen this via their own policy.
			return allUppercaseLetters(s) || allLowercaseLetters(s)
		}
	}
	return true
}

func allUppercaseLetters(s string) bool {
	for _, r := range s {
		if r < 'A' || r > 'Z' {
			return false
		}
	}
	return len(s) > 0
}

func allLowercaseLetters(s string) bool {
	for _, r := range s {
		if r < 'a' || r > 'z' {
			return false
		}
	}
	return len(s) > 0
}

// MarshalJSON encodes l as a JSON object with string-typed keys.
func (l Lease) MarshalJSON() ([]byte, error) {
	if l == nil {
		return []byte("null"), nil
	}
	tmp := make(map[string][]string, len(l))
	for k, v := range l {
		tmp[string(k)] = v
	}
	return json.Marshal(tmp)
}

// UnmarshalJSON decodes a JSON object into l.
func (l *Lease) UnmarshalJSON(data []byte) error {
	var tmp map[string][]string
	if err := json.Unmarshal(data, &tmp); err != nil {
		return err
	}
	out := make(Lease, len(tmp))
	for k, v := range tmp {
		out[Capability(k)] = v
	}
	*l = out
	return nil
}

// IsLeaseSubset reports nil when child is a valid subset of parent
// per the spec §10 delegation rules: every child capability appears in
// parent, every child pattern is matched by at least one parent
// pattern, child cost.budget amounts fit within parentRemaining, and
// any child expires_at is at or before the parent's. A non-nil return
// is always *Error with Code == CodeLeaseSubsetViolation (or an
// underlying parse error for malformed budget amounts).
//
// parentRemaining maps currency to the remaining budget on the parent
// at the moment of the subset check; for a freshly issued lease this
// equals the initial budget. parentExpiry and childExpiry are
// optional. Passing nil for either skips the expiry comparison.
//
// Use this when implementing custom delegation flows on top of the
// SDK. Sub-job submission is not a first-class client API: callers that
// want to issue a child job over the wire must dial a second session and
// submit the child lease after verifying it with IsLeaseSubset.
//
// Pattern inclusion is decided by internal/glob.Covers, the single
// canonical glob implementation shared with the runtime's lease checks.
func IsLeaseSubset(parent, child Lease, parentRemaining map[Currency]float64, parentExpiry, childExpiry *time.Time) error {
	for ns, patterns := range child {
		if ns == CapCostBudget {
			for _, pat := range patterns {
				amt, err := ParseBudgetAmount(pat)
				if err != nil {
					return err
				}
				remaining, ok := parentRemaining[amt.Currency]
				if !ok {
					return ErrLeaseSubsetViolation.WithMessage("child budget currency " + string(amt.Currency) + " not in parent")
				}
				if amt.Value > remaining {
					return ErrLeaseSubsetViolation.WithMessage("child budget " + pat + " exceeds parent remaining")
				}
			}
			continue
		}
		parentPatterns, ok := parent[ns]
		if !ok {
			return ErrLeaseSubsetViolation.WithMessage("child lease has capability " + string(ns) + " missing from parent")
		}
		for _, cp := range patterns {
			matched := false
			for _, p := range parentPatterns {
				if glob.Covers(p, cp) {
					matched = true
					break
				}
			}
			if !matched {
				return ErrLeaseSubsetViolation.WithMessage("child pattern " + cp + " not covered by parent " + string(ns))
			}
		}
	}
	if childExpiry != nil && parentExpiry != nil && childExpiry.After(*parentExpiry) {
		return ErrLeaseSubsetViolation.WithMessage("child expires_at exceeds parent")
	}
	return nil
}

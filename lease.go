package arcp

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
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
// "USD:5.00".
type BudgetAmount struct {
	Currency Currency
	Value    float64
}

// String returns the canonical "CUR:value" representation.
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
			// Vendor-defined currencies may use any printable; we
			// accept ISO-4217-shaped uppercase by default and let
			// the runtime widen via deployment policy.
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

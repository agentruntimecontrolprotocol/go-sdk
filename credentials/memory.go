package credentials

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"sync"
	"time"

	arcp "github.com/agentruntimecontrolprotocol/go-sdk"
	"github.com/agentruntimecontrolprotocol/go-sdk/messages"
)

// Memory is a deterministic in-memory Provisioner for tests, examples,
// and local development.
type Memory struct {
	mu            sync.Mutex
	prefix        string
	next          int
	outstanding   map[string]messages.Credential
	issued        []IssueRequest
	revoked       []string
	revokedValues []string
}

// NewMemory returns an in-memory provisioner whose credential IDs are
// prefix + counter.
func NewMemory(prefix string) *Memory {
	return &Memory{
		prefix:      prefix,
		outstanding: map[string]messages.Credential{},
	}
}

// Issue returns one bearer credential scoped to req's budget, model,
// and expiration constraints.
func (m *Memory) Issue(_ context.Context, req IssueRequest) ([]messages.Credential, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.next++
	id := m.prefix + strconv.Itoa(m.next)
	cred := messages.Credential{
		ID:       id,
		Scheme:   "bearer",
		Value:    "memory-secret-" + id,
		Endpoint: "memory://credentials/" + id,
		Constraints: &messages.CredentialConstraints{
			CostBudget: budgetPatterns(req.Budget),
			ModelUse:   append([]string(nil), req.Lease[arcp.CapModelUse]...),
			ExpiresAt:  cloneTime(req.ExpiresAt),
		},
	}
	m.outstanding[id] = cred
	m.issued = append(m.issued, cloneIssueRequest(req))
	return []messages.Credential{cred}, nil
}

// Revoke removes credentialID from the outstanding set. It is
// idempotent so terminal cleanup can retry safely.
func (m *Memory) Revoke(_ context.Context, credentialID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.outstanding, credentialID)
	m.revoked = append(m.revoked, credentialID)
	return nil
}

// RevokePriorValue revokes only the prior value of a rotated credential
// (§9.8.2): the credential id stays outstanding with its new value and
// is only fully revoked at terminal cleanup. The prior value is
// recorded for inspection.
func (m *Memory) RevokePriorValue(_ context.Context, credentialID, priorValue string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.revokedValues = append(m.revokedValues, priorValue)
	return nil
}

// RevokedValues returns the prior credential values passed to
// RevokePriorValue in call order.
func (m *Memory) RevokedValues() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]string(nil), m.revokedValues...)
}

// Outstanding returns the number of credentials not yet revoked.
func (m *Memory) Outstanding() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.outstanding)
}

// Issued returns a snapshot of issue requests.
func (m *Memory) Issued() []IssueRequest {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]IssueRequest, len(m.issued))
	for i, req := range m.issued {
		out[i] = cloneIssueRequest(req)
	}
	return out
}

// Revoked returns credential IDs passed to Revoke in call order.
func (m *Memory) Revoked() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]string(nil), m.revoked...)
}

func budgetPatterns(budget map[arcp.Currency]float64) []string {
	if len(budget) == 0 {
		return nil
	}
	keys := make([]string, 0, len(budget))
	for cur := range budget {
		keys = append(keys, string(cur))
	}
	sort.Strings(keys)
	out := make([]string, 0, len(keys))
	for _, key := range keys {
		cur := arcp.Currency(key)
		out = append(out, fmt.Sprintf("%s:%s", cur, strconv.FormatFloat(budget[cur], 'f', -1, 64)))
	}
	return out
}

func cloneIssueRequest(req IssueRequest) IssueRequest {
	cp := req
	cp.Lease = req.Lease.Clone()
	cp.ExpiresAt = cloneTime(req.ExpiresAt)
	if req.Budget != nil {
		cp.Budget = make(map[arcp.Currency]float64, len(req.Budget))
		for k, v := range req.Budget {
			cp.Budget[k] = v
		}
	}
	return cp
}

func cloneTime(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	cp := *t
	return &cp
}

// Package credentials defines the provisioner surface used by runtimes
// to mint lease-bound credentials for accepted jobs.
package credentials

import (
	"context"
	"time"

	arcp "github.com/agentruntimecontrolprotocol/go-sdk"
	"github.com/agentruntimecontrolprotocol/go-sdk/messages"
)

// IssueRequest carries the finalized job lease and context a
// provisioner needs to mint scoped upstream credentials.
type IssueRequest struct {
	JobID       string
	Principal   string
	Agent       string
	Lease       arcp.Lease
	Budget      map[arcp.Currency]float64
	ExpiresAt   *time.Time
	ParentJobID string
}

// Provisioner issues credentials after job acceptance and revokes them
// when the job reaches a terminal state.
type Provisioner interface {
	Issue(ctx context.Context, req IssueRequest) ([]messages.Credential, error)
	Revoke(ctx context.Context, credentialID string) error
}

// BudgetExhausted maps upstream per-credential budget exhaustion to
// the ARCP boundary error.
var BudgetExhausted = arcp.ErrBudgetExhausted

// ErrNoRevocation signals that a provisioner cannot provide a
// revocation path acceptable for provisioned_credentials.
var ErrNoRevocation = arcp.Newf(arcp.CodeInternalError, "provisioner lacks durable revocation path")

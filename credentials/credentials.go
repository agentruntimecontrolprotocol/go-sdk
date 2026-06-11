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

// PriorValueRevoker is an optional Provisioner extension for §9.8.2
// credential rotation. On rotation the runtime installs a new value for
// an existing credential id and the PRIOR value MUST be revoked
// promptly while the credential id stays live with the new value.
//
// A Provisioner that implements this interface is asked to revoke only
// the prior value (RevokePriorValue); the credential id remains
// outstanding and is revoked wholesale via Revoke at terminal cleanup.
// A Provisioner that does NOT implement it is assumed to own prior-value
// revocation itself (e.g. the upstream mints and rotates the value), and
// the runtime will not call Revoke at rotation time — doing so would
// kill the freshly rotated-in credential.
type PriorValueRevoker interface {
	RevokePriorValue(ctx context.Context, credentialID, priorValue string) error
}

// BudgetExhausted maps upstream per-credential budget exhaustion to
// the ARCP boundary error.
var BudgetExhausted = arcp.ErrBudgetExhausted

// ErrNoRevocation signals that a provisioner cannot provide a
// revocation path acceptable for provisioned_credentials. Return it
// from Issue (to refuse minting credentials a runtime could not later
// revoke) or from Revoke (to report that no durable revocation path
// exists); a runtime observing it should reject the
// provisioned_credentials feature for the session rather than mint
// unrevocable credentials.
var ErrNoRevocation = arcp.Newf(arcp.CodeInternalError, "provisioner lacks durable revocation path")

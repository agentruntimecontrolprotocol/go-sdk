package messages

import (
	"time"

	"github.com/agentruntimecontrolprotocol/go-sdk"
)

// Wire type names for the permissions/leases group (RFC §6.2, §15).
const (
	TypePermissionRequest = "permission.request"
	TypePermissionGrant   = "permission.grant"
	TypePermissionDeny    = "permission.deny"
	TypeLeaseGranted      = "lease.granted"
	TypeLeaseExtended     = "lease.extended"
	TypeLeaseRevoked      = "lease.revoked"
	TypeLeaseRefresh      = "lease.refresh"
)

// PermissionRequest is the runtime's challenge to obtain a permission
// it does not yet hold (RFC §15.4).
type PermissionRequest struct {
	Permission            string `json:"permission"`
	Resource              string `json:"resource,omitempty"`
	Operation             string `json:"operation,omitempty"`
	Reason                string `json:"reason,omitempty"`
	RequestedLeaseSeconds int    `json:"requested_lease_seconds,omitempty"`
}

// ARCPType returns the wire type name.
func (PermissionRequest) ARCPType() string { return TypePermissionRequest }

// PermissionGrant grants the requested permission (RFC §15.4).
type PermissionGrant struct {
	Permission   string `json:"permission"`
	Resource     string `json:"resource,omitempty"`
	Operation    string `json:"operation,omitempty"`
	LeaseSeconds int    `json:"lease_seconds,omitempty"`
}

// ARCPType returns the wire type name.
func (PermissionGrant) ARCPType() string { return TypePermissionGrant }

// PermissionDeny denies the requested permission (RFC §15.4).
type PermissionDeny struct {
	Permission string `json:"permission"`
	Reason     string `json:"reason,omitempty"`
}

// ARCPType returns the wire type name.
func (PermissionDeny) ARCPType() string { return TypePermissionDeny }

// LeaseGranted reports a granted lease (RFC §15.5).
type LeaseGranted struct {
	Lease
}

// ARCPType returns the wire type name.
func (LeaseGranted) ARCPType() string { return TypeLeaseGranted }

// LeaseExtended reports a successful lease extension (RFC §15.5).
type LeaseExtended struct {
	LeaseID   arcp.LeaseID `json:"lease_id"`
	ExpiresAt time.Time    `json:"expires_at"`
}

// ARCPType returns the wire type name.
func (LeaseExtended) ARCPType() string { return TypeLeaseExtended }

// LeaseRevoked indicates the grantor terminated the lease early
// (RFC §15.5).
type LeaseRevoked struct {
	LeaseID arcp.LeaseID   `json:"lease_id"`
	Code    arcp.ErrorCode `json:"code,omitempty"`
	Reason  string         `json:"reason,omitempty"`
}

// ARCPType returns the wire type name.
func (LeaseRevoked) ARCPType() string { return TypeLeaseRevoked }

// LeaseRefresh requests a lease extension before expiry (RFC §15.5).
type LeaseRefresh struct {
	LeaseID               arcp.LeaseID `json:"lease_id"`
	RequestedExtraSeconds int          `json:"requested_extra_seconds,omitempty"`
}

// ARCPType returns the wire type name.
func (LeaseRefresh) ARCPType() string { return TypeLeaseRefresh }

func init() {
	register(TypePermissionRequest, func() arcp.MessageType { return &PermissionRequest{} })
	register(TypePermissionGrant, func() arcp.MessageType { return &PermissionGrant{} })
	register(TypePermissionDeny, func() arcp.MessageType { return &PermissionDeny{} })
	register(TypeLeaseGranted, func() arcp.MessageType { return &LeaseGranted{} })
	register(TypeLeaseExtended, func() arcp.MessageType { return &LeaseExtended{} })
	register(TypeLeaseRevoked, func() arcp.MessageType { return &LeaseRevoked{} })
	register(TypeLeaseRefresh, func() arcp.MessageType { return &LeaseRefresh{} })
}

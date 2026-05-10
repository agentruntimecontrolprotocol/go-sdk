package messages

import (
	"time"

	"github.com/fizzpop/arcp-go"
)

// Wire type names for the artifacts group (RFC §6.2, §16).
const (
	TypeArtifactPut     = "artifact.put"
	TypeArtifactFetch   = "artifact.fetch"
	TypeArtifactRef     = "artifact.ref"
	TypeArtifactRelease = "artifact.release"
)

// ArtifactRef is the canonical pointer to a stored artifact
// (RFC §16.1). May appear inside other payloads.
type ArtifactRef struct {
	ArtifactID arcp.ArtifactID `json:"artifact_id"`
	URI        string          `json:"uri,omitempty"`
	MediaType  string          `json:"media_type,omitempty"`
	Size       int64           `json:"size,omitempty"`
	Sha256     string          `json:"sha256,omitempty"`
	ExpiresAt  time.Time       `json:"expires_at,omitempty"`
}

// ARCPType returns the wire type name. Allows ArtifactRef to be sent
// directly as a payload (e.g. when embedded in subscribe.event).
func (ArtifactRef) ARCPType() string { return TypeArtifactRef }

// ArtifactPut uploads an artifact (RFC §16.2). v0.1 supports inline
// base64 only.
type ArtifactPut struct {
	ArtifactID arcp.ArtifactID `json:"artifact_id,omitempty"`
	MediaType  string          `json:"media_type,omitempty"`
	Data       string          `json:"data,omitempty"` // base64
	Sha256     string          `json:"sha256,omitempty"`
	RetentionS int             `json:"retention_seconds,omitempty"`
}

// ARCPType returns the wire type name.
func (ArtifactPut) ARCPType() string { return TypeArtifactPut }

// ArtifactFetch requests an artifact by id (RFC §16.2).
type ArtifactFetch struct {
	ArtifactID arcp.ArtifactID `json:"artifact_id"`
}

// ARCPType returns the wire type name.
func (ArtifactFetch) ARCPType() string { return TypeArtifactFetch }

// ArtifactRelease signals the holder no longer needs the artifact
// (RFC §16.2). The runtime MAY garbage-collect.
type ArtifactRelease struct {
	ArtifactID arcp.ArtifactID `json:"artifact_id"`
}

// ARCPType returns the wire type name.
func (ArtifactRelease) ARCPType() string { return TypeArtifactRelease }

func init() {
	register(TypeArtifactPut, func() arcp.MessageType { return &ArtifactPut{} })
	register(TypeArtifactFetch, func() arcp.MessageType { return &ArtifactFetch{} })
	register(TypeArtifactRef, func() arcp.MessageType { return &ArtifactRef{} })
	register(TypeArtifactRelease, func() arcp.MessageType { return &ArtifactRelease{} })
}

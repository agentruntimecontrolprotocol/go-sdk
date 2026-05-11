// Cheap-tier first; escalate to deep tier via agent.handoff.
package main

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"

	arcp "github.com/agentruntimecontrolprotocol/go-sdk"
	"github.com/agentruntimecontrolprotocol/go-sdk/messages"
)

const (
	confidenceThreshold = 0.65
	cheapURL            = "wss://haiku-pool.tier1.internal"
	deepURL             = "wss://opus-pool.tier3.internal"
	deepKind            = "arcp-opus-pool"
	deepFingerprint     = "sha256:0a37bf7d61cca21f00..." // pinned
)

type Transcript struct {
	UserRequest     string                   `json:"user_request"`
	Transcript      []map[string]interface{} `json:"transcript"`
	CheapConfidence float64                  `json:"cheap_confidence"`
}

func packageContext(ctx context.Context, c *Session, t Transcript) (*messages.ArtifactRef, error) {
	body, err := json.Marshal(t)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(body)
	reply, err := c.Request(ctx, &arcp.Envelope{
		Payload: &messages.ArtifactPut{
			ArtifactID: arcp.NewArtifactID(),
			MediaType:  "application/json",
			Data:       base64.StdEncoding.EncodeToString(body),
			SHA256:     hex.EncodeToString(sum[:]),
		},
	})
	if err != nil {
		return nil, err
	}
	ref, ok := reply.Payload.(*messages.ArtifactRef)
	if !ok {
		return nil, arcp.NewError(arcp.CodeInternal, "got "+reply.Type())
	}
	return ref, nil
}

func emitHandoff(ctx context.Context, c *Session,
	ref *messages.ArtifactRef, traceID arcp.TraceID,
) error {
	// Spec gestures at shared_memory_ref (RFC §14); we attach it via
	// extensions so the deep tier knows where the transcript lives.
	refBytes, _ := json.Marshal(ref)
	return c.Send(ctx, &arcp.Envelope{
		TraceID: traceID,
		Extensions: map[string]json.RawMessage{
			"arcpx.handoff.shared_memory_ref.v1": refBytes,
		},
		Payload: &messages.AgentHandoff{
			TargetRuntime: messages.RuntimeIdentity{
				Kind:        deepKind,
				Fingerprint: deepFingerprint,
			},
			SessionID: c.SessionID(),
		},
	})
}

func main() {
	ctx := context.Background()
	cheap, accepted := openCheap(ctx, cheapURL)
	defer cheap.Close(ctx)

	// Pin runtime kind + fingerprint (RFC §8.3); refuse on mismatch.
	if accepted.Runtime.Kind != "arcp-haiku-pool" {
		log.Fatal(arcp.NewError(arcp.CodeUnauthenticated, "cheap kind mismatch"))
	}

	request := "what does CRDT stand for?"
	traceID := arcp.NewTraceID()

	answer, confidence := attempt(ctx, request)
	if confidence >= confidenceThreshold {
		fmt.Println(answer)
		return
	}

	ref, err := packageContext(ctx, cheap, Transcript{
		UserRequest: request,
		Transcript: []map[string]interface{}{
			{"role": "user", "content": request},
			{"role": "assistant", "content": answer},
		},
		CheapConfidence: confidence,
	})
	if err != nil {
		log.Fatal(err)
	}
	if err := emitHandoff(ctx, cheap, ref, traceID); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("[handed off to %s trace_id=%s]\n", deepKind, traceID)
}

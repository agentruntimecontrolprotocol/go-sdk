// Generator proposes; reviewer holds veto via permission.request.
package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log"

	arcp "github.com/agentruntimecontrolprotocol/go-sdk"
	"github.com/agentruntimecontrolprotocol/go-sdk/messages"
)

const maxRevisions = 4

func fingerprint(diff string) string {
	sum := sha256.Sum256([]byte(diff))
	return hex.EncodeToString(sum[:])[:16]
}

// requestApply asks for a repo.write lease scoped to this exact diff.
// Identical patches dedupe at the runtime via idempotency_key.
func requestApply(ctx context.Context, c *Session,
	ticketID string, patch Patch,
) (arcp.LeaseID, error) {
	fp := fingerprint(patch.Diff)
	reply, err := c.Request(ctx, &arcp.Envelope{
		IdempotencyKey: fmt.Sprintf("review:%s:%s", ticketID, fp),
		Payload: &messages.PermissionRequest{
			Permission:            "repo.write",
			Resource:              fmt.Sprintf("ticket:%s/%s", ticketID, fp),
			Operation:             "apply_patch",
			Reason:                "apply patch",
			RequestedLeaseSeconds: 90,
		},
	})
	if err != nil {
		return "", err
	}
	switch p := reply.Payload.(type) {
	case *messages.LeaseGranted:
		return p.Lease.LeaseID, nil
	case *messages.PermissionDeny:
		return "", arcp.NewError(arcp.CodePermissionDenied, p.Reason)
	default:
		return "", arcp.NewError(arcp.CodeFailedPrecondition,
			"unexpected: "+reply.Type())
	}
}

// respond is the reviewer side: grant or typed deny.
func respond(ctx context.Context, reviewer *Session,
	request arcp.Envelope, verdict ReviewVerdict,
) error {
	req := request.Payload.(*messages.PermissionRequest)
	if verdict.Grant {
		return reviewer.Send(ctx, &arcp.Envelope{
			CorrelationID: request.ID,
			Payload: &messages.PermissionGrant{
				Permission:   req.Permission,
				Resource:     req.Resource,
				Operation:    req.Operation,
				LeaseSeconds: 90,
			},
		})
	}
	return reviewer.Send(ctx, &arcp.Envelope{
		CorrelationID: request.ID,
		Payload: &messages.PermissionDeny{
			Permission: req.Permission,
			Reason:     verdict.Reason,
		},
	})
}

func reviewerLoop(ctx context.Context, reviewer *Session, ticket string) {
	for env := range reviewer.Events(ctx) {
		if _, ok := env.Payload.(*messages.PermissionRequest); ok {
			v := review(ctx, ticket, env)
			if err := respond(ctx, reviewer, env, v); err != nil {
				log.Printf("respond: %v", err)
			}
		}
	}
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Two sessions, one per agent. In production they'd be separate
	// processes on separate runtimes; the message contract is identical.
	generator := openGenerator(ctx)
	reviewer := openReviewer(ctx)
	defer generator.Close(ctx)
	defer reviewer.Close(ctx)

	ticketID := "JIRA-4812"
	ticket := "Reject JWTs whose `aud` does not match the configured " +
		"audience. Add a unit test."

	go reviewerLoop(ctx, reviewer, ticket)

	var priorDenial string
	for i := 0; i < maxRevisions; i++ {
		patch := propose(ctx, ticket, priorDenial)
		lease, err := requestApply(ctx, generator, ticketID, patch)
		var ae *arcp.Error
		if errors.As(err, &ae) && ae.Code == arcp.CodePermissionDenied {
			priorDenial = ae.Message
			continue
		}
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("applied %s lease=%s\n", fingerprint(patch.Diff), lease)
		return
	}
	fmt.Println("abandoned after max_revisions")
}

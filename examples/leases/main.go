// Sandboxed on-call agent. Lease-gated shell, reasoning streamed.
package main

import (
	"context"
	"errors"
	"fmt"
	"log"

	arcp "github.com/agentruntimecontrolprotocol/go-sdk"
	"github.com/agentruntimecontrolprotocol/go-sdk/messages"
)

var (
	readBinaries = map[string]struct{}{
		"/usr/bin/journalctl": {}, "/usr/bin/cat": {},
		"/usr/bin/ss": {}, "/usr/bin/ps": {},
	}
	writeBinaries = map[string]struct{}{
		"/usr/bin/systemctl": {}, "/usr/bin/kill": {},
	}
)

const (
	readLeaseSeconds  = 30 * 60
	writeLeaseSeconds = 60
)

// classify returns (permission, resource, operation, lease seconds).
func classify(argv []string, host string) (string, string, string, int, error) {
	bin := argv[0]
	if _, ok := readBinaries[bin]; ok {
		return "host.read", "host:" + host, "read", readLeaseSeconds, nil
	}
	if _, ok := writeBinaries[bin]; ok {
		target := argv[1]
		if bin == "/usr/bin/systemctl" && len(argv) > 2 {
			target = argv[2]
		}
		res := fmt.Sprintf("host:%s/%s/%s", host, bin, target)
		return "host.write", res, "write", writeLeaseSeconds, nil
	}
	return "", "", "", 0, arcp.NewError(
		arcp.CodePermissionDenied, "binary not allowed: "+bin)
}

func acquireLease(ctx context.Context, c *Session,
	permission, resource, operation, reason string, seconds int,
) (arcp.LeaseID, error) {
	reply, err := c.Request(ctx, &arcp.Envelope{
		Payload: &messages.PermissionRequest{
			Permission:            permission,
			Resource:              resource,
			Operation:             operation,
			Reason:                reason,
			RequestedLeaseSeconds: seconds,
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
			"unexpected reply: "+reply.Type())
	}
}

func runCommand(ctx context.Context, c *Session,
	argv []string, reason, host string,
) (string, error) {
	perm, res, op, secs, err := classify(argv, host)
	if err != nil {
		return "", err
	}
	lease, err := acquireLease(ctx, c, perm, res, op, reason, secs)
	if err != nil {
		return "", err
	}
	// Lease is the only guard. Spawn the subprocess elsewhere.
	return fmt.Sprintf("<would run %v under lease %s>", argv, lease), nil
}

func emitThought(ctx context.Context, c *Session,
	streamID arcp.StreamID, seq int, text string,
) error {
	return c.Send(ctx, &arcp.Envelope{
		StreamID: streamID,
		Payload: &messages.StreamChunk{
			Sequence: seq,
			Role:     "assistant_thought",
			Content:  text,
		},
	})
}

func main() {
	ctx := context.Background()
	c := openConstrained(ctx) // identity (constrained), auth elided
	defer c.Close(ctx)

	streamID := arcp.NewStreamID()
	if err := c.Send(ctx, &arcp.Envelope{
		StreamID: streamID,
		Payload:  &messages.StreamOpen{Kind: messages.StreamKindThought},
	}); err != nil {
		log.Fatal(err)
	}

	seq := 0
	for step := range llmLoop(ctx, "api-gateway pod is OOMing every 4 minutes") {
		if err := emitThought(ctx, c, streamID, seq, step.Thought); err != nil {
			log.Fatal(err)
		}
		seq++
		if step.ToolCall != nil {
			_, err := runCommand(ctx, c,
				step.ToolCall.Argv, step.ToolCall.Reason, "edge-pod-04")
			var ae *arcp.Error
			if errors.As(err, &ae) && ae.Code == arcp.CodePermissionDenied {
				continue // PERMISSION_DENIED feeds back into the next prompt
			}
			if err != nil {
				log.Fatal(err)
			}
		}
		if step.Final != "" {
			fmt.Println(step.Final)
			break
		}
	}
}

// Durable research job with real crash and resume.
//
//	# First call: crash after `synthesize`. Prints the resume token.
//	CRASH_AFTER_STEP=synthesize go run ./examples/resumability
//
//	# Second call: pick up from the printed checkpoint.
//	RESUME_JOB_ID=... RESUME_AFTER_MSG_ID=... RESUME_CHECKPOINT_ID=... \
//	  go run ./examples/resumability
package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"slices"

	arcp "github.com/agentruntimecontrolprotocol/go-sdk"
	"github.com/agentruntimecontrolprotocol/go-sdk/messages"
)

var steps = []string{"plan", "gather", "synthesize", "critique", "finalize"}

// stepKey is a deterministic per-step idempotency key (RFC §6.4).
// Re-issuing the same step with the same input returns the prior
// outcome instead of re-running the LLM.
func stepKey(jobID arcp.JobID, step, salt string) string {
	h := sha256.New()
	for _, p := range []string{string(jobID), step, salt} {
		h.Write([]byte(p))
		h.Write([]byte{0})
	}
	return fmt.Sprintf("research:%s:%s:%s", jobID, step, hex.EncodeToString(h.Sum(nil))[:16])
}

func emitProgress(ctx context.Context, c *Session, jid arcp.JobID, step string) error {
	pct := 100.0 * float64(slices.Index(steps, step)+1) / float64(len(steps))
	return c.Send(ctx, &arcp.Envelope{
		JobID:   jid,
		Payload: &messages.JobProgress{Percent: pct, Message: step},
	})
}

func emitCheckpoint(ctx context.Context, c *Session, jid arcp.JobID, step string) (arcp.CheckpointID, error) {
	chk := arcp.CheckpointID(fmt.Sprintf("chk_%s_%s", step, jid[len(jid)-6:]))
	return chk, c.Send(ctx, &arcp.Envelope{
		JobID:   jid,
		Payload: &messages.JobCheckpoint{CheckpointID: chk, Label: step},
	})
}

func executeSteps(ctx context.Context, c *Session,
	jid arcp.JobID, request any, startingAt, crashAfter string,
) (any, error) {
	output := request
	for _, step := range steps {
		if slices.Index(steps, step) < slices.Index(steps, startingAt) {
			continue
		}
		key := stepKey(jid, step, fmt.Sprintf("%v", output))
		_ = emitProgress(ctx, c, jid, step)
		out, err := runStep(ctx, c, jid, step, map[string]any{
			"prior":           output,
			"idempotency_key": key,
		})
		if err != nil {
			return nil, err
		}
		output = out
		_, _ = emitCheckpoint(ctx, c, jid, step)
		if crashAfter == step {
			fmt.Printf("[crash after %s; resume with "+
				"RESUME_JOB_ID=%s "+
				"RESUME_CHECKPOINT_ID=chk_%s_%s "+
				"RESUME_AFTER_MSG_ID=<last id from your event log>]\n",
				step, jid, step, jid[len(jid)-6:])
			os.Exit(137) // process death is fine; runtime kept everything
		}
	}
	return output, nil
}

func issueResume(ctx context.Context, c *Session,
	jid arcp.JobID, after arcp.MessageID, chk arcp.CheckpointID,
) (string, error) {
	if err := c.Send(ctx, &arcp.Envelope{
		JobID: jid,
		Payload: &messages.Resume{
			AfterMessageID:     after,
			CheckpointID:       chk,
			IncludeOpenStreams: true,
		},
	}); err != nil {
		return "", err
	}
	var last string
	for env := range c.Events(ctx) {
		if env.JobID != jid {
			continue
		}
		if te, ok := env.Payload.(*messages.ToolError); ok && te.Code == arcp.CodeDataLoss {
			return "", arcp.NewError(arcp.CodeDataLoss, "retention expired")
		}
		switch p := env.Payload.(type) {
		case *messages.JobCheckpoint:
			last = p.Label
		case *messages.JobCompleted, *messages.JobFailed, *messages.JobCancelled:
			return "", nil
		case *messages.EventEmit:
			if p.Type == "subscription.backfill_complete" {
				return last, nil // replay window closed; we're now live
			}
		}
	}
	return last, nil
}

func main() {
	ctx := context.Background()
	c := openClient(ctx)
	defer c.Close(ctx)

	if rj := os.Getenv("RESUME_JOB_ID"); rj != "" {
		jid := arcp.JobID(rj)
		after := arcp.MessageID(os.Getenv("RESUME_AFTER_MSG_ID"))
		chk := arcp.CheckpointID(os.Getenv("RESUME_CHECKPOINT_ID"))
		last, err := issueResume(ctx, c, jid, after, chk)
		if err != nil {
			log.Fatal(err)
		}
		if last == "" {
			fmt.Println("already terminal during replay")
			return
		}
		next := slices.Index(steps, last) + 1
		if next >= len(steps) {
			fmt.Println("nothing to resume")
			return
		}
		fmt.Printf("[resuming at %s]\n", steps[next])
		final, err := executeSteps(ctx, c, jid, "<replayed>", steps[next], "")
		if err != nil {
			log.Fatal(err)
		}
		val, _ := json.Marshal(final)
		_ = c.Send(ctx, &arcp.Envelope{
			JobID:   jid,
			Payload: &messages.JobCompleted{Value: val},
		})
		return
	}

	jid := arcp.NewJobID()
	request := "Survey CRDT-based collaborative editing in 2026."
	_ = c.Send(ctx, &arcp.Envelope{
		JobID: jid,
		Payload: &messages.WorkflowStart{
			Workflow: "research.v1",
			Inputs:   map[string]any{"request": request},
		},
	})
	final, err := executeSteps(ctx, c, jid, request, steps[0],
		os.Getenv("CRASH_AFTER_STEP"))
	if err != nil {
		log.Fatal(err)
	}
	val, _ := json.Marshal(final)
	_ = c.Send(ctx, &arcp.Envelope{
		JobID: jid, Payload: &messages.JobCompleted{Value: val},
	})
	fmt.Printf("job_id=%s\n%v\n", jid, final)
}

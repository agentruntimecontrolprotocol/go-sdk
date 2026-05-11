// Two scenarios over the §10.4 / §10.5 control surface.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	arcp "github.com/agentruntimecontrolprotocol/go-sdk"
	"github.com/agentruntimecontrolprotocol/go-sdk/messages"
)

const cancelDeadline = 5000 * time.Millisecond

func startLongJob(ctx context.Context, c *Session) (arcp.JobID, error) {
	resp, err := c.Request(ctx, &arcp.Envelope{
		Payload: &messages.ToolInvoke{
			Tool:      "demo.long_running",
			Arguments: map[string]any{"work_seconds": 600},
		}})
	if err != nil {
		return "", err
	}
	return resp.Payload.(*messages.JobAccepted).JobID, nil
}

// cancelJob is the cooperative path. Runtime drives the target to a
// clean checkpoint inside `deadline_ms` before terminating; escalates
// to ABORTED on timeout (RFC §10.4).
func cancelJob(ctx context.Context, c *Session,
	jid arcp.JobID, reason string, deadlineMs int,
) (*arcp.Envelope, error) {
	reply, err := c.Request(ctx, &arcp.Envelope{
		Payload: &messages.Cancel{
			Target: messages.CancelTargetJob, TargetID: string(jid),
			Reason: reason, DeadlineMilliseconds: deadlineMs,
		}})
	if err != nil {
		return nil, err
	}
	if r, ok := reply.Payload.(*messages.CancelRefused); ok {
		return nil, arcp.NewError(arcp.CodeFailedPrecondition, r.Reason)
	}
	return reply, nil
}

// interruptJob is distinct from cancel: pauses the job (`blocked`),
// runtime emits human.input.request. Job is NOT terminated (RFC §10.5).
func interruptJob(ctx context.Context, c *Session, jid arcp.JobID, prompt string) error {
	return c.Send(ctx, &arcp.Envelope{
		Payload: &messages.Interrupt{
			Target: messages.CancelTargetJob, TargetID: string(jid),
			Prompt: prompt,
		}})
}

func awaitTerminal(ctx context.Context, c *Session, jid arcp.JobID) (arcp.Envelope, error) {
	for env := range c.Events(ctx) {
		if env.JobID != jid {
			continue
		}
		switch env.Payload.(type) {
		case *messages.JobCompleted, *messages.JobFailed, *messages.JobCancelled:
			return env, nil
		}
	}
	return arcp.Envelope{}, fmt.Errorf("event stream closed before terminal")
}

func scenarioCancel(ctx context.Context) error {
	c := openClient(ctx)
	defer c.Close(ctx)
	jid, err := startLongJob(ctx, c)
	if err != nil {
		return err
	}
	time.Sleep(2 * time.Second) // let the job actually start
	ack, err := cancelJob(ctx, c, jid, "user_aborted",
		int(cancelDeadline/time.Millisecond))
	if err != nil {
		return err
	}
	fmt.Printf("cancel ack: %s\n", ack.Type())
	terminal, err := awaitTerminal(ctx, c, jid)
	if err != nil {
		return err
	}
	fmt.Printf("terminal: %s\n", terminal.Type())
	return nil
}

func scenarioInterrupt(ctx context.Context) error {
	c := openClient(ctx)
	defer c.Close(ctx)
	jid, err := startLongJob(ctx, c)
	if err != nil {
		return err
	}
	time.Sleep(2 * time.Second)
	if err := interruptJob(ctx, c, jid,
		"Pause and ask before touching production tables."); err != nil {
		return err
	}
	// Runtime now emits human.input.request; answer via examples/human_input.
	for env := range c.Events(ctx) {
		if r, ok := env.Payload.(*messages.HumanInputRequest); ok && env.JobID == jid {
			fmt.Printf("awaiting human: %q\n", r.Prompt)
			return nil
		}
	}
	return nil
}

func main() {
	ctx := context.Background()
	which := "cancel"
	if len(os.Args) > 1 {
		which = os.Args[1]
	}
	var err error
	switch which {
	case "cancel":
		err = scenarioCancel(ctx)
	case "interrupt":
		err = scenarioInterrupt(ctx)
	default:
		log.Fatalf("unknown scenario: %s", which)
	}
	if err != nil {
		log.Fatal(err)
	}
}

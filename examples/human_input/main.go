// Fan human.input.request across channels; resolve on first response.
package main

import (
	"context"
	"encoding/json"
	"log"
	"time"

	arcp "github.com/agentruntimecontrolprotocol/go-sdk"
	"github.com/agentruntimecontrolprotocol/go-sdk/messages"
)

var destinations = []string{"ntfy:phone", "email:oncall", "slack:ops"}

type winner struct {
	dest  string
	value json.RawMessage
}

func fanOut(ctx context.Context, c *Session, request arcp.Envelope) {
	req := request.Payload.(*messages.HumanInputRequest)
	timeout := time.Until(req.ExpiresAt)
	if timeout < 0 {
		timeout = 0
	}

	winCh := make(chan winner, len(destinations))
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	for _, d := range destinations {
		d := d
		go func() {
			if v, err := registry[d](cctx, req.Prompt, req.ResponseSchema); err == nil {
				select {
				case winCh <- winner{dest: d, value: v}:
				case <-cctx.Done():
				}
			}
		}()
	}

	var w winner
	select {
	case w = <-winCh:
	case <-cctx.Done():
		// Deadline elapsed; translate timeout into the cancelled-input
		// shape (RFC §12.4).
		_ = c.Send(ctx, &arcp.Envelope{
			CorrelationID: request.ID,
			Payload: &messages.HumanInputCancelled{
				Code:   arcp.CodeDeadlineExceeded,
				Reason: "no channel responded before expires_at",
			}})
		return
	}

	_ = c.Send(ctx, &arcp.Envelope{
		CorrelationID: request.ID,
		Payload: &messages.HumanInputResponse{
			Value:       w.value,
			RespondedBy: w.dest,
			RespondedAt: time.Now().UTC(),
		}})

	// Tell the losing destinations the question is settled. Each
	// adapter would translate to "delete the push" / "edit the slack
	// message to '(answered)'".
	losers := []string{}
	for _, d := range destinations {
		if d != w.dest {
			losers = append(losers, d)
		}
	}
	if len(losers) > 0 {
		details := map[string]any{"channels": losers}
		_ = c.Send(ctx, &arcp.Envelope{
			CorrelationID: request.ID,
			Payload: &messages.HumanInputCancelled{
				Code: arcp.CodeOK, Reason: "answered elsewhere",
			},
			Extensions: jsonExtension("arcpx.humaninput.cancelled_channels.v1", details),
		})
	}
}

func main() {
	ctx := context.Background()
	c := openHITL(ctx) // transport, identity, auth elided
	defer c.Close(ctx)

	for env := range c.Events(ctx) {
		if _, ok := env.Payload.(*messages.HumanInputRequest); ok {
			go fanOut(ctx, c, env)
		}
	}
	log.Print("event stream closed")
}

// Primary emits reasoning; mirror peer subscribes, critiques back.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	arcp "github.com/agentruntimecontrolprotocol/go-sdk"
	"github.com/agentruntimecontrolprotocol/go-sdk/messages"
)

const (
	maxDepth    = 3
	tokenBudget = 8000
)

// Primary side -----------------------------------------------------------

type Critique struct {
	TargetThoughtSequence int    `json:"target_thought_sequence"`
	Severity              string `json:"severity"`
	Summary               string `json:"summary"`
	Suggestion            string `json:"suggestion"`
	ConsumedTokens        int    `json:"consumed_tokens"`
}

func runPrimary(ctx context.Context, c *Session,
	request string, inbound <-chan Critique,
) (string, error) {
	streamID := arcp.NewStreamID()
	if err := c.Send(ctx, &arcp.Envelope{
		StreamID: streamID,
		Payload:  &messages.StreamOpen{Kind: messages.StreamKindThought},
	}); err != nil {
		return "", err
	}

	var (
		last   *Critique
		answer string
	)
	for step := 0; step < maxDepth; step++ {
		answer = primaryStep(ctx, request, last)
		_ = c.Send(ctx, &arcp.Envelope{
			StreamID: streamID,
			Payload: &messages.StreamChunk{
				Sequence: step,
				Role:     "assistant_thought",
				Content:  answer,
			},
		})
		select {
		case crit := <-inbound:
			last = &crit
			if crit.Severity == "halt" {
				return answer, nil
			}
		case <-time.After(5 * time.Second):
			last = nil
		}
	}
	return answer, nil
}

// Mirror side: a peer runtime, NOT a pure observer — it both reads the
// thought stream AND delegates critique events back. ----------------------

func subscribeThoughts(ctx context.Context, mirror *Session,
	target arcp.SessionID,
) (arcp.SubscriptionID, error) {
	resp, err := mirror.Request(ctx, &arcp.Envelope{
		Payload: &messages.Subscribe{
			Filter: messages.SubscribeFilter{
				SessionID: []arcp.SessionID{target},
				Types:     []string{"stream.chunk"},
			}}})
	if err != nil {
		return "", err
	}
	return resp.Payload.(*messages.SubscribeAccepted).SubscriptionID, nil
}

func isThought(env arcp.Envelope) bool {
	chunk, ok := env.Payload.(*messages.StreamChunk)
	return ok && chunk.Role == "assistant_thought"
}

func runMirror(ctx context.Context, mirror *Session, target arcp.SessionID) {
	subID, err := subscribeThoughts(ctx, mirror, target)
	if err != nil {
		return
	}
	defer mirror.Send(ctx, &arcp.Envelope{
		Payload: &messages.Unsubscribe{SubscriptionID: subID},
	})

	spent := 0
	for env := range mirror.Events(ctx) {
		wrap, ok := env.Payload.(*messages.SubscribeEvent)
		if !ok {
			continue
		}
		var inner arcp.Envelope
		if err := json.Unmarshal(wrap.Event, &inner); err != nil {
			continue
		}
		if !isThought(inner) {
			continue
		}
		if spent >= tokenBudget {
			// Tear down cleanly: runtime stops paying for events
			// we'll never act on.
			return
		}
		chunk := inner.Payload.(*messages.StreamChunk)
		severity, summary, suggestion, consumed := critiqueThought(ctx, chunk.Content)
		spent += consumed
		_ = mirror.Send(ctx, &arcp.Envelope{
			Target: string(target),
			Payload: &messages.AgentDelegate{
				Target: "primary",
				Task:   "consume_critique",
				Context: map[string]any{
					"critique": Critique{
						TargetThoughtSequence: chunk.Sequence,
						Severity:              severity,
						Summary:               summary,
						Suggestion:            suggestion,
						ConsumedTokens:        consumed,
					},
				}}})
	}
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	primary := openPrimary(ctx)
	mirror := openMirror(ctx)
	defer primary.Close(ctx)
	defer mirror.Close(ctx)

	inbound := make(chan Critique, 4)
	go func() {
		for env := range primary.Events(ctx) {
			d, ok := env.Payload.(*messages.AgentDelegate)
			if !ok {
				continue
			}
			critRaw, _ := d.Context["critique"]
			b, _ := json.Marshal(critRaw)
			var c Critique
			if json.Unmarshal(b, &c) == nil {
				inbound <- c
			}
		}
	}()
	go runMirror(ctx, mirror, primary.SessionID())

	answer, err := runPrimary(ctx, primary,
		"Argue both sides: serializable vs snapshot iso?", inbound)
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Println(answer)
}

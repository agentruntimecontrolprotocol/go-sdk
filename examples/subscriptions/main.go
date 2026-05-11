// Boot three Observer clients on a single producing session.
//
// Each runs in its own goroutine, with its own filter and its own
// sink. The producing session (elided) never knows they exist —
// subscriptions are entirely runtime-driven.
package main

import (
	"context"
	"encoding/json"
	"sync"

	arcp "github.com/agentruntimecontrolprotocol/go-sdk"
	"github.com/agentruntimecontrolprotocol/go-sdk/messages"
)

var (
	stdoutTypes = []string{
		"log", "job.started", "job.progress",
		"job.completed", "job.failed", "tool.error",
	}
	otlpTypes = []string{"metric", "trace.span"}
)

// Sink is a per-observer event handler. Real implementations:
//   - StdoutSink: log/slog summarizer.
//   - SQLiteSink: arcp/store/eventlog schema.
//   - OTLPSink: forwards metric + trace.span to OTLP.
type Sink interface {
	Handle(ctx context.Context, env arcp.Envelope) error
}

func subscribe(ctx context.Context, c *Session,
	sessionID arcp.SessionID, types []string,
) (arcp.SubscriptionID, error) {
	filter := messages.SubscribeFilter{
		SessionID: []arcp.SessionID{sessionID},
		Types:     types,
	}
	resp, err := c.Request(ctx, &arcp.Envelope{
		Payload: &messages.Subscribe{Filter: filter},
	})
	if err != nil {
		return "", err
	}
	return resp.Payload.(*messages.SubscribeAccepted).SubscriptionID, nil
}

// unwrapEvent decodes the inner envelope wrapped in subscribe.event,
// or returns nil if env is not a subscription delivery.
func unwrapEvent(env arcp.Envelope) *arcp.Envelope {
	wrap, ok := env.Payload.(*messages.SubscribeEvent)
	if !ok {
		return nil
	}
	var inner arcp.Envelope
	if err := json.Unmarshal(wrap.Event, &inner); err != nil {
		return nil
	}
	return &inner
}

func attach(ctx context.Context, target arcp.SessionID,
	types []string, sink Sink,
) error {
	c := openObserver(ctx) // transport, identity, auth elided
	defer c.Close(ctx)

	subID, err := subscribe(ctx, c, target, types)
	if err != nil {
		return err
	}
	defer c.Send(ctx, &arcp.Envelope{
		Payload: &messages.Unsubscribe{SubscriptionID: subID},
	})

	for env := range c.Events(ctx) {
		if inner := unwrapEvent(env); inner != nil {
			if err := sink.Handle(ctx, *inner); err != nil {
				return err
			}
		}
	}
	return nil
}

func main() {
	ctx := context.Background()
	target := arcp.SessionID("sess_target_elided")

	stdout, sqlite, otlp := newSinks()
	defer sqlite.Close()

	var wg sync.WaitGroup
	for _, spec := range []struct {
		types []string
		sink  Sink
	}{
		{stdoutTypes, stdout},
		{nil, sqlite}, // nil filter → everything
		{otlpTypes, otlp},
	} {
		wg.Add(1)
		go func(types []string, sink Sink) {
			defer wg.Done()
			if err := attach(ctx, target, types, sink); err != nil {
				_ = err // real version: structured log + retry
			}
		}(spec.types, spec.sink)
	}
	wg.Wait()
}

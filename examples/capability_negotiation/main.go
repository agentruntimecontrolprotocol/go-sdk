// Capability-driven peer routing with ordered fallback + cost rollup.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sync"

	arcp "github.com/agentruntimecontrolprotocol/go-sdk"
	"github.com/agentruntimecontrolprotocol/go-sdk/messages"
)

var peers = []string{
	"anthropic-haiku", "anthropic-sonnet", "openai-4o", "groq-llama",
}

var fallbackChains = map[string][]string{
	"cheap_fast": {"groq-llama", "anthropic-haiku", "openai-4o"},
	"balanced":   {"anthropic-sonnet", "openai-4o", "anthropic-haiku"},
	"deep":       {"anthropic-sonnet"},
}

const (
	costCeilingUSDPerMtok = 8.0
	latencyCeilingMs      = 800
)

func retryable(c arcp.ErrorCode) bool {
	switch c {
	case arcp.CodeResourceExhausted, arcp.CodeUnavailable,
		arcp.CodeDeadlineExceeded, arcp.CodeAborted:
		return true
	}
	return false
}

type Profile struct {
	CostPerMtok  float64
	P50LatencyMs int
	ModelClass   string
}

// profileFrom reads the per-peer marketplace fields off the
// negotiated capability extensions. NOTE: §21 covers extension
// *messages*; this is the load-bearing convention for extension
// *capability values*.
func profileFrom(_ messages.Capabilities, exts map[string]json.RawMessage) Profile {
	var p Profile
	_ = json.Unmarshal(exts["arcpx.market.cost_per_mtok.v1"], &p.CostPerMtok)
	_ = json.Unmarshal(exts["arcpx.market.p50_latency_ms.v1"], &p.P50LatencyMs)
	_ = json.Unmarshal(exts["arcpx.market.model_class.v1"], &p.ModelClass)
	return p
}

func candidateChain(profiles map[string]Profile, requestClass string) []string {
	out := []string{}
	for _, name := range fallbackChains[requestClass] {
		p, ok := profiles[name]
		if !ok || p.CostPerMtok > costCeilingUSDPerMtok || p.P50LatencyMs > latencyCeilingMs {
			continue
		}
		out = append(out, name)
	}
	return out
}

func invokeWithFallback(ctx context.Context,
	clients map[string]*Session, chain []string,
	tool string, arguments map[string]any, traceID arcp.TraceID,
) (*arcp.Envelope, error) {
	var last error
	for _, name := range chain {
		nameBytes, _ := json.Marshal(name)
		reply, err := clients[name].Request(ctx, &arcp.Envelope{
			TraceID:    traceID,
			Extensions: map[string]json.RawMessage{"arcpx.market.peer.v1": nameBytes},
			Payload:    &messages.ToolInvoke{Tool: tool, Arguments: arguments},
		})
		var ae *arcp.Error
		if err != nil {
			last = err
			if errors.As(err, &ae) && retryable(ae.Code) {
				continue
			}
			return nil, err
		}
		if te, ok := reply.Payload.(*messages.ToolError); ok {
			last = arcp.NewError(te.Code, te.Message)
			if retryable(te.Code) {
				continue
			}
			return nil, last
		}
		return reply, nil
	}
	if last == nil {
		last = arcp.NewError(arcp.CodeUnavailable, "no peers available")
	}
	return nil, last
}

type Usage struct {
	TokensIn, TokensOut int
	CostUSD             float64
	ByPeer              map[string]float64
}

type Totals struct {
	mu sync.Mutex
	m  map[string]*Usage
}

func (t *Totals) consume(env arcp.Envelope) {
	m, ok := env.Payload.(*messages.Metric)
	if !ok {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	tenant, _ := m.Dims["tenant"].(string)
	u := t.m[tenant]
	if u == nil {
		u = &Usage{ByPeer: map[string]float64{}}
		t.m[tenant] = u
	}
	switch m.Name {
	case "tokens.used":
		switch m.Dims["kind"] {
		case "input":
			u.TokensIn += int(m.Value)
		case "output":
			u.TokensOut += int(m.Value)
		}
	case "cost.usd":
		u.CostUSD += m.Value
		peer, _ := m.Dims["peer"].(string)
		u.ByPeer[peer] += m.Value
	}
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	clients := map[string]*Session{}
	profiles := map[string]Profile{}
	for _, name := range peers {
		c, accepted, exts := openPeer(ctx, name)
		clients[name] = c
		profiles[name] = profileFrom(accepted.Capabilities, exts)
	}
	defer func() {
		for _, c := range clients {
			c.Close(ctx)
		}
	}()

	totals := &Totals{m: map[string]*Usage{}}
	for _, c := range clients {
		c := c
		go func() {
			for env := range c.Events(ctx) {
				totals.consume(env)
			}
		}()
	}

	chain := candidateChain(profiles, "balanced")
	reply, err := invokeWithFallback(ctx, clients, chain,
		"chat.completion",
		map[string]any{"prompt": "Hello", "tenant": "acme-corp"},
		arcp.NewTraceID())
	if err != nil {
		log.Fatal(err)
	}
	var chosen string
	_ = json.Unmarshal(reply.Extensions["arcpx.market.peer.v1"], &chosen)
	fmt.Println("chosen=", chosen)
	fmt.Printf("usage=%+v\n", totals.m)
}

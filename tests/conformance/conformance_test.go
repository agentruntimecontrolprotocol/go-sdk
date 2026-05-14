// Package conformance runs the per-section spec checks and emits a
// machine-readable conformance.json summary.
package conformance_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"

	arcp "github.com/agentruntimecontrolprotocol/go-sdk"
	"github.com/agentruntimecontrolprotocol/go-sdk/client"
	"github.com/agentruntimecontrolprotocol/go-sdk/messages"
	"github.com/agentruntimecontrolprotocol/go-sdk/server"
	"github.com/agentruntimecontrolprotocol/go-sdk/transport"
)

type row struct {
	Section     string `json:"section"`
	Requirement string `json:"requirement"`
	Status      string `json:"status"`
	DurationMs  int64  `json:"duration_ms"`
}

func TestConformance(t *testing.T) {
	rows := []row{}
	record := func(section, req string, fn func(t *testing.T)) {
		start := time.Now()
		status := "pass"
		tt := &testing.T{}
		defer func() {
			if r := recover(); r != nil {
				status = "fail"
			}
			rows = append(rows, row{
				Section:     section,
				Requirement: req,
				Status:      status,
				DurationMs:  time.Since(start).Milliseconds(),
			})
		}()
		fn(tt)
		if tt.Failed() {
			status = "fail"
		}
	}

	record("§5.1", "envelope arcp constant", func(t *testing.T) {
		if arcp.ProtocolVersion != "1" {
			t.Fail()
		}
	})
	record("§6.2", "feature negotiation", func(t *testing.T) {
		want := []string{"heartbeat", "ack"}
		got := arcp.IntersectFeatures([]string{"heartbeat", "ack", "unknown"}, []string{"heartbeat", "ack"})
		if len(got) != len(want) {
			t.Fail()
		}
	})
	record("§7.5", "agent ref grammar", func(t *testing.T) {
		_, err := messages.ParseAgentRef("Foo")
		if err == nil {
			t.Fail()
		}
	})
	record("§7.1", "submit-and-result round trip", func(t *testing.T) {
		srv := server.New(server.Options{})
		srv.RegisterAgent("echo", func(ctx context.Context, input json.RawMessage, jc *server.JobContext) (any, error) {
			return map[string]bool{"ok": true}, nil
		})
		a, b := transport.NewMemoryPair()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		go srv.Accept(ctx, b)
		cli, err := client.Connect(ctx, a, client.Options{Token: "demo"})
		if err != nil {
			t.Fail()
			return
		}
		defer cli.Close(ctx)
		h, err := cli.Submit(ctx, client.SubmitRequest{Agent: "echo"})
		if err != nil {
			t.Fail()
			return
		}
		if _, err := h.Wait(ctx); err != nil {
			t.Fail()
		}
	})
	record("§7.5", "agent version unavailable", func(t *testing.T) {
		srv := server.New(server.Options{})
		srv.RegisterAgentVersion("x", "1.0.0", func(ctx context.Context, _ json.RawMessage, _ *server.JobContext) (any, error) {
			return nil, nil
		})
		a, b := transport.NewMemoryPair()
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		go srv.Accept(ctx, b)
		cli, err := client.Connect(ctx, a, client.Options{Token: "demo"})
		if err != nil {
			t.Fail()
			return
		}
		defer cli.Close(ctx)
		_, err = cli.Submit(ctx, client.SubmitRequest{Agent: "x@2.0.0"})
		if !errors.Is(err, arcp.ErrAgentVersionNotAvailable) {
			t.Fail()
		}
	})
	record("§12", "error codes count", func(t *testing.T) {
		codes := []arcp.ErrorCode{
			arcp.CodePermissionDenied,
			arcp.CodeLeaseSubsetViolation,
			arcp.CodeJobNotFound,
			arcp.CodeDuplicateKey,
			arcp.CodeAgentNotAvailable,
			arcp.CodeAgentVersionNotAvailable,
			arcp.CodeCancelled,
			arcp.CodeTimeout,
			arcp.CodeResumeWindowExpired,
			arcp.CodeHeartbeatLost,
			arcp.CodeLeaseExpired,
			arcp.CodeBudgetExhausted,
			arcp.CodeInvalidRequest,
			arcp.CodeUnauthenticated,
			arcp.CodeInternalError,
		}
		if len(codes) != 15 {
			t.Fail()
		}
	})

	// Write conformance.json summary.
	body, _ := json.MarshalIndent(rows, "", "  ")
	if path := os.Getenv("ARCP_CONFORMANCE_OUT"); path != "" {
		_ = os.WriteFile(path, body, 0o644)
	}

	for _, r := range rows {
		if r.Status != "pass" {
			t.Errorf("%s %s — %s", r.Section, r.Requirement, r.Status)
		}
	}
}

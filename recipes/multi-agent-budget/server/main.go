package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"

	arcp "github.com/agentruntimecontrolprotocol/go-sdk"
	"github.com/agentruntimecontrolprotocol/go-sdk/middleware/nethttp"
	"github.com/agentruntimecontrolprotocol/go-sdk/server"
)

func main() {
	srv := server.New(server.Options{Name: "multi-agent-budget"})
	srv.RegisterAgent("planner", func(ctx context.Context, input json.RawMessage, jc *server.JobContext) (any, error) {
		if err := jc.ValidateLeaseOp(arcp.CapToolCall, "plan"); err != nil {
			return nil, err
		}
		jc.Metric("cost.plan", 0.40, "USD", nil)
		jc.ArtifactRef("memory://plan/1", "application/json", 0, "")
		return map[string]string{"status": "planned"}, nil
	})
	mux := http.NewServeMux()
	mux.Handle("/arcp", nethttp.NewHandler(srv, nethttp.Options{}))
	log.Fatal(http.ListenAndServe(":7842", mux))
}

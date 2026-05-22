package main

import (
	"context"
	"encoding/json"
	"log"
	"os"

	arcp "github.com/agentruntimecontrolprotocol/go-sdk"
	"github.com/agentruntimecontrolprotocol/go-sdk/server"
	"github.com/agentruntimecontrolprotocol/go-sdk/transport"
)

func main() {
	srv := server.New(server.Options{Name: "mcp-skill"})
	srv.RegisterAgent("skill.research", func(ctx context.Context, input json.RawMessage, jc *server.JobContext) (any, error) {
		if err := jc.ValidateLeaseOp(arcp.CapToolCall, "mcp.search"); err != nil {
			return nil, err
		}
		call := jc.ToolCall("mcp.search", map[string]string{"query": string(input)})
		jc.ToolResult(call, map[string]int{"hits": 3})
		return map[string]string{"summary": "research complete"}, nil
	})
	t := transport.NewStdioTransport(os.Stdin, os.Stdout)
	log.Fatal(srv.Accept(context.Background(), t))
}

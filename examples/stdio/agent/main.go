package main

import (
	"context"
	"encoding/json"
	"os"

	"github.com/agentruntimecontrolprotocol/go-sdk/server"
	"github.com/agentruntimecontrolprotocol/go-sdk/transport"
)

func main() {
	srv := server.New(server.Options{Name: "stdio-agent"})
	srv.RegisterAgent("echo", func(ctx context.Context, input json.RawMessage, jc *server.JobContext) (any, error) {
		return map[string]json.RawMessage{"echo": input}, nil
	})
	t := transport.NewStdioTransport(os.Stdin, os.Stdout)
	_ = srv.Accept(context.Background(), t)
}

package main

import (
	"context"
	"errors"
	"log"
	"time"

	arcp "github.com/agentruntimecontrolprotocol/go-sdk"
	"github.com/agentruntimecontrolprotocol/go-sdk/client"
	"github.com/agentruntimecontrolprotocol/go-sdk/examples/internal/demo"
	"github.com/agentruntimecontrolprotocol/go-sdk/transport"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	t, err := transport.DialWebSocket(ctx, demo.Addr(7824), transport.WebSocketOptions{})
	demo.Must(err)
	cli, err := client.Connect(ctx, t, client.Options{})
	demo.Must(err)
	defer cli.Close(ctx)

	// 1) bare name → default version
	h1, err := cli.Submit(ctx, client.SubmitRequest{Agent: "code-refactor"})
	demo.Must(err)
	res1, err := h1.Wait(ctx)
	demo.Must(err)
	log.Println("bare → ", string(res1.Output))

	// 2) pinned version
	h2, err := cli.Submit(ctx, client.SubmitRequest{Agent: "code-refactor@1.0.0"})
	demo.Must(err)
	res2, err := h2.Wait(ctx)
	demo.Must(err)
	log.Println("@1.0.0 → ", string(res2.Output))

	// 3) unavailable
	_, err = cli.Submit(ctx, client.SubmitRequest{Agent: "code-refactor@3.0.0"})
	if err == nil || !errors.Is(err, arcp.ErrAgentVersionNotAvailable) {
		log.Fatalf("expected AGENT_VERSION_NOT_AVAILABLE, got %v", err)
	}
	log.Println("@3.0.0 → AGENT_VERSION_NOT_AVAILABLE (as expected)")
}

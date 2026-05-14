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
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	t, err := transport.DialWebSocket(ctx, demo.Addr(7814), transport.WebSocketOptions{})
	demo.Must(err)
	cli, err := client.Connect(ctx, t, client.Options{})
	demo.Must(err)
	defer cli.Close(ctx)

	h1, err := cli.Submit(ctx, client.SubmitRequest{Agent: "echo", IdempotencyKey: "demo-1"})
	demo.Must(err)
	_, err = cli.Submit(ctx, client.SubmitRequest{Agent: "echo", IdempotencyKey: "demo-1"})
	if !errors.Is(err, arcp.ErrDuplicateKey) {
		log.Fatalf("expected DUPLICATE_KEY, got %v", err)
	}
	log.Println("duplicate key rejected; first job:", h1.ID())
}

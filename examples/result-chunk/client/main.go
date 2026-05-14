package main

import (
	"context"
	"log"
	"time"

	"github.com/agentruntimecontrolprotocol/go-sdk/client"
	"github.com/agentruntimecontrolprotocol/go-sdk/examples/internal/demo"
	"github.com/agentruntimecontrolprotocol/go-sdk/transport"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	t, err := transport.DialWebSocket(ctx, demo.Addr(7828), transport.WebSocketOptions{})
	demo.Must(err)
	cli, err := client.Connect(ctx, t, client.Options{})
	demo.Must(err)
	defer cli.Close(ctx)
	h, err := cli.Submit(ctx, client.SubmitRequest{Agent: "report"})
	demo.Must(err)
	bytes, err := h.CollectChunks(ctx)
	demo.Must(err)
	log.Printf("assembled %d bytes", len(bytes))
}

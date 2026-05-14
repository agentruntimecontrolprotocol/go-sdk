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
	t, err := transport.DialWebSocket(ctx, demo.Addr(7816), transport.WebSocketOptions{})
	demo.Must(err)
	cli, err := client.Connect(ctx, t, client.Options{})
	demo.Must(err)
	defer cli.Close(ctx)
	h, err := cli.Submit(ctx, client.SubmitRequest{Agent: "long"})
	demo.Must(err)
	time.Sleep(500 * time.Millisecond)
	demo.Must(h.Cancel(ctx, "user requested"))
	_, err = h.Wait(ctx)
	if !errors.Is(err, arcp.ErrCancelled) {
		log.Fatalf("expected CANCELLED, got %v", err)
	}
	log.Println("cancelled as expected")
}

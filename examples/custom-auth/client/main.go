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
	// Bad token first.
	t1, err := transport.DialWebSocket(ctx, demo.Addr(7819), transport.WebSocketOptions{})
	demo.Must(err)
	_, err = client.Connect(ctx, t1, client.Options{Token: "wrong"})
	if !errors.Is(err, arcp.ErrUnauthenticated) {
		log.Fatalf("expected UNAUTHENTICATED, got %v", err)
	}
	log.Println("bad token rejected")

	// Good token.
	t2, err := transport.DialWebSocket(ctx, demo.Addr(7819), transport.WebSocketOptions{})
	demo.Must(err)
	cli, err := client.Connect(ctx, t2, client.Options{Token: "sk-demo"})
	demo.Must(err)
	defer cli.Close(ctx)
	h, err := cli.Submit(ctx, client.SubmitRequest{Agent: "ping"})
	demo.Must(err)
	_, err = h.Wait(ctx)
	demo.Must(err)
	log.Println("ok")
}

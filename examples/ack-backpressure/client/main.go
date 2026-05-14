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
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	t, err := transport.DialWebSocket(ctx, demo.Addr(7821), transport.WebSocketOptions{})
	demo.Must(err)
	cli, err := client.Connect(ctx, t, client.Options{AutoAckWindow: 32})
	demo.Must(err)
	defer cli.Close(ctx)
	h, err := cli.Submit(ctx, client.SubmitRequest{Agent: "chatty"})
	demo.Must(err)
	count := 0
	go func() {
		for range h.Events() {
			count++
		}
	}()
	_, err = h.Wait(ctx)
	demo.Must(err)
	log.Printf("received %d events", count)
}

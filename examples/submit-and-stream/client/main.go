package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/agentruntimecontrolprotocol/go-sdk/client"
	"github.com/agentruntimecontrolprotocol/go-sdk/examples/internal/demo"
	"github.com/agentruntimecontrolprotocol/go-sdk/transport"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	t, err := transport.DialWebSocket(ctx, demo.Addr(7811), transport.WebSocketOptions{})
	demo.Must(err)
	cli, err := client.Connect(ctx, t, client.Options{Token: "demo"})
	demo.Must(err)
	defer cli.Close(ctx)

	h, err := cli.Submit(ctx, client.SubmitRequest{
		Agent: "counter",
		Input: map[string]int{"n": 5},
	})
	demo.Must(err)
	go func() {
		for ev := range h.Events() {
			fmt.Println("event:", ev.Kind, string(ev.Body))
		}
	}()
	res, err := h.Wait(ctx)
	demo.Must(err)
	log.Println("result:", string(res.Output))
}

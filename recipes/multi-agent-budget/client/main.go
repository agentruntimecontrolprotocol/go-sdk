package main

import (
	"context"
	"log"
	"time"

	arcp "github.com/agentruntimecontrolprotocol/go-sdk"
	"github.com/agentruntimecontrolprotocol/go-sdk/client"
	"github.com/agentruntimecontrolprotocol/go-sdk/transport"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	t, err := transport.DialWebSocket(ctx, "ws://127.0.0.1:7842/arcp", transport.WebSocketOptions{})
	must(err)
	cli, err := client.Connect(ctx, t, client.Options{})
	must(err)
	defer cli.Close(ctx)
	h, err := cli.Submit(ctx, client.SubmitRequest{
		Agent: "planner",
		LeaseRequest: arcp.Lease{
			arcp.CapToolCall:   {"plan"},
			arcp.CapCostBudget: {"USD:1.00"},
		},
	})
	must(err)
	res, err := h.Wait(ctx)
	must(err)
	log.Println(string(res.Output))
}

func must(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

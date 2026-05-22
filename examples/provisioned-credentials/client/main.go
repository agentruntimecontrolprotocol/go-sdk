package main

import (
	"context"
	"fmt"
	"log"
	"time"

	arcp "github.com/agentruntimecontrolprotocol/go-sdk"
	"github.com/agentruntimecontrolprotocol/go-sdk/client"
	"github.com/agentruntimecontrolprotocol/go-sdk/examples/internal/demo"
	"github.com/agentruntimecontrolprotocol/go-sdk/messages"
	"github.com/agentruntimecontrolprotocol/go-sdk/transport"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	t, err := transport.DialWebSocket(ctx, demo.Addr(7836), transport.WebSocketOptions{})
	demo.Must(err)
	cli, err := client.Connect(ctx, t, client.Options{})
	demo.Must(err)
	defer cli.Close(ctx)
	exp := time.Now().Add(5 * time.Minute).UTC()
	h, err := cli.Submit(ctx, client.SubmitRequest{
		Agent: "chat",
		LeaseRequest: arcp.Lease{
			arcp.CapModelUse:   {"tier-fast/*"},
			arcp.CapCostBudget: {"USD:1.00"},
		},
		LeaseConstraints: &messages.LeaseConstraints{ExpiresAt: &exp},
	})
	demo.Must(err)
	for _, cred := range h.Accepted().Credentials {
		fmt.Printf("credential id=%s scheme=%s endpoint=%s\n", cred.ID, cred.Scheme, cred.Endpoint)
	}
	res, err := h.Wait(ctx)
	demo.Must(err)
	log.Println("result:", string(res.Output))
}

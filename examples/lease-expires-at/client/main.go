package main

import (
	"context"
	"errors"
	"log"
	"time"

	arcp "github.com/agentruntimecontrolprotocol/go-sdk"
	"github.com/agentruntimecontrolprotocol/go-sdk/client"
	"github.com/agentruntimecontrolprotocol/go-sdk/examples/internal/demo"
	"github.com/agentruntimecontrolprotocol/go-sdk/messages"
	"github.com/agentruntimecontrolprotocol/go-sdk/transport"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	t, err := transport.DialWebSocket(ctx, demo.Addr(7825), transport.WebSocketOptions{})
	demo.Must(err)
	cli, err := client.Connect(ctx, t, client.Options{})
	demo.Must(err)
	defer cli.Close(ctx)
	exp := time.Now().Add(2 * time.Second)
	h, err := cli.Submit(ctx, client.SubmitRequest{
		Agent:        "slow-job",
		LeaseRequest: arcp.Lease{arcp.CapFSRead: {"/data/**"}},
		LeaseConstraints: &messages.LeaseConstraints{
			ExpiresAt: &exp,
		},
	})
	demo.Must(err)
	_, err = h.Wait(ctx)
	if err == nil {
		log.Fatal("expected LEASE_EXPIRED, got nil")
	}
	if !errors.Is(err, arcp.ErrLeaseExpired) {
		log.Fatalf("expected LEASE_EXPIRED, got %v", err)
	}
	log.Println("lease expired as expected")
}

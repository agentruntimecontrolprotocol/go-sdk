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

	// Session A: submitter.
	tA, err := transport.DialWebSocket(ctx, demo.Addr(7823), transport.WebSocketOptions{})
	demo.Must(err)
	cliA, err := client.Connect(ctx, tA, client.Options{Token: "alice"})
	demo.Must(err)
	defer cliA.Close(ctx)
	h, err := cliA.Submit(ctx, client.SubmitRequest{Agent: "ticker"})
	demo.Must(err)

	// Session B: observer (same principal name → same logical principal
	// in this demo's anonymous Verifier flow).
	tB, err := transport.DialWebSocket(ctx, demo.Addr(7823), transport.WebSocketOptions{})
	demo.Must(err)
	cliB, err := client.Connect(ctx, tB, client.Options{Token: "alice", ClientName: "arcp-go-client"})
	demo.Must(err)
	defer cliB.Close(ctx)
	sub, err := cliB.Subscribe(ctx, h.ID(), client.SubscribeOptions{History: true})
	demo.Must(err)
	go func() {
		for ev := range sub.Events() {
			log.Println("observer:", ev.Kind)
		}
	}()
	_, err = h.Wait(ctx)
	demo.Must(err)
	_ = sub.Close(ctx)
}

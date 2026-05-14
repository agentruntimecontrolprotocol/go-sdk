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
	t, err := transport.DialWebSocket(ctx, demo.Addr(7820), transport.WebSocketOptions{})
	demo.Must(err)
	cli, err := client.Connect(ctx, t, client.Options{})
	demo.Must(err)
	defer cli.Close(ctx)
	log.Printf("welcome heartbeat_interval_sec=%d", cli.Welcome().HeartbeatIntervalSec)
	h, err := cli.Submit(ctx, client.SubmitRequest{Agent: "idle"})
	demo.Must(err)
	_, err = h.Wait(ctx)
	demo.Must(err)
	log.Println("done")
}

package main

import (
	"context"
	"log"
	"time"

	arcp "github.com/agentruntimecontrolprotocol/go-sdk"
	"github.com/agentruntimecontrolprotocol/go-sdk/client"
	"github.com/agentruntimecontrolprotocol/go-sdk/examples/internal/demo"
	arcpotel "github.com/agentruntimecontrolprotocol/go-sdk/middleware/otel"
	"github.com/agentruntimecontrolprotocol/go-sdk/transport"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	t, err := transport.DialWebSocket(ctx, demo.Addr(7829), transport.WebSocketOptions{})
	demo.Must(err)
	wrapped := arcpotel.WrapTransport(t, arcpotel.Options{})
	cli, err := client.Connect(ctx, wrapped, client.Options{})
	demo.Must(err)
	defer cli.Close(ctx)
	h, err := cli.Submit(ctx, client.SubmitRequest{
		Agent:   "echo",
		TraceID: arcp.NewTraceID(),
	})
	demo.Must(err)
	res, err := h.Wait(ctx)
	demo.Must(err)
	log.Println("result:", string(res.Output))
}

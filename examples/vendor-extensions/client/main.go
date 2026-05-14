package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/agentruntimecontrolprotocol/go-sdk/client"
	"github.com/agentruntimecontrolprotocol/go-sdk/examples/internal/demo"
	"github.com/agentruntimecontrolprotocol/go-sdk/messages"
	"github.com/agentruntimecontrolprotocol/go-sdk/transport"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	t, err := transport.DialWebSocket(ctx, demo.Addr(7818), transport.WebSocketOptions{})
	demo.Must(err)
	cli, err := client.Connect(ctx, t, client.Options{})
	demo.Must(err)
	defer cli.Close(ctx)
	h, err := cli.Submit(ctx, client.SubmitRequest{Agent: "vendor"})
	demo.Must(err)
	go func() {
		for ev := range h.Events() {
			if ev.Kind == messages.KindMetric {
				var m messages.MetricBody
				_ = json.Unmarshal(ev.Body, &m)
				fmt.Printf("metric %s=%v %s dims=%v\n", m.Name, m.Value, m.Unit, m.Dimensions)
			}
		}
	}()
	_, err = h.Wait(ctx)
	demo.Must(err)
}

package main

import (
	"context"
	"encoding/json"
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
	t, err := transport.DialWebSocket(ctx, demo.Addr(7826), transport.WebSocketOptions{})
	demo.Must(err)
	cli, err := client.Connect(ctx, t, client.Options{})
	demo.Must(err)
	defer cli.Close(ctx)
	h, err := cli.Submit(ctx, client.SubmitRequest{
		Agent: "researcher",
		LeaseRequest: arcp.Lease{
			arcp.CapToolCall:   {"search.*"},
			arcp.CapCostBudget: {"USD:1.00"},
		},
	})
	demo.Must(err)
	go func() {
		for ev := range h.Events() {
			if ev.Kind == messages.KindMetric {
				var m messages.MetricBody
				_ = json.Unmarshal(ev.Body, &m)
				fmt.Printf("metric %s=%.2f %s\n", m.Name, m.Value, m.Unit)
			}
		}
	}()
	res, err := h.Wait(ctx)
	demo.Must(err)
	log.Println("result:", string(res.Output))
}

package main

import (
	"context"
	"encoding/json"
	"fmt"
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
	t, err := transport.DialWebSocket(ctx, demo.Addr(7815), transport.WebSocketOptions{})
	demo.Must(err)
	cli, err := client.Connect(ctx, t, client.Options{})
	demo.Must(err)
	defer cli.Close(ctx)
	h, err := cli.Submit(ctx, client.SubmitRequest{
		Agent:        "rogue",
		LeaseRequest: arcp.Lease{arcp.CapFSRead: {"/safe/**"}},
	})
	demo.Must(err)
	go func() {
		for ev := range h.Events() {
			if ev.Kind == messages.KindToolResult {
				var body messages.ToolResultBody
				_ = json.Unmarshal(ev.Body, &body)
				if body.Error != nil {
					fmt.Printf("tool_result error: %s — %s\n", body.Error.Code, body.Error.Message)
				}
			}
		}
	}()
	_, err = h.Wait(ctx)
	demo.Must(err)
}

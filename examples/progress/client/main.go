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
	t, err := transport.DialWebSocket(ctx, demo.Addr(7827), transport.WebSocketOptions{})
	demo.Must(err)
	cli, err := client.Connect(ctx, t, client.Options{})
	demo.Must(err)
	defer cli.Close(ctx)
	h, err := cli.Submit(ctx, client.SubmitRequest{Agent: "indexer"})
	demo.Must(err)
	go func() {
		for ev := range h.Events() {
			if ev.Kind != messages.KindProgress {
				continue
			}
			var p messages.ProgressBody
			_ = json.Unmarshal(ev.Body, &p)
			fmt.Printf("\r[%-20s] %d/%d %s", bar(p.Current, p.Total), p.Current, p.Total, p.Units)
		}
		fmt.Println()
	}()
	_, err = h.Wait(ctx)
	demo.Must(err)
}

func bar(cur, total uint64) string {
	if total == 0 {
		return ""
	}
	n := int(20 * cur / total)
	if n > 20 {
		n = 20
	}
	out := make([]byte, n)
	for i := range out {
		out[i] = '#'
	}
	return string(out)
}

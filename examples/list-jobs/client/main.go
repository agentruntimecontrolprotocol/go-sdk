package main

import (
	"context"
	"log"
	"time"

	"github.com/agentruntimecontrolprotocol/go-sdk/client"
	"github.com/agentruntimecontrolprotocol/go-sdk/examples/internal/demo"
	"github.com/agentruntimecontrolprotocol/go-sdk/messages"
	"github.com/agentruntimecontrolprotocol/go-sdk/transport"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	t, err := transport.DialWebSocket(ctx, demo.Addr(7822), transport.WebSocketOptions{})
	demo.Must(err)
	cli, err := client.Connect(ctx, t, client.Options{})
	demo.Must(err)
	defer cli.Close(ctx)
	for i := 0; i < 5; i++ {
		_, err := cli.Submit(ctx, client.SubmitRequest{Agent: "idle"})
		demo.Must(err)
	}
	resp, err := cli.ListJobs(ctx, client.ListJobsRequest{
		Filter: messages.ListJobsFilter{Status: []string{messages.StatusRunning, messages.StatusPending}},
		Limit:  2,
	})
	demo.Must(err)
	log.Printf("page 1: %d jobs, next=%q", len(resp.Jobs), resp.NextCursor)
	for resp.NextCursor != "" {
		resp, err = cli.ListJobs(ctx, client.ListJobsRequest{
			Filter: messages.ListJobsFilter{Status: []string{messages.StatusRunning, messages.StatusPending}},
			Limit:  2,
			Cursor: resp.NextCursor,
		})
		demo.Must(err)
		log.Printf("page: %d jobs, next=%q", len(resp.Jobs), resp.NextCursor)
	}
}

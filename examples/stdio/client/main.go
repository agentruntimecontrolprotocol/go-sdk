package main

import (
	"context"
	"log"
	"os/exec"
	"time"

	"github.com/agentruntimecontrolprotocol/go-sdk/client"
	"github.com/agentruntimecontrolprotocol/go-sdk/transport"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "run", "./examples/stdio/agent")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		log.Fatal(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		log.Fatal(err)
	}
	defer cmd.Process.Kill()
	t := transport.NewStdioTransport(stdout, stdin)
	cli, err := client.Connect(ctx, t, client.Options{})
	if err != nil {
		log.Fatal(err)
	}
	defer cli.Close(ctx)
	h, err := cli.Submit(ctx, client.SubmitRequest{Agent: "echo", Input: map[string]string{"hi": "stdio"}})
	if err != nil {
		log.Fatal(err)
	}
	res, err := h.Wait(ctx)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("result:", string(res.Output))
}

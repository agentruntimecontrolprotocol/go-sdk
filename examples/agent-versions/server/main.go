package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/agentruntimecontrolprotocol/go-sdk/examples/internal/demo"
	"github.com/agentruntimecontrolprotocol/go-sdk/middleware/nethttp"
	"github.com/agentruntimecontrolprotocol/go-sdk/server"
)

func main() {
	srv := server.New(server.Options{Name: "agent-versions"})
	srv.RegisterAgentVersion("code-refactor", "1.0.0", func(ctx context.Context, input json.RawMessage, jc *server.JobContext) (any, error) {
		return map[string]string{"version": "1.0.0"}, nil
	})
	srv.RegisterAgentVersion("code-refactor", "2.0.0", func(ctx context.Context, input json.RawMessage, jc *server.JobContext) (any, error) {
		return map[string]string{"version": "2.0.0"}, nil
	})
	demo.Must(srv.SetDefaultAgentVersion("code-refactor", "2.0.0"))
	mux := http.NewServeMux()
	mux.Handle("/arcp", nethttp.NewHandler(srv, nethttp.Options{}))
	httpSrv := &http.Server{Addr: demo.Listen(7824), Handler: mux}
	go func() {
		log.Println("listening on", httpSrv.Addr)
		_ = httpSrv.ListenAndServe()
	}()
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	_ = httpSrv.Shutdown(context.Background())
}

package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/agentruntimecontrolprotocol/go-sdk/examples/internal/demo"
	"github.com/agentruntimecontrolprotocol/go-sdk/middleware/nethttp"
	"github.com/agentruntimecontrolprotocol/go-sdk/server"
)

func main() {
	srv := server.New(server.Options{
		Name:              "heartbeat-demo",
		HeartbeatInterval: 2 * time.Second,
	})
	srv.RegisterAgent("idle", func(ctx context.Context, input json.RawMessage, jc *server.JobContext) (any, error) {
		<-time.After(500 * time.Millisecond)
		return map[string]string{"status": "ok"}, nil
	})
	mux := http.NewServeMux()
	mux.Handle("/arcp", nethttp.NewHandler(srv, nethttp.Options{}))
	httpSrv := &http.Server{Addr: demo.Listen(7820), Handler: mux}
	go func() {
		log.Println("listening on", httpSrv.Addr)
		_ = httpSrv.ListenAndServe()
	}()
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	_ = httpSrv.Shutdown(context.Background())
}

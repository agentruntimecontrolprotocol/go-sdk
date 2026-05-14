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
	srv := server.New(server.Options{Name: "list-jobs"})
	srv.RegisterAgent("idle", func(ctx context.Context, input json.RawMessage, jc *server.JobContext) (any, error) {
		time.Sleep(2 * time.Second)
		return map[string]bool{"ok": true}, nil
	})
	mux := http.NewServeMux()
	mux.Handle("/arcp", nethttp.NewHandler(srv, nethttp.Options{}))
	httpSrv := &http.Server{Addr: demo.Listen(7822), Handler: mux}
	go func() {
		log.Println("listening on", httpSrv.Addr)
		_ = httpSrv.ListenAndServe()
	}()
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	_ = httpSrv.Shutdown(context.Background())
}

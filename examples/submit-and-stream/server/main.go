package main

import (
	"context"
	"encoding/json"
	"fmt"
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
	srv := server.New(server.Options{Name: "submit-and-stream"})
	srv.RegisterAgent("counter", func(ctx context.Context, input json.RawMessage, jc *server.JobContext) (any, error) {
		var req struct {
			N int `json:"n"`
		}
		_ = json.Unmarshal(input, &req)
		if req.N == 0 {
			req.N = 5
		}
		for i := 1; i <= req.N; i++ {
			jc.Log(0, fmt.Sprintf("counted %d", i))
			time.Sleep(50 * time.Millisecond)
		}
		return map[string]any{"counted": req.N}, nil
	})

	mux := http.NewServeMux()
	mux.Handle("/arcp", nethttp.NewHandler(srv, nethttp.Options{}))
	httpSrv := &http.Server{Addr: demo.Listen(7811), Handler: mux}
	go func() {
		log.Println("listening on", httpSrv.Addr)
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	_ = httpSrv.Shutdown(context.Background())
}

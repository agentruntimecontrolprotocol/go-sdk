package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	arcp "github.com/agentruntimecontrolprotocol/go-sdk"
	"github.com/agentruntimecontrolprotocol/go-sdk/credentials"
	"github.com/agentruntimecontrolprotocol/go-sdk/examples/internal/demo"
	"github.com/agentruntimecontrolprotocol/go-sdk/middleware/nethttp"
	"github.com/agentruntimecontrolprotocol/go-sdk/server"
)

func main() {
	provisioner := credentials.NewMemory("demo-cred-")
	srv := server.New(server.Options{
		Name:        "provisioned-credentials",
		Provisioner: provisioner,
	})
	srv.RegisterAgent("chat", func(ctx context.Context, input json.RawMessage, jc *server.JobContext) (any, error) {
		if err := jc.ValidateLeaseOp(arcp.CapModelUse, "tier-fast/gpt-4o-mini"); err != nil {
			return nil, err
		}
		jc.Metric("cost.model", 0.25, "USD", nil)
		return map[string]string{"model": "tier-fast/gpt-4o-mini"}, nil
	})
	mux := http.NewServeMux()
	mux.Handle("/arcp", nethttp.NewHandler(srv, nethttp.Options{}))
	httpSrv := &http.Server{Addr: demo.Listen(7836), Handler: mux}
	go func() {
		log.Println("listening on", httpSrv.Addr)
		_ = httpSrv.ListenAndServe()
	}()
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	_ = httpSrv.Shutdown(context.Background())
}

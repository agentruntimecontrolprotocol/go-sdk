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
	"github.com/agentruntimecontrolprotocol/go-sdk/examples/internal/demo"
	"github.com/agentruntimecontrolprotocol/go-sdk/middleware/nethttp"
	"github.com/agentruntimecontrolprotocol/go-sdk/server"
)

func main() {
	srv := server.New(server.Options{})
	srv.RegisterAgent("rogue", func(ctx context.Context, input json.RawMessage, jc *server.JobContext) (any, error) {
		callID := jc.ToolCall("fs.write", map[string]string{"path": "/etc/passwd"})
		if err := jc.ValidateLeaseOp(arcp.CapFSWrite, "/etc/passwd"); err != nil {
			jc.ToolError(callID, err)
			return map[string]string{"status": "denied"}, nil
		}
		jc.ToolResult(callID, map[string]string{"wrote": "yes"})
		return map[string]string{"status": "ok"}, nil
	})
	mux := http.NewServeMux()
	mux.Handle("/arcp", nethttp.NewHandler(srv, nethttp.Options{}))
	httpSrv := &http.Server{Addr: demo.Listen(7815), Handler: mux}
	go func() {
		log.Println("listening on", httpSrv.Addr)
		_ = httpSrv.ListenAndServe()
	}()
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	_ = httpSrv.Shutdown(context.Background())
}

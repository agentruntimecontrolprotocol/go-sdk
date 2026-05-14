package main

import (
	"context"
	"encoding/json"
	"errors"
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
	srv := server.New(server.Options{Name: "cost-budget"})
	srv.RegisterAgent("researcher", func(ctx context.Context, input json.RawMessage, jc *server.JobContext) (any, error) {
		for i := 0; i < 3; i++ {
			if err := jc.ValidateLeaseOp(arcp.CapToolCall, "search.web"); err != nil {
				if errors.Is(err, arcp.ErrBudgetExhausted) {
					jc.ToolError("c"+itoa(i), err)
					return map[string]string{"status": "budget exhausted"}, nil
				}
				return nil, err
			}
			call := jc.ToolCall("search.web", map[string]any{"q": "topic"})
			jc.ToolResult(call, map[string]any{"hits": 3})
			jc.Metric("cost.search", 0.42, "USD", nil)
		}
		return map[string]any{"status": "done"}, nil
	})
	mux := http.NewServeMux()
	mux.Handle("/arcp", nethttp.NewHandler(srv, nethttp.Options{}))
	httpSrv := &http.Server{Addr: demo.Listen(7826), Handler: mux}
	go func() {
		log.Println("listening on", httpSrv.Addr)
		_ = httpSrv.ListenAndServe()
	}()
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	_ = httpSrv.Shutdown(context.Background())
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	out := []byte{}
	for i > 0 {
		out = append([]byte{byte('0' + i%10)}, out...)
		i /= 10
	}
	return string(out)
}

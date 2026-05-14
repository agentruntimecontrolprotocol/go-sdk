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
	arcpchi "github.com/agentruntimecontrolprotocol/go-sdk/middleware/chi"
	"github.com/agentruntimecontrolprotocol/go-sdk/server"
	chir "github.com/go-chi/chi/v5"
)

func main() {
	srv := server.New(server.Options{})
	srv.RegisterAgent("echo", func(ctx context.Context, input json.RawMessage, jc *server.JobContext) (any, error) {
		return map[string]json.RawMessage{"echo": input}, nil
	})
	r := chir.NewRouter()
	arcpchi.Mount(r, srv, arcpchi.Options{})
	r.Get("/jobs", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("[]"))
	})
	httpSrv := &http.Server{Addr: demo.Listen(7831), Handler: r}
	go func() {
		log.Println("listening on", httpSrv.Addr)
		_ = httpSrv.ListenAndServe()
	}()
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	_ = httpSrv.Shutdown(context.Background())
}

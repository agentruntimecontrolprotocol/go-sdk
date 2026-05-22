package main

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"

	"github.com/agentruntimecontrolprotocol/go-sdk/middleware/nethttp"
	"github.com/agentruntimecontrolprotocol/go-sdk/server"
)

func main() {
	srv := server.New(server.Options{Name: "stream-resume"})
	srv.RegisterAgent("streamer", func(ctx context.Context, input json.RawMessage, jc *server.JobContext) (any, error) {
		w, err := jc.StreamResult("utf8")
		if err != nil {
			return nil, err
		}
		_, _ = io.WriteString(w, "chunk-one\n")
		_, _ = io.WriteString(w, "chunk-two\n")
		return nil, w.Close()
	})
	mux := http.NewServeMux()
	mux.Handle("/arcp", nethttp.NewHandler(srv, nethttp.Options{}))
	log.Fatal(http.ListenAndServe(":7843", mux))
}

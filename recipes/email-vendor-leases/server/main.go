package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"

	"github.com/agentruntimecontrolprotocol/go-sdk/middleware/nethttp"
	"github.com/agentruntimecontrolprotocol/go-sdk/server"
)

func main() {
	srv := server.New(server.Options{Name: "email-vendor-leases"})
	srv.RegisterAgent("mailer", func(ctx context.Context, input json.RawMessage, jc *server.JobContext) (any, error) {
		if err := jc.ValidateLeaseOp("x-vendor.acme.email.send", "tenant-a/welcome"); err != nil {
			return nil, err
		}
		return map[string]string{"queued": "tenant-a/welcome"}, nil
	})
	mux := http.NewServeMux()
	mux.Handle("/arcp", nethttp.NewHandler(srv, nethttp.Options{}))
	log.Fatal(http.ListenAndServe(":7841", mux))
}

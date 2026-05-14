// Package chi mounts an ARCP server on a chi.Router.
package chi

import (
	"github.com/agentruntimecontrolprotocol/go-sdk/middleware/nethttp"
	"github.com/agentruntimecontrolprotocol/go-sdk/server"
	chir "github.com/go-chi/chi/v5"
)

// Options is an alias for nethttp.Options.
type Options = nethttp.Options

// Mount attaches an ARCP handler to r at opts.Path (default "/arcp").
// The returned *nethttp.Handler can be used for graceful shutdown.
func Mount(r chir.Router, srv *server.Server, opts Options) *nethttp.Handler {
	h := nethttp.NewHandler(srv, opts)
	path := opts.Path
	if path == "" {
		path = "/arcp"
	}
	r.Handle(path, h)
	return h
}

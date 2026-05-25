// Package nethttp exposes a net/http handler that upgrades incoming
// requests to WebSocket and hands the resulting connection to a
// server.Server.
package nethttp

import (
	"context"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/agentruntimecontrolprotocol/go-sdk/server"
	"github.com/agentruntimecontrolprotocol/go-sdk/transport"
	"github.com/coder/websocket"
)

// Options configures the HTTP handler.
type Options struct {
	// Path is the request path served by the handler. Defaults to
	// "/arcp"; the handler returns 404 for any other request path
	// when invoked through a parent mux that does not strip the
	// prefix.
	Path string
	// AllowedHosts is the list of acceptable HTTP Host headers. The
	// SDK default is the loopback set per spec §14 DNS-rebind
	// protection.
	AllowedHosts []string
	// ReadLimit caps the inbound WebSocket frame size in bytes. Zero
	// uses 1 MiB.
	ReadLimit int64
	// Subprotocols negotiated with the client. Empty selects none.
	Subprotocols []string
	// Origins allowed for browser clients. nil disables CORS.
	Origins []string
	// PingInterval drives WS-layer keepalives. Zero disables.
	PingInterval time.Duration
}

func (o Options) withDefaults() Options {
	if o.Path == "" {
		o.Path = "/arcp"
	}
	if len(o.AllowedHosts) == 0 {
		o.AllowedHosts = []string{"localhost", "127.0.0.1", "[::1]"}
	}
	if o.ReadLimit == 0 {
		o.ReadLimit = 1 << 20
	}
	return o
}

// Handler is the http.Handler returned by NewHandler. It is also
// callable as a graceful shutter via Shutdown.
type Handler struct {
	opts   Options
	srv    *server.Server
	mu     sync.Mutex
	active map[uint64]*websocket.Conn
	nextID uint64
	// drained is signalled (closed and reopened) every time a
	// connection is removed from active so Shutdown can wake without
	// polling. Set by removeConn; replaced under mu.
	drained chan struct{}
}

// NewHandler returns a Handler that upgrades requests on opts.Path
// to WebSocket and serves them with srv.
func NewHandler(srv *server.Server, opts Options) *Handler {
	return &Handler{
		opts:    opts.withDefaults(),
		srv:     srv,
		active:  map[uint64]*websocket.Conn{},
		drained: make(chan struct{}),
	}
}

// removeConn deletes id from active and signals any Shutdown waiter
// that one connection has drained. The drained channel is replaced
// under the lock so subsequent Shutdown attempts get a fresh signal.
func (h *Handler) removeConn(id uint64) {
	h.mu.Lock()
	delete(h.active, id)
	prev := h.drained
	h.drained = make(chan struct{})
	h.mu.Unlock()
	close(prev)
}

// ServeHTTP implements http.Handler.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if h.opts.Path != "" && r.URL.Path != h.opts.Path {
		http.NotFound(w, r)
		return
	}
	if !hostAllowed(r.Host, h.opts.AllowedHosts) {
		http.Error(w, "host not allowed", http.StatusMisdirectedRequest)
		return
	}
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		Subprotocols:   h.opts.Subprotocols,
		OriginPatterns: h.opts.Origins,
	})
	if err != nil {
		return
	}
	conn.SetReadLimit(h.opts.ReadLimit)
	t := transport.NewWebSocket(conn)
	h.mu.Lock()
	h.nextID++
	id := h.nextID
	h.active[id] = conn
	h.mu.Unlock()
	defer h.removeConn(id)
	if h.opts.PingInterval > 0 {
		pingCtx, cancelPing := context.WithCancel(r.Context())
		defer cancelPing()
		go pingLoop(pingCtx, conn, h.opts.PingInterval)
	}
	_ = h.srv.Accept(r.Context(), t)
}

// pingLoop sends a WebSocket-layer Ping at the configured interval
// until ctx is cancelled or the conn fails. WS pings are independent
// of the ARCP-level session.ping heartbeat — they keep idle TCP
// connections alive through NAT timeouts and load balancers.
func pingLoop(ctx context.Context, conn *websocket.Conn, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			pingCtx, cancel := context.WithTimeout(ctx, interval)
			err := conn.Ping(pingCtx)
			cancel()
			if err != nil {
				return
			}
		}
	}
}

// Shutdown closes every active WebSocket with status 1001 (Going
// Away) and waits until the active connection map is empty or ctx
// expires. Returns nil when all connections drain before the deadline
// and ctx.Err when the context expires. Calling Shutdown on a handler
// with no active connections returns immediately with nil.
func (h *Handler) Shutdown(ctx context.Context) error {
	h.mu.Lock()
	conns := make([]*websocket.Conn, 0, len(h.active))
	for _, c := range h.active {
		conns = append(conns, c)
	}
	h.mu.Unlock()
	for _, c := range conns {
		_ = c.Close(websocket.StatusGoingAway, "shutdown")
	}
	for {
		h.mu.Lock()
		n := len(h.active)
		drained := h.drained
		h.mu.Unlock()
		if n == 0 {
			return nil
		}
		select {
		case <-drained:
			// Loop and recheck active count.
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func hostAllowed(hostHeader string, allowed []string) bool {
	host := hostHeader
	if h, _, err := net.SplitHostPort(hostHeader); err == nil {
		host = h
	}
	for _, a := range allowed {
		if a == host {
			return true
		}
	}
	return false
}

package transport

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"

	arcp "github.com/agentruntimecontrolprotocol/go-sdk"
	"github.com/coder/websocket"
)

// WebSocketOptions configures a WebSocket transport.
type WebSocketOptions struct {
	// Subprotocols is the list of WebSocket subprotocols to negotiate.
	// Empty means "no subprotocol".
	Subprotocols []string
	// HTTPHeader is sent on the dial request (clients only).
	HTTPHeader http.Header
	// HTTPClient overrides the dial client. Optional.
	HTTPClient *http.Client
	// ReadLimit caps the inbound frame size in bytes. Zero uses 1 MiB.
	ReadLimit int64
}

// DialWebSocket opens a client WebSocket transport to url.
func DialWebSocket(ctx context.Context, url string, opts WebSocketOptions) (Transport, error) {
	dialOpts := &websocket.DialOptions{
		Subprotocols: opts.Subprotocols,
		HTTPHeader:   opts.HTTPHeader,
		HTTPClient:   opts.HTTPClient,
	}
	conn, _, err := websocket.Dial(ctx, url, dialOpts)
	if err != nil {
		return nil, fmt.Errorf("dial websocket: %w", err)
	}
	limit := opts.ReadLimit
	if limit == 0 {
		limit = 1 << 20
	}
	conn.SetReadLimit(limit)
	return NewWebSocket(conn), nil
}

// NewWebSocket wraps an established websocket.Conn as a Transport.
// The transport takes ownership of conn; Close releases it.
func NewWebSocket(conn *websocket.Conn) Transport {
	return &wsTransport{conn: conn}
}

type wsTransport struct {
	conn   *websocket.Conn
	writeM sync.Mutex
	closed atomicBool
}

// Send marshals env and writes one text frame.
func (t *wsTransport) Send(ctx context.Context, env arcp.Envelope) error {
	if t.closed.Get() {
		return ErrClosed
	}
	if env.ARCP == "" {
		env.ARCP = arcp.ProtocolVersion
	}
	body, err := json.Marshal(env)
	if err != nil {
		return fmt.Errorf("marshal envelope: %w", err)
	}
	t.writeM.Lock()
	defer t.writeM.Unlock()
	if err := t.conn.Write(ctx, websocket.MessageText, body); err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return ctx.Err()
		}
		return err
	}
	return nil
}

// Recv reads one frame and unmarshals it into an envelope.
func (t *wsTransport) Recv(ctx context.Context) (arcp.Envelope, error) {
	if t.closed.Get() {
		return arcp.Envelope{}, ErrClosed
	}
	_, body, err := t.conn.Read(ctx)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return arcp.Envelope{}, ctx.Err()
		}
		return arcp.Envelope{}, err
	}
	var env arcp.Envelope
	if err := json.Unmarshal(body, &env); err != nil {
		return arcp.Envelope{}, arcp.ErrInvalidRequest.WithMessage("decode envelope: " + err.Error())
	}
	return env, nil
}

// Close gracefully shuts the conn down with status 1000 / "bye".
func (t *wsTransport) Close() error {
	if !t.closed.Set(true) {
		return nil
	}
	return t.conn.Close(websocket.StatusNormalClosure, "bye")
}

type atomicBool struct {
	mu sync.Mutex
	v  bool
}

func (a *atomicBool) Get() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.v
}

// Set sets the boolean to v and returns true if the value changed.
func (a *atomicBool) Set(v bool) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.v == v {
		return false
	}
	a.v = v
	return true
}

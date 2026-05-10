package client

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/agentruntimecontrolprotocol/go-sdk"
	"github.com/agentruntimecontrolprotocol/go-sdk/messages"
	"github.com/agentruntimecontrolprotocol/go-sdk/transport"
)

// OpenOptions configures the session.open call.
type OpenOptions struct {
	Auth         messages.Auth
	Client       messages.ClientIdentity
	Capabilities messages.Capabilities
	Logger       *slog.Logger
}

// Client is an active ARCP client wrapping a single open session
// (RFC §9). Construct one via Open.
type Client struct {
	t          transport.Transport
	sessionID  arcp.SessionID
	serverCaps messages.Capabilities
	logger     *slog.Logger
}

// Open performs the four-step handshake (RFC §8.1) and returns a
// connected Client. v0.1 supports the no-challenge happy path; if the
// runtime returns a session.challenge, Open returns UNIMPLEMENTED so
// the caller knows to wire up signed_jwt challenge handling (deferred
// to v0.2).
func Open(ctx context.Context, t transport.Transport, opts OpenOptions) (*Client, error) {
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	open := arcp.Envelope{
		ID:        arcp.NewMessageID(),
		Timestamp: time.Now().UTC(),
		Payload: &messages.SessionOpen{
			Auth:         opts.Auth,
			Client:       opts.Client,
			Capabilities: opts.Capabilities,
		},
	}
	if err := t.Send(ctx, open); err != nil {
		return nil, fmt.Errorf("client.Open: send session.open: %w", err)
	}
	resp, err := t.Recv(ctx)
	if err != nil {
		return nil, fmt.Errorf("client.Open: receive handshake response: %w", err)
	}
	switch p := resp.Payload.(type) {
	case *messages.SessionAccepted:
		return &Client{
			t:          t,
			sessionID:  p.SessionID,
			serverCaps: p.Capabilities,
			logger:     opts.Logger,
		}, nil
	case *messages.SessionRejected:
		return nil, arcp.NewError(p.Code, p.Message)
	case *messages.SessionUnauthenticated:
		return nil, arcp.ErrUnauthenticated.WithMessage(p.Message)
	case *messages.SessionChallenge:
		return nil, arcp.ErrUnimplemented.WithMessage(
			"client.Open: session.challenge handling deferred to v0.2")
	default:
		return nil, arcp.NewError(arcp.CodeFailedPrecondition,
			fmt.Sprintf("client.Open: unexpected response %s", resp.Type()))
	}
}

// SessionID returns the id assigned by the runtime in session.accepted.
func (c *Client) SessionID() arcp.SessionID { return c.sessionID }

// Capabilities returns the negotiated capability set.
func (c *Client) Capabilities() messages.Capabilities { return c.serverCaps }

// Send delivers env to the runtime. Sets env.SessionID, env.ID, and
// env.Timestamp if unset.
func (c *Client) Send(ctx context.Context, env arcp.Envelope) error {
	if env.ID == "" {
		env.ID = arcp.NewMessageID()
	}
	if env.SessionID == "" {
		env.SessionID = c.sessionID
	}
	if env.Timestamp.IsZero() {
		env.Timestamp = time.Now().UTC()
	}
	return c.t.Send(ctx, env)
}

// Recv waits for the next envelope from the runtime.
func (c *Client) Recv(ctx context.Context) (arcp.Envelope, error) {
	return c.t.Recv(ctx)
}

// Ping sends a ping and waits for the matching pong. Convenience
// helper used by tests and by simple clients that aren't running a
// pending registry.
func (c *Client) Ping(ctx context.Context, note string) (string, error) {
	if err := c.Send(ctx, arcp.Envelope{Payload: &messages.Ping{Note: note}}); err != nil {
		return "", err
	}
	resp, err := c.Recv(ctx)
	if err != nil {
		return "", err
	}
	pong, ok := resp.Payload.(*messages.Pong)
	if !ok {
		return "", arcp.NewError(arcp.CodeFailedPrecondition,
			fmt.Sprintf("ping: expected pong, got %s", resp.Type()))
	}
	return pong.Note, nil
}

// Close sends session.close and shuts down the transport.
func (c *Client) Close(ctx context.Context) error {
	if err := c.Send(ctx, arcp.Envelope{Payload: &messages.SessionClose{Reason: "client_close"}}); err != nil {
		c.logger.Warn("client.Close: session.close send failed", "error", err)
	}
	return c.t.Close()
}

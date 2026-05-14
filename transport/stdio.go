package transport

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"sync"

	arcp "github.com/agentruntimecontrolprotocol/go-sdk"
)

// NewStdioTransport wraps an io.Reader / io.Writer pair as a Transport
// using NDJSON framing (one JSON envelope per line). Closes the reader
// and writer (if they are io.Closer) when Close is called.
func NewStdioTransport(in io.Reader, out io.Writer) Transport {
	return &stdioTransport{
		scanner: bufio.NewScanner(in),
		out:     out,
		in:      in,
	}
}

type stdioTransport struct {
	scanner *bufio.Scanner
	out     io.Writer
	in      io.Reader
	writeM  sync.Mutex
	closed  atomicBool
}

// Send writes env as one NDJSON line.
func (t *stdioTransport) Send(ctx context.Context, env arcp.Envelope) error {
	if t.closed.Get() {
		return ErrClosed
	}
	if env.ARCP == "" {
		env.ARCP = arcp.ProtocolVersion
	}
	body, err := json.Marshal(env)
	if err != nil {
		return err
	}
	t.writeM.Lock()
	defer t.writeM.Unlock()
	body = append(body, '\n')
	if _, err := t.out.Write(body); err != nil {
		return err
	}
	return nil
}

// Recv reads one NDJSON line.
func (t *stdioTransport) Recv(ctx context.Context) (arcp.Envelope, error) {
	if t.closed.Get() {
		return arcp.Envelope{}, ErrClosed
	}
	// Run the scan on a goroutine so ctx cancellation can preempt.
	type result struct {
		env arcp.Envelope
		err error
	}
	ch := make(chan result, 1)
	go func() {
		if !t.scanner.Scan() {
			err := t.scanner.Err()
			if err == nil {
				err = io.EOF
			}
			ch <- result{err: err}
			return
		}
		line := t.scanner.Bytes()
		var env arcp.Envelope
		if err := json.Unmarshal(line, &env); err != nil {
			ch <- result{err: arcp.ErrInvalidRequest.WithMessage("decode envelope: " + err.Error())}
			return
		}
		ch <- result{env: env}
	}()
	select {
	case r := <-ch:
		return r.env, r.err
	case <-ctx.Done():
		return arcp.Envelope{}, ctx.Err()
	}
}

// Close releases the underlying reader/writer if they implement
// io.Closer.
func (t *stdioTransport) Close() error {
	if !t.closed.Set(true) {
		return nil
	}
	var firstErr error
	if c, ok := t.in.(io.Closer); ok {
		if err := c.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if c, ok := t.out.(io.Closer); ok && c != any(t.in) {
		if err := c.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if firstErr != nil && !errors.Is(firstErr, io.ErrClosedPipe) {
		return firstErr
	}
	return nil
}

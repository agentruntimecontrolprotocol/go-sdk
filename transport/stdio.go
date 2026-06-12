package transport

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"sync"
	"sync/atomic"

	arcp "github.com/agentruntimecontrolprotocol/go-sdk"
)

// defaultStdioMaxLine bounds a single NDJSON line. The default
// bufio.Scanner token size is only 64 KiB, which would kill the
// transport on any larger envelope; this mirrors the 1 MiB WebSocket
// ReadLimit default.
const defaultStdioMaxLine = 1 << 20

// NewStdioTransport wraps an io.Reader / io.Writer pair as a Transport
// using NDJSON framing (one JSON envelope per line). Lines up to 1 MiB
// are accepted; larger lines fail the read with an explicit error
// rather than silently truncating. Closes the reader and writer (if
// they are io.Closer) when Close is called.
func NewStdioTransport(in io.Reader, out io.Writer) Transport {
	sc := bufio.NewScanner(in)
	sc.Buffer(make([]byte, 0, 64*1024), defaultStdioMaxLine)
	t := &stdioTransport{
		scanner: sc,
		out:     out,
		in:      in,
		recvCh:  make(chan recvResult, 1),
		done:    make(chan struct{}),
	}
	go t.readerLoop()
	return t
}

type recvResult struct {
	env arcp.Envelope
	err error
}

type stdioTransport struct {
	scanner *bufio.Scanner
	out     io.Writer
	in      io.Reader
	writeM  sync.Mutex
	closed  atomic.Bool
	// recvCh feeds the dedicated reader goroutine's decoded envelopes
	// (or errors) into Recv. Sized to 1; the reader blocks on the
	// next scan after each delivery, so Recv is the back-pressure
	// signal.
	recvCh chan recvResult
	// done is closed when readerLoop exits, signalling that no further
	// envelopes will arrive.
	done chan struct{}
}

// Send writes env as one NDJSON line.
func (t *stdioTransport) Send(ctx context.Context, env arcp.Envelope) error {
	if t.closed.Load() {
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

// Recv selects on the dedicated reader goroutine's channel so context
// cancellation never leaks a per-call scan goroutine and the
// underlying bufio.Scanner is only ever touched by one goroutine for
// the lifetime of the transport.
func (t *stdioTransport) Recv(ctx context.Context) (arcp.Envelope, error) {
	select {
	case r, ok := <-t.recvCh:
		if !ok {
			return arcp.Envelope{}, ErrClosed
		}
		return r.env, r.err
	case <-t.done:
		return arcp.Envelope{}, ErrClosed
	case <-ctx.Done():
		return arcp.Envelope{}, ctx.Err()
	}
}

// readerLoop owns the bufio.Scanner. It loops Scan → decode → send on
// recvCh. It exits when Scan returns false (EOF or error), at which
// point recvCh is closed so subsequent Recv calls observe EOF.
func (t *stdioTransport) readerLoop() {
	defer close(t.done)
	defer close(t.recvCh)
	for {
		if !t.scanner.Scan() {
			err := t.scanner.Err()
			if errors.Is(err, bufio.ErrTooLong) {
				err = arcp.ErrInvalidRequest.WithMessage("NDJSON line exceeds maximum size")
			}
			if err == nil {
				err = io.EOF
			}
			select {
			case t.recvCh <- recvResult{err: err}:
			case <-t.done:
			}
			return
		}
		line := append([]byte(nil), t.scanner.Bytes()...)
		var env arcp.Envelope
		if err := json.Unmarshal(line, &env); err != nil {
			select {
			case t.recvCh <- recvResult{err: arcp.ErrInvalidRequest.WithMessage("decode envelope: " + err.Error())}:
			case <-t.done:
				return
			}
			continue
		}
		select {
		case t.recvCh <- recvResult{env: env}:
		case <-t.done:
			return
		}
	}
}

// Close releases the underlying reader/writer if they implement
// io.Closer. Closing the reader unblocks the dedicated scan goroutine.
func (t *stdioTransport) Close() error {
	if !t.closed.CompareAndSwap(false, true) {
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

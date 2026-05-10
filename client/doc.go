// Package client provides the active-client wrapper around an ARCP
// session. It opens the transport, completes the four-message
// handshake (RFC §8.1), and exposes ergonomic methods for invoking
// tools, subscribing to events, and registering handlers for
// human-in-the-loop and permission challenges.
package client

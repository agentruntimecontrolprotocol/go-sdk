// Package demo provides shared helpers for the examples — addr
// resolution and a tiny "log fatal on error" idiom.
package demo

import (
	"fmt"
	"log"
	"os"
)

// Addr returns ws://127.0.0.1:<port>/arcp. If $ARCP_DEMO_PORT is set
// it overrides defaultPort.
func Addr(defaultPort int) string {
	port := defaultPort
	if v := os.Getenv("ARCP_DEMO_PORT"); v != "" {
		var p int
		_, err := fmt.Sscan(v, &p)
		if err == nil && p > 0 {
			port = p
		}
	}
	return fmt.Sprintf("ws://127.0.0.1:%d/arcp", port)
}

// Listen returns :<port>. Same env override.
func Listen(defaultPort int) string {
	port := defaultPort
	if v := os.Getenv("ARCP_DEMO_PORT"); v != "" {
		var p int
		_, err := fmt.Sscan(v, &p)
		if err == nil && p > 0 {
			port = p
		}
	}
	return fmt.Sprintf(":%d", port)
}

// Must logs err and exits 1.
func Must(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

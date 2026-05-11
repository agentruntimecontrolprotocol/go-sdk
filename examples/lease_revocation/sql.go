// SQL classifier — sqlglot-equivalent in production. Real version
// uses a Go SQL parser (e.g. github.com/auxten/postgresql-parser) and
// walks the AST for table refs.
package main

import (
	"context"

	arcp "github.com/agentruntimecontrolprotocol/go-sdk"
)

// StatementClass is the {read|write|ddl} verdict + table refs.
type StatementClass struct {
	Op     string // "read" | "write" | "ddl"
	Tables []string
}

func classify(string) StatementClass {
	panic("not implemented: classify")
}

// Session: as-if-published high-level wrapper.
type Session struct{}

func openAdmin(context.Context) *Session {
	panic("not implemented: openAdmin — transport, identity, auth elided")
}
func (*Session) Request(context.Context, *arcp.Envelope) (*arcp.Envelope, error) {
	panic("not implemented")
}
func (*Session) Send(context.Context, *arcp.Envelope) error  { panic("not implemented") }
func (*Session) Events(context.Context) <-chan arcp.Envelope { panic("not implemented") }
func (*Session) Close(context.Context) error                 { return nil }

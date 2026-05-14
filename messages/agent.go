package messages

import (
	"strings"

	arcp "github.com/agentruntimecontrolprotocol/go-sdk"
)

// AgentRef is a parsed agent identifier ("name" or "name@version").
type AgentRef struct {
	Name    string
	Version string
}

// String returns the canonical "name@version" or bare "name" form.
func (a AgentRef) String() string {
	if a.Version == "" {
		return a.Name
	}
	return a.Name + "@" + a.Version
}

// ParseAgentRef parses an agent identifier per the spec grammar:
//
//	agent   ::= name | name "@" version
//	name    ::= [a-z0-9][a-z0-9._-]*
//	version ::= [a-zA-Z0-9.+_-]+
func ParseAgentRef(s string) (AgentRef, error) {
	if s == "" {
		return AgentRef{}, arcp.ErrInvalidRequest.WithMessage("agent must be non-empty")
	}
	parts := strings.SplitN(s, "@", 2)
	name := parts[0]
	if !validAgentName(name) {
		return AgentRef{}, arcp.ErrInvalidRequest.WithMessage("agent name does not match [a-z0-9][a-z0-9._-]*")
	}
	if len(parts) == 1 {
		return AgentRef{Name: name}, nil
	}
	version := parts[1]
	if !validAgentVersion(version) {
		return AgentRef{}, arcp.ErrInvalidRequest.WithMessage("agent version does not match [a-zA-Z0-9.+_-]+")
	}
	return AgentRef{Name: name, Version: version}, nil
}

func validAgentName(s string) bool {
	if s == "" {
		return false
	}
	c0 := s[0]
	if !((c0 >= 'a' && c0 <= 'z') || (c0 >= '0' && c0 <= '9')) {
		return false
	}
	for i := 1; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'a' && c <= 'z':
		case c >= '0' && c <= '9':
		case c == '.' || c == '_' || c == '-':
		default:
			return false
		}
	}
	return true
}

func validAgentVersion(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'a' && c <= 'z':
		case c >= 'A' && c <= 'Z':
		case c >= '0' && c <= '9':
		case c == '.' || c == '_' || c == '-' || c == '+':
		default:
			return false
		}
	}
	return true
}

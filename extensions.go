package arcp

import (
	"fmt"
	"regexp"
	"strings"
)

// Extension namespace patterns per RFC §21.1.
//
// arcpxNamespacePattern matches "arcpx.<vendor-or-domain>.<name>.v<n>".
// reverseDNSPattern matches a reverse-DNS form ending in ".v<n>", with
// at least two dotted labels before the version suffix.
//
// Both patterns are precompiled at package load. They are read-only
// after construction so they qualify as "true constants" under the
// no-globals rule.
var (
	arcpxNamespacePattern = regexp.MustCompile(`^arcpx\.[a-z0-9-]+(?:\.[a-z0-9-]+)*\.v\d+$`)
	reverseDNSPattern     = regexp.MustCompile(`^[a-z0-9-]+(?:\.[a-z0-9-]+){1,}\.v\d+$`)
)

// ValidateExtensionType returns nil if t is a valid extension namespace
// per RFC §21.1, or an *Error with code INVALID_ARGUMENT otherwise.
//
// The bare "x-" prefix is reserved for transport-internal experimental
// fields (RFC §21.1) and is rejected for use as a long-lived message
// type.
func ValidateExtensionType(t string) error {
	if t == "" {
		return NewError(CodeInvalidArgument, "extension type is empty")
	}
	if strings.HasPrefix(t, "x-") {
		return NewError(CodeInvalidArgument,
			fmt.Sprintf("extension type %q uses the reserved x- prefix (RFC §21.1)", t))
	}
	if arcpxNamespacePattern.MatchString(t) {
		return nil
	}
	if reverseDNSPattern.MatchString(t) {
		return nil
	}
	return NewError(CodeInvalidArgument,
		fmt.Sprintf("extension type %q does not match arcpx.<vendor>.<name>.v<n> or reverse-DNS form", t))
}

// IsCoreType reports whether the wire type belongs to the core ARCP
// surface (i.e. is not namespaced as an extension).
func IsCoreType(t string) bool {
	if t == "" {
		return false
	}
	if strings.HasPrefix(t, "arcpx.") {
		return false
	}
	if reverseDNSPattern.MatchString(t) {
		return false
	}
	return true
}

// ExtensionRegistry tracks extensions advertised by a session (its own
// or its peer's). It is used to negotiate (RFC §21.2) and to apply the
// unknown-message handling rules (RFC §21.3): unknown extension types
// that the peer did not advertise are NACKed; advertised extensions are
// forwarded for handler dispatch.
//
// A zero-value ExtensionRegistry is empty and safe to use.
type ExtensionRegistry struct {
	known map[string]struct{}
}

// NewExtensionRegistry returns a new empty registry.
func NewExtensionRegistry() *ExtensionRegistry {
	return &ExtensionRegistry{known: make(map[string]struct{})}
}

// Add registers an extension namespace. Returns an error if the
// namespace is invalid per ValidateExtensionType.
func (r *ExtensionRegistry) Add(name string) error {
	if err := ValidateExtensionType(name); err != nil {
		return err
	}
	if r.known == nil {
		r.known = make(map[string]struct{})
	}
	r.known[name] = struct{}{}
	return nil
}

// Has reports whether the given extension namespace has been
// registered. The lookup matches on full namespace string.
func (r *ExtensionRegistry) Has(name string) bool {
	if r == nil || r.known == nil {
		return false
	}
	_, ok := r.known[name]
	return ok
}

// List returns the registered namespaces in deterministic order
// (alphabetical) so equality testing and capability advertisement are
// stable.
func (r *ExtensionRegistry) List() []string {
	if r == nil || len(r.known) == 0 {
		return nil
	}
	out := make([]string, 0, len(r.known))
	for k := range r.known {
		out = append(out, k)
	}
	// Stable order (alphabetical) keeps tests deterministic.
	sortStrings(out)
	return out
}

// sortStrings is a tiny insertion sort to avoid pulling in sort just
// for short slices. ExtensionRegistry typically holds a handful of
// entries.
func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j-1] > s[j]; j-- {
			s[j-1], s[j] = s[j], s[j-1]
		}
	}
}

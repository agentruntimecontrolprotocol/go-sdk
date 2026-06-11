package glob_test

import (
	"testing"

	"github.com/agentruntimecontrolprotocol/go-sdk/internal/glob"
)

func TestMatch(t *testing.T) {
	cases := []struct {
		pattern string
		path    string
		want    bool
	}{
		{"/foo/*", "/foo/bar", true},
		{"/foo/*", "/foo/bar/baz", false},
		{"/foo/**", "/foo/bar/baz", true},
		{"/foo/**", "/foo", true},
		{"/foo/**/x", "/foo/a/b/x", true},
		{"tool.*", "tool.search", true},
		{"tool.*", "tool", false},
		{"search.*", "search.web", true},
	}
	for _, tc := range cases {
		got := glob.Match(tc.pattern, tc.path)
		if got != tc.want {
			t.Errorf("Match(%q,%q) = %v want %v", tc.pattern, tc.path, got, tc.want)
		}
	}
}

// TestCovers covers #147: pattern-inclusion must be sound — a parent
// may only cover a child whose language it fully contains.
func TestCovers(t *testing.T) {
	cases := []struct {
		parent string
		child  string
		want   bool
	}{
		{"/data/*", "/data/**", false},  // child widens: reject
		{"/data/**", "/data/*", true},   // parent wider: accept
		{"/data/**", "/data/**", true},  // identical
		{"/data/*", "/data/*", true},    // identical
		{"/data/*", "/data/x", true},    // concrete literal under *
		{"/data/*", "/data/x/y", false}, // * is one segment only
		{"/data/**", "/data/x/y", true},
		{"a*", "a**", false}, // child wildcard segment differs
		{"a*", "*", false},   // child * is broader than a*
		{"a*", "abc", true},  // concrete literal matches a*
		{"*", "abc", true},
		{"*", "**", false},
		{"**", "anything/deep/path", true},
		{"tier-fast/*", "tier-fast/**", false},
		{"tier-fast/**", "tier-fast/x", true},
	}
	for _, tc := range cases {
		if got := glob.Covers(tc.parent, tc.child); got != tc.want {
			t.Errorf("Covers(%q,%q) = %v want %v", tc.parent, tc.child, got, tc.want)
		}
	}
}

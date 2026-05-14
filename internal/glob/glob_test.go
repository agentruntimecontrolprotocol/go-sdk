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

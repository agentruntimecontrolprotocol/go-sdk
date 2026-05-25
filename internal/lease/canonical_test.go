package lease

import "testing"

func TestCanonicalizePath(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"", ""},
		{"/foo/bar", "/foo/bar"},
		{"/foo//bar", "/foo/bar"},
		{"/foo/./bar", "/foo/bar"},
		{"/foo/../bar", "/bar"},
		{"foo/bar", "foo/bar"},
	}
	for _, tc := range cases {
		if got := CanonicalizePath(tc.in); got != tc.want {
			t.Errorf("CanonicalizePath(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestCanonicalizeURL(t *testing.T) {
	cases := []struct {
		in, want string
		err      bool
	}{
		{"HTTPS://API.Example.com/v1/x", "https://api.example.com/v1/x", false},
		{"http://h//double", "http://h/double", false},
		{":://bad", "", true},
	}
	for _, tc := range cases {
		got, err := CanonicalizeURL(tc.in)
		if tc.err {
			if err == nil {
				t.Errorf("CanonicalizeURL(%q) want err", tc.in)
			}
			continue
		}
		if err != nil {
			t.Errorf("CanonicalizeURL(%q): %v", tc.in, err)
		}
		if got != tc.want {
			t.Errorf("CanonicalizeURL(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

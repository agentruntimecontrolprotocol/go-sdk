package lease

import (
	"net/url"
	"path"
	"strings"
)

// CanonicalizePath collapses dot segments and double slashes. A
// leading slash is preserved.
func CanonicalizePath(p string) string {
	if p == "" {
		return ""
	}
	abs := strings.HasPrefix(p, "/")
	clean := path.Clean(p)
	if abs && !strings.HasPrefix(clean, "/") {
		clean = "/" + clean
	}
	return clean
}

// CanonicalizeURL lowercases scheme/host and normalises path.
func CanonicalizeURL(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	u.Scheme = strings.ToLower(u.Scheme)
	u.Host = strings.ToLower(u.Host)
	if u.Path != "" {
		u.Path = path.Clean(u.Path)
	}
	return u.String(), nil
}

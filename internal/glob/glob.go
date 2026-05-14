// Package glob implements the lease pattern matcher. * matches any
// single path segment; ** matches zero or more segments.
package glob

import "strings"

// Match reports whether pattern matches s. Separator is '/'.
func Match(pattern, s string) bool {
	return matchSegments(splitSep(pattern), splitSep(s))
}

func splitSep(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(s, "/")
}

func matchSegments(p, s []string) bool {
	if len(p) == 0 {
		return len(s) == 0
	}
	head := p[0]
	rest := p[1:]
	switch head {
	case "**":
		if len(rest) == 0 {
			return true
		}
		for i := 0; i <= len(s); i++ {
			if matchSegments(rest, s[i:]) {
				return true
			}
		}
		return false
	case "*":
		if len(s) == 0 {
			return false
		}
		return matchSegments(rest, s[1:])
	default:
		if len(s) == 0 {
			return false
		}
		if !literalSegmentMatch(head, s[0]) {
			return false
		}
		return matchSegments(rest, s[1:])
	}
}

// literalSegmentMatch supports a single '*' wildcard within a segment,
// matching any non-empty sequence of characters not containing '/'.
func literalSegmentMatch(pat, s string) bool {
	if pat == s {
		return true
	}
	if !strings.Contains(pat, "*") {
		return false
	}
	parts := strings.Split(pat, "*")
	// First part must prefix s.
	if !strings.HasPrefix(s, parts[0]) {
		return false
	}
	s = s[len(parts[0]):]
	for i, part := range parts[1 : len(parts)-1] {
		idx := strings.Index(s, part)
		if idx < 0 {
			return false
		}
		_ = i
		s = s[idx+len(part):]
	}
	return strings.HasSuffix(s, parts[len(parts)-1])
}

// Package glob implements the lease pattern matcher. * matches any
// single path segment; ** matches zero or more segments.
package glob

import "strings"

// Match reports whether pattern matches s. Separator is '/'.
func Match(pattern, s string) bool {
	return matchSegments(splitSep(pattern), splitSep(s))
}

// Covers reports whether every concrete target matched by the child
// pattern is also matched by the parent pattern, for this restricted
// glob dialect. Unlike Match (which matches a pattern against a
// concrete string), Covers decides language inclusion between two
// patterns: it treats the child's own wildcards as wildcards rather
// than as literal text.
//
// Covers is sound — it never returns true when the child grants
// authority the parent does not — and conservative: it may return
// false for some pairs that are technically equivalent but
// structurally different (e.g. "a*" vs "a**"). Soundness is the
// security-critical property for §9.4 lease subset checks.
//
// Segment rules:
//   - parent "**" covers any (possibly empty) sequence of child segments;
//   - parent "*" covers exactly one child segment that is itself a single
//     segment (a literal or a single-segment "*"/literal-with-"*"), but
//     not the child's "**";
//   - a parent literal segment covers a child segment only when the child
//     segment is identical or is a concrete literal the parent matches.
func Covers(parent, child string) bool {
	return covers(splitSep(parent), splitSep(child))
}

func covers(p, c []string) bool {
	if len(p) == 0 {
		return len(c) == 0
	}
	head := p[0]
	rest := p[1:]
	switch head {
	case "**":
		for i := 0; i <= len(c); i++ {
			if covers(rest, c[i:]) {
				return true
			}
		}
		return false
	case "*":
		if len(c) == 0 || c[0] == "**" {
			return false
		}
		return covers(rest, c[1:])
	default:
		if len(c) == 0 {
			return false
		}
		ch := c[0]
		if ch == "*" || ch == "**" {
			return false
		}
		if !segmentCovers(head, ch) {
			return false
		}
		return covers(rest, c[1:])
	}
}

// segmentCovers reports whether the parent single-segment pattern
// covers the child single-segment pattern. A child segment that itself
// contains a '*' wildcard is only covered when it is identical to the
// parent segment; otherwise the child must be a concrete literal the
// parent pattern matches.
func segmentCovers(parent, child string) bool {
	if parent == child {
		return true
	}
	if strings.Contains(child, "*") {
		return false
	}
	return literalSegmentMatch(parent, child)
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

// literalSegmentMatch supports '*' wildcards within a single segment.
// Each '*' matches any (possibly empty) sequence of characters that does
// not contain '/', so "foo*" matches both "foobar" and "foo".
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
	for _, part := range parts[1 : len(parts)-1] {
		idx := strings.Index(s, part)
		if idx < 0 {
			return false
		}
		s = s[idx+len(part):]
	}
	return strings.HasSuffix(s, parts[len(parts)-1])
}

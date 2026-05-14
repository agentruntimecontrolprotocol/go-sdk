package arcp

// IntersectFeatures returns the set of feature flags present in both a
// and b, in the order they appear in a. Used at handshake to compute
// the effective negotiated feature set.
func IntersectFeatures(a, b []string) []string {
	if len(a) == 0 || len(b) == 0 {
		return nil
	}
	want := make(map[string]struct{}, len(b))
	for _, x := range b {
		want[x] = struct{}{}
	}
	out := make([]string, 0, len(a))
	seen := make(map[string]struct{}, len(a))
	for _, x := range a {
		if _, ok := want[x]; !ok {
			continue
		}
		if _, dup := seen[x]; dup {
			continue
		}
		seen[x] = struct{}{}
		out = append(out, x)
	}
	return out
}

// HasFeature reports whether name appears in features.
func HasFeature(features []string, name string) bool {
	for _, f := range features {
		if f == name {
			return true
		}
	}
	return false
}

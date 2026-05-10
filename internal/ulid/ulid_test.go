package ulid_test

import (
	"strings"
	"sync"
	"testing"

	"github.com/fizzpop/arcp-go/internal/ulid"
)

func TestNewProducesDistinctValues(t *testing.T) {
	t.Parallel()
	seen := make(map[string]struct{}, 4096)
	for i := 0; i < 4096; i++ {
		v := ulid.New()
		if v == "" {
			t.Fatalf("got empty ulid")
		}
		if len(v) != 26 {
			t.Fatalf("ulid length = %d, want 26: %q", len(v), v)
		}
		if _, dup := seen[v]; dup {
			t.Fatalf("duplicate ulid %q at iter %d", v, i)
		}
		seen[v] = struct{}{}
	}
}

func TestMonotonicityWithinSameMillisecond(t *testing.T) {
	t.Parallel()
	g := ulid.NewGenerator()
	prev := g.New()
	for i := 0; i < 1024; i++ {
		next := g.New()
		if strings.Compare(next, prev) <= 0 {
			t.Fatalf("ulid not monotonic: prev=%q next=%q", prev, next)
		}
		prev = next
	}
}

func TestConcurrentSafe(t *testing.T) {
	t.Parallel()
	g := ulid.NewGenerator()
	const goroutines = 16
	const perG = 1000
	var wg sync.WaitGroup
	results := make(chan string, goroutines*perG)
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < perG; j++ {
				results <- g.New()
			}
		}()
	}
	wg.Wait()
	close(results)
	seen := make(map[string]struct{}, goroutines*perG)
	for v := range results {
		if _, dup := seen[v]; dup {
			t.Fatalf("duplicate %q under concurrent load", v)
		}
		seen[v] = struct{}{}
	}
}

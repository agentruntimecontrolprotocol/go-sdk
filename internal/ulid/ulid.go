package ulid

import (
	crand "crypto/rand"
	"sync"
	"time"

	olu "github.com/oklog/ulid/v2"
)

// Generator produces monotonically-increasing ULIDs. Each Runtime and
// Client owns one; tests may construct their own. ULIDs are
// crypto-strength (entropy from crypto/rand) and lexicographically
// orderable by time.
type Generator struct {
	mu  sync.Mutex
	ent *olu.MonotonicEntropy
}

// NewGenerator returns a fresh Generator seeded from crypto/rand.
func NewGenerator() *Generator {
	return &Generator{ent: olu.Monotonic(crand.Reader, 0)}
}

// New returns a new ULID string. Safe for concurrent use.
func (g *Generator) New() string {
	g.mu.Lock()
	defer g.mu.Unlock()
	return olu.MustNew(olu.Timestamp(time.Now()), g.ent).String()
}

// defaultGenerator is the package-level convenience generator used by
// the package-level New function. Lazy-initialized via sync.OnceValue
// (no init() function and no observable mutable state from outside).
var defaultGenerator = sync.OnceValue(NewGenerator)

// New returns a fresh ULID string from a process-wide default
// generator. Safe for concurrent use.
func New() string { return defaultGenerator().New() }

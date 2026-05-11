// Warehouse DB admin agent. Reads pre-granted; writes prompt operator.
//
// Per-table leases are cached. Inbound lease.revoked invalidates the
// cache so the next statement re-prompts.
package main

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	arcp "github.com/agentruntimecontrolprotocol/go-sdk"
	"github.com/agentruntimecontrolprotocol/go-sdk/messages"
)

var preGranted = []string{
	"public.orders",
	"public.customers",
	"warehouse.fct_revenue_daily",
}

const (
	readLeaseSeconds  = 60 * 60
	writeLeaseSeconds = 5 * 60
)

type cacheKey struct{ table, op string }
type cacheVal struct {
	leaseID   arcp.LeaseID
	expiresAt time.Time
}

type LeaseCache struct {
	mu sync.Mutex
	m  map[cacheKey]cacheVal
}

func (l *LeaseCache) get(k cacheKey) (cacheVal, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	v, ok := l.m[k]
	return v, ok
}
func (l *LeaseCache) put(k cacheKey, v cacheVal) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.m[k] = v
}
func (l *LeaseCache) revoke(id arcp.LeaseID) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for k, v := range l.m {
		if v.leaseID == id {
			delete(l.m, k)
		}
	}
}

func requestLease(ctx context.Context, c *Session,
	permission, table, operation, reason string, seconds int,
) (cacheVal, error) {
	reply, err := c.Request(ctx, &arcp.Envelope{
		Payload: &messages.PermissionRequest{
			Permission:            permission,
			Resource:              "table:" + table,
			Operation:             operation,
			Reason:                reason,
			RequestedLeaseSeconds: seconds,
		},
	})
	if err != nil {
		return cacheVal{}, err
	}
	switch p := reply.Payload.(type) {
	case *messages.LeaseGranted:
		return cacheVal{leaseID: p.Lease.LeaseID, expiresAt: p.Lease.ExpiresAt}, nil
	case *messages.PermissionDeny:
		return cacheVal{}, arcp.NewError(arcp.CodePermissionDenied,
			fmt.Sprintf("%s denied on %s", permission, table))
	default:
		return cacheVal{}, arcp.NewError(arcp.CodeFailedPrecondition,
			"unexpected: "+reply.Type())
	}
}

func authorize(ctx context.Context, c *Session, sql string,
	cache *LeaseCache,
) (string, error) {
	cls := classify(sql)
	if len(cls.Tables) == 0 {
		return "", arcp.NewError(arcp.CodeInvalidArgument, "no table referenced")
	}
	op := cls.Op
	secs := writeLeaseSeconds
	if op == "read" {
		secs = readLeaseSeconds
	}
	for _, t := range cls.Tables {
		k := cacheKey{table: t, op: op}
		if v, ok := cache.get(k); ok && v.expiresAt.After(time.Now()) {
			continue
		}
		v, err := requestLease(ctx, c,
			"db."+op, t, op, fmt.Sprintf("%s on %s: %.80s", op, t, sql), secs)
		if err != nil {
			return op, err
		}
		cache.put(k, v)
	}
	return op, nil
}

// handleInbound wires lease.revoked into the cache so the next call
// re-prompts.
func handleInbound(env arcp.Envelope, cache *LeaseCache) {
	if rv, ok := env.Payload.(*messages.LeaseRevoked); ok {
		cache.revoke(rv.LeaseID)
	}
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	c := openAdmin(ctx) // transport, identity, auth elided
	defer c.Close(ctx)

	cache := &LeaseCache{m: map[cacheKey]cacheVal{}}

	go func() {
		for env := range c.Events(ctx) {
			handleInbound(env, cache)
		}
	}()

	// Pre-grant the broad reads. SELECT on these now runs free.
	for _, t := range preGranted {
		v, err := requestLease(ctx, c,
			"db.read", t, "read", "bootstrap", readLeaseSeconds)
		if err != nil {
			log.Fatal(err)
		}
		cache.put(cacheKey{t, "read"}, v)
	}

	// SELECT — covered by the bootstrap lease.
	if _, err := authorize(ctx, c,
		"SELECT count(*) FROM public.orders WHERE shipped_at::date = "+
			"current_date - 1", cache); err != nil {
		log.Fatal(err)
	}
	// UPDATE — triggers permission.request; operator must approve.
	if _, err := authorize(ctx, c,
		"UPDATE public.orders SET status='refunded' WHERE id=4812",
		cache); err != nil {
		log.Print(err)
	}
}

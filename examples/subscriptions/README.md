# subscriptions

One producing session, three Observer clients, three different sinks.
None of them ever issue a command.

## Before ARCP

Most teams sidecar the agent with a tee: agent emits to stdout, a
shipper tails the log, a second tail re-parses for metrics, a third
process writes to SQLite for replay. Three pipelines diverge over
time, none of them know about each other, and adding a fourth
consumer means another sidecar.

## With ARCP

```go
c := openObserver(ctx)               // observer client; subscriptions: true
defer c.Close(ctx)
subID, _ := subscribe(ctx, c, target, []string{"metric"})
for env := range c.Events(ctx) {
    if inner := unwrapEvent(env); inner != nil {
        sink.Handle(ctx, *inner)
    }
}
```

Three observers, one transport each, filters declared inline. The
producing session never knows they exist.

## ARCP primitives

- Subscriptions, filters, Observer role — RFC §13, §5.
- `since.after_message_id` backfill + the synthetic
  `subscription.backfill_complete` marker — §13.3.
- Standard metrics + trace spans — §17.
- Stream-kind filtering for `kind: thought` redaction — §11.4.

## File tour

- `main.go` — boots three observers in goroutines.
- `sinks.go` — sink stand-ins + `Session` shim that elides the
  pending-request / event-fanout layer.

## Variations

- Replace SQLite with ClickHouse for fleet-wide replay.
- Tee the stdout observer into Slack via a `min_priority: critical`
  filter on the runtime side.
- Add a fourth subscriber that filters `kind: thought` only and
  routes to a stricter access-controlled sink.

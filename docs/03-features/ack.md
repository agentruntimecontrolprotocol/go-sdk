---
title: Event Acknowledgement
sdk: go
spec_sections: [§6.5]
order: 2
kind: feature
---

# Event Acknowledgement

**Negotiation flag:** `ack`. The client emits `session.ack` with the
highest event_seq it has processed; the runtime may free buffered
events and emit `back_pressure` status when lag crosses
`AckLagThreshold`.

```go
client.Options{AutoAckWindow: 32}
```

## See also

- [examples/ack-backpressure](../../examples/ack-backpressure)

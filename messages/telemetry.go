package messages

import (
	"encoding/json"
	"time"

	"github.com/fizzpop/arcp-go"
)

// Wire type names for the telemetry group (RFC §6.2, §17).
const (
	TypeEventEmit = "event.emit"
	TypeLog       = "log"
	TypeMetric    = "metric"
	TypeTraceSpan = "trace.span"
)

// EventEmit wraps a free-form event for delivery (RFC §6.2). Used
// notably for the synthetic `subscription.backfill_complete` boundary
// marker (RFC §13.3).
type EventEmit struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

// ARCPType returns the wire type name.
func (EventEmit) ARCPType() string { return TypeEventEmit }

// SubscriptionBackfillCompleteEventType is the synthetic event type
// emitted via EventEmit at the boundary between historical replay and
// live tail in a subscription (RFC §13.3).
const SubscriptionBackfillCompleteEventType = "subscription.backfill_complete"

// Log is a structured log envelope (RFC §17.2).
type Log struct {
	Level      LogLevel       `json:"level"`
	Message    string         `json:"message"`
	Attributes map[string]any `json:"attributes,omitempty"`
}

// ARCPType returns the wire type name.
func (Log) ARCPType() string { return TypeLog }

// Metric is a single telemetry sample (RFC §17.3).
type Metric struct {
	Name  string         `json:"name"`
	Value float64        `json:"value"`
	Unit  string         `json:"unit,omitempty"`
	Dims  map[string]any `json:"dims,omitempty"`
}

// ARCPType returns the wire type name.
func (Metric) ARCPType() string { return TypeMetric }

// Standard metric names (RFC §17.3.1). Runtimes producing these
// concepts MUST use these names with the indicated units. Non-standard
// metrics MUST be namespaced under arcpx.<vendor>.<name>.
const (
	MetricTokensUsed      = "tokens.used"
	MetricCostUSD         = "cost.usd"
	MetricGPUSeconds      = "gpu.seconds"
	MetricToolInvocations = "tool.invocations"
	MetricLatencyMS       = "latency.ms"
	MetricBytesIn         = "bytes.in"
	MetricBytesOut        = "bytes.out"
	MetricErrorsTotal     = "errors.total"
)

// TraceSpan is a span event (RFC §17.1).
type TraceSpan struct {
	Name       string         `json:"name"`
	StartTime  time.Time      `json:"start_time"`
	EndTime    time.Time      `json:"end_time,omitempty"`
	Attributes map[string]any `json:"attributes,omitempty"`
	Status     string         `json:"status,omitempty"`
	Code       arcp.ErrorCode `json:"code,omitempty"`
}

// ARCPType returns the wire type name.
func (TraceSpan) ARCPType() string { return TypeTraceSpan }

func init() {
	register(TypeEventEmit, func() arcp.MessageType { return &EventEmit{} })
	register(TypeLog, func() arcp.MessageType { return &Log{} })
	register(TypeMetric, func() arcp.MessageType { return &Metric{} })
	register(TypeTraceSpan, func() arcp.MessageType { return &TraceSpan{} })
}

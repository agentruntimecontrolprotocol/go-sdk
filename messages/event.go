package messages

import (
	"encoding/json"
	"fmt"

	arcp "github.com/agentruntimecontrolprotocol/go-sdk"
)

// LogBody is the body of a "log" event. Fields carries the structured
// slog.Attr key/values passed to JobContext.Log.
type LogBody struct {
	Level   string         `json:"level"`
	Message string         `json:"message"`
	Fields  map[string]any `json:"fields,omitempty"`
}

// ThoughtBody is the body of a "thought" event.
type ThoughtBody struct {
	Text string `json:"text"`
}

// ToolCallBody is the body of a "tool_call" event.
type ToolCallBody struct {
	Tool   string          `json:"tool"`
	Args   json.RawMessage `json:"args,omitempty"`
	CallID string          `json:"call_id"`
}

// ToolResultBody is the body of a "tool_result" event. Exactly one of
// Result and Error must be set.
type ToolResultBody struct {
	CallID string          `json:"call_id"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *ToolError      `json:"error,omitempty"`
}

// Validate enforces the spec rule that result and error are mutually
// exclusive.
func (b ToolResultBody) Validate() error {
	hasResult := len(b.Result) > 0 && string(b.Result) != "null"
	hasError := b.Error != nil
	if hasResult == hasError {
		return arcp.ErrInvalidRequest.WithMessage("tool_result body must carry exactly one of result or error")
	}
	return nil
}

// ToolError describes a structured tool_result error body.
type ToolError struct {
	Code      arcp.ErrorCode `json:"code"`
	Message   string         `json:"message,omitempty"`
	Retryable bool           `json:"retryable"`
	Details   map[string]any `json:"details,omitempty"`
}

// StatusBody is the body of a "status" event.
type StatusBody struct {
	Phase   string         `json:"phase"`
	Message string         `json:"message,omitempty"`
	Details map[string]any `json:"details,omitempty"`
}

// MetricBody is the body of a "metric" event.
type MetricBody struct {
	Name       string            `json:"name"`
	Value      float64           `json:"value"`
	Unit       string            `json:"unit,omitempty"`
	Dimensions map[string]string `json:"dimensions,omitempty"`
}

// ArtifactRefBody is the body of an "artifact_ref" event.
type ArtifactRefBody struct {
	URI         string `json:"uri"`
	ContentType string `json:"content_type"`
	ByteSize    uint64 `json:"byte_size,omitempty"`
	SHA256      string `json:"sha256,omitempty"`
}

// DelegateBody is the body of a "delegate" event.
type DelegateBody struct {
	ChildJobID string     `json:"child_job_id"`
	Agent      string     `json:"agent"`
	Lease      arcp.Lease `json:"lease,omitempty"`
}

// ProgressBody is the body of a "progress" event.
type ProgressBody struct {
	Current uint64 `json:"current"`
	Total   uint64 `json:"total,omitempty"`
	Units   string `json:"units,omitempty"`
	Message string `json:"message,omitempty"`
}

// Validate enforces non-negative current. Because Current is uint64,
// the only constraint is the optional current <= total when total is
// present and nonzero.
func (b ProgressBody) Validate() error {
	if b.Total != 0 && b.Current > b.Total {
		return arcp.ErrInvalidRequest.WithMessage("progress current must be <= total when total is set")
	}
	return nil
}

// ResultChunkBody is the body of a "result_chunk" event.
type ResultChunkBody struct {
	ResultID string `json:"result_id"`
	ChunkSeq uint64 `json:"chunk_seq"`
	Data     string `json:"data"`
	Encoding string `json:"encoding"`
	More     bool   `json:"more"`
}

// Validate enforces the encoding values defined by the spec.
func (b ResultChunkBody) Validate() error {
	if b.ResultID == "" {
		return arcp.ErrInvalidRequest.WithMessage("result_chunk result_id is required")
	}
	switch b.Encoding {
	case "utf8", "base64":
	default:
		return arcp.ErrInvalidRequest.WithMessage("result_chunk encoding must be utf8 or base64")
	}
	return nil
}

// DecodeEventBody unmarshals e.Body into the kind-specific struct.
// Unknown kinds return the raw json.RawMessage unchanged.
func DecodeEventBody(e *JobEvent) (any, error) {
	switch e.Kind {
	case KindLog:
		var b LogBody
		return &b, json.Unmarshal(e.Body, &b)
	case KindThought:
		var b ThoughtBody
		return &b, json.Unmarshal(e.Body, &b)
	case KindToolCall:
		var b ToolCallBody
		return &b, json.Unmarshal(e.Body, &b)
	case KindToolResult:
		var b ToolResultBody
		if err := json.Unmarshal(e.Body, &b); err != nil {
			return nil, err
		}
		return &b, b.Validate()
	case KindStatus:
		var b StatusBody
		return &b, json.Unmarshal(e.Body, &b)
	case KindMetric:
		var b MetricBody
		return &b, json.Unmarshal(e.Body, &b)
	case KindArtifactRef:
		var b ArtifactRefBody
		return &b, json.Unmarshal(e.Body, &b)
	case KindDelegate:
		var b DelegateBody
		return &b, json.Unmarshal(e.Body, &b)
	case KindProgress:
		var b ProgressBody
		if err := json.Unmarshal(e.Body, &b); err != nil {
			return nil, err
		}
		return &b, b.Validate()
	case KindResultChunk:
		var b ResultChunkBody
		if err := json.Unmarshal(e.Body, &b); err != nil {
			return nil, err
		}
		return &b, b.Validate()
	default:
		// Vendor / unknown kinds: return the raw bytes for caller decoding.
		return e.Body, nil
	}
}

// NewEventBody wraps a typed body value into a JobEvent body field.
func NewEventBody(v any) (json.RawMessage, error) {
	body, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("event body marshal: %w", err)
	}
	return body, nil
}

// ARCP runtime fronting an MCP server (RFC §20).
//
// MCP describes capabilities; ARCP operationalizes them. This bridge
// translates inbound ARCP `tool.invoke` envelopes into MCP `call_tool`
// calls against an upstream MCP server, and emits the ARCP job
// lifecycle back to the calling client.
//
//	ARCP client ──tool.invoke──> bridge ──call_tool──> MCP server
//	ARCP client <─job.{accepted,started,completed,failed}─ bridge
//
// The `mcp` symbols below stand in for the upstream Go MCP client SDK
// (e.g. github.com/modelcontextprotocol/go-sdk). They are stubbed in
// upstream.go so this example compiles standalone; swap them for the
// real import when vendoring.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	arcp "github.com/agentruntimecontrolprotocol/go-sdk"
	"github.com/agentruntimecontrolprotocol/go-sdk/messages"
)

// SendEnvelope is the runtime's outbound channel.
type SendEnvelope func(context.Context, arcp.Envelope) error

// advertiseFromMCP turns MCP `tools/list` into namespaced ARCP
// capability extensions. Each upstream tool surfaces as
// `arcpx.mcp.tool.<name>.v1` so clients can negotiate which tools
// they require at session open.
func advertiseFromMCP(ctx context.Context, c *MCPClient) ([]string, error) {
	listed, err := c.ListTools(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(listed))
	for _, t := range listed {
		out = append(out, "arcpx.mcp.tool."+t.Name+".v1")
	}
	return out, nil
}

// callViaMCP translates ARCP tool.invoke.payload into MCP call_tool.
// MCP returns a list of typed content blocks; we flatten to JSON for
// the ARCP tool.result / job.completed payload. MCP errors map to
// canonical ARCP error codes.
func callViaMCP(ctx context.Context, c *MCPClient,
	tool string, args map[string]any,
) (json.RawMessage, error) {
	res, err := c.CallTool(ctx, tool, args)
	if err != nil {
		return nil, arcp.NewError(arcp.CodeInternal, err.Error())
	}
	if res.IsError {
		// MCP doesn't carry a typed error code; FAILED_PRECONDITION is
		// the right canonical mapping for "tool ran, said no".
		return nil, arcp.NewError(arcp.CodeFailedPrecondition, joinContent(res.Content))
	}
	return json.Marshal(map[string]any{"content": res.Content})
}

// handleInvoke is the single inbound tool.invoke → MCP call → ARCP job
// lifecycle reactor.
func handleInvoke(ctx context.Context, send SendEnvelope,
	c *MCPClient, request arcp.Envelope,
) {
	jid := arcp.NewJobID()
	_ = send(ctx, arcp.Envelope{
		ID: arcp.NewMessageID(), CorrelationID: request.ID, JobID: jid,
		Payload: &messages.JobAccepted{JobID: jid},
	})
	_ = send(ctx, arcp.Envelope{
		ID: arcp.NewMessageID(), JobID: jid,
		Payload: &messages.JobStarted{StartedAt: time.Now()},
	})

	inv := request.Payload.(*messages.ToolInvoke)
	value, err := callViaMCP(ctx, c, inv.Tool, inv.Arguments)
	if err != nil {
		ae, _ := err.(*arcp.Error)
		if ae == nil {
			ae = arcp.NewError(arcp.CodeInternal, err.Error())
		}
		_ = send(ctx, arcp.Envelope{
			ID: arcp.NewMessageID(), JobID: jid,
			Payload: &messages.JobFailed{
				ErrorPayload: messages.ErrorPayload{
					Code: ae.Code, Message: ae.Message,
				}}})
		return
	}
	_ = send(ctx, arcp.Envelope{
		ID: arcp.NewMessageID(), JobID: jid,
		Payload: &messages.JobCompleted{Value: value},
	})
}

// runBridge wires one MCP session as the upstream for one ARCP runtime.
func runBridge(ctx context.Context, send SendEnvelope, inbound <-chan arcp.Envelope) error {
	c, err := dialUpstream(ctx)
	if err != nil {
		return err
	}
	defer c.Close()

	exts, err := advertiseFromMCP(ctx, c)
	if err != nil {
		return err
	}
	// In production this list would feed Capabilities.Extensions at
	// the runtime's session.accepted so clients negotiate exactly the
	// MCP tools they expect to use.
	fmt.Printf("bridged: %v\n", exts)

	for env := range inbound {
		if _, ok := env.Payload.(*messages.ToolInvoke); ok {
			go handleInvoke(ctx, send, c, env)
		}
	}
	return nil
}

func main() {
	// Production version: instantiate an arcp Runtime, point its
	// tool-invoke handler at handleInvoke, and let the WebSocket
	// transport carry inbound envelopes from real ARCP clients. We
	// elide the runtime wiring (symmetric with examples in
	// arcp/runtime) so this file stays focused on the §20 translation
	// between protocols.
	var send SendEnvelope            // bound to the runtime's outbound channel
	var inbound <-chan arcp.Envelope // inbound queue from the runtime
	if err := runBridge(context.Background(), send, inbound); err != nil {
		fmt.Println("bridge:", err)
	}
}

// Upstream MCP server invocation.
//
// The MCP* types and dialUpstream below stand in for the upstream Go
// MCP client SDK. As of this writing the canonical Go SDK lives at
// github.com/modelcontextprotocol/go-sdk; until vendored, treat this
// file as the bridge between the example's protocol code and whatever
// MCP client implementation you wire in.
//
// Real version parameterizes command, args, env via your config layer.
// Reference servers from the modelcontextprotocol org publish under
// `mcp-server-*` (filesystem, git, postgres, slack, ...).
package main

import "context"

// MCPTool mirrors the upstream SDK's tool descriptor.
type MCPTool struct{ Name string }

// MCPContent mirrors a typed content block.
type MCPContent struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// MCPCallResult mirrors the upstream call_tool result envelope.
type MCPCallResult struct {
	Content []MCPContent
	IsError bool
}

// MCPClient is the upstream MCP session handle.
type MCPClient struct{}

func (*MCPClient) ListTools(context.Context) ([]MCPTool, error) {
	panic("not implemented: MCPClient.ListTools")
}

func (*MCPClient) CallTool(context.Context, string, map[string]any) (*MCPCallResult, error) {
	panic("not implemented: MCPClient.CallTool")
}

func (*MCPClient) Close() error { return nil }

// dialUpstream spins the MCP child process and returns a session.
// Real version: exec.Command("uvx", "mcp-server-filesystem", "/srv/data")
// piped through stdio, then mcp.NewClient(...).Connect(ctx, ...).
func dialUpstream(context.Context) (*MCPClient, error) {
	panic("not implemented: dialUpstream")
}

// joinContent flattens MCP content blocks to a single text string for
// error messages.
func joinContent(blocks []MCPContent) string {
	out := ""
	for i, b := range blocks {
		if i > 0 {
			out += "\n"
		}
		out += b.Text
	}
	return out
}

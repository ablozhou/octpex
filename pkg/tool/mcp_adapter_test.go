package tool

import (
	"context"
	"encoding/json"
	"octpex/pkg/mcp"
	"testing"
)

type mockMCPClient struct {
	tools []mcp.Tool
}

func (m *mockMCPClient) Initialize(ctx context.Context) error {
	return nil
}

func (m *mockMCPClient) ListTools(ctx context.Context) ([]mcp.Tool, error) {
	return m.tools, nil
}

func (m *mockMCPClient) CallTool(ctx context.Context, name string, args map[string]interface{}) (*mcp.CallToolResult, error) {
	res := &mcp.CallToolResult{}
	res.Content = append(res.Content, struct {
		Type string `json:"type"`
		Text string `json:"text,omitempty"`
	}{
		Type: "text",
		Text: "mcp output for " + name,
	})
	return res, nil
}

func (m *mockMCPClient) Close() error {
	return nil
}

func TestDynamicMCPBox_RegisterMCPServer(t *testing.T) {
	box := NewDynamicMCPBox()
	mockClient := &mockMCPClient{
		tools: []mcp.Tool{
			{
				Name:        "external_fetch",
				Description: "Fetches remote data",
				InputSchema: json.RawMessage(`{"type":"object"}`),
			},
		},
	}

	ctx := context.Background()
	err := box.RegisterMCPServer(ctx, mockClient)
	if err != nil {
		t.Fatalf("RegisterMCPServer failed: %v", err)
	}

	// Verify tool presence in box
	schema, err := box.InspectTool(ctx, "external_fetch")
	if err != nil {
		t.Fatalf("InspectTool failed: %v", err)
	}
	if schema.Name != "external_fetch" {
		t.Fatalf("expected tool name external_fetch, got %s", schema.Name)
	}

	// Invoke via DynamicMCPBox
	result, err := box.InvokeTool(ctx, "external_fetch", map[string]any{"url": "https://example.com"})
	if err != nil {
		t.Fatalf("InvokeTool failed: %v", err)
	}
	if !result.Success || result.Output != "mcp output for external_fetch" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

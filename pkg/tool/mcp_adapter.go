package tool

import (
	"context"
	"fmt"
	"octpex/pkg/mcp"
	"time"
)

// RegisterMCPServer connects to an MCP server, queries its tools, and registers them into the DynamicMCPBox.
func (b *DynamicMCPBox) RegisterMCPServer(ctx context.Context, client mcp.Client) error {
	tools, err := client.ListTools(ctx)
	if err != nil {
		return fmt.Errorf("failed to discover tools from MCP client: %w", err)
	}

	for _, t := range tools {
		toolName := t.Name
		schema := &ToolSchema{
			Name:        toolName,
			Description: t.Description,
			Parameters:  map[string]any{"raw_schema": string(t.InputSchema)},
			IsDynamic:   false,
		}

		capturedName := toolName
		handler := func(ctx context.Context, params map[string]any) (*ToolResult, error) {
			start := time.Now()
			res, err := client.CallTool(ctx, capturedName, params)
			dur := time.Since(start).Milliseconds()
			if err != nil {
				return &ToolResult{
					ToolName:   capturedName,
					Success:    false,
					Error:      err.Error(),
					DurationMs: dur,
				}, nil
			}

			output := ""
			for _, c := range res.Content {
				output += c.Text
			}

			return &ToolResult{
				ToolName:   capturedName,
				Success:    !res.IsError,
				Output:     output,
				DurationMs: dur,
			}, nil
		}

		b.RegisterTool(schema, handler)
	}
	return nil
}

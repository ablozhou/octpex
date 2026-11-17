package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"testing"
	"time"
)

func TestStreamClient_Lifecycle(t *testing.T) {
	// clientWriter -> serverReader
	serverReader, clientWriter := io.Pipe()
	// serverWriter -> clientReader
	clientReader, serverWriter := io.Pipe()

	// Mock MCP Server
	go func() {
		scanner := bufio.NewScanner(serverReader)
		for scanner.Scan() {
			var req JSONRPCRequest
			if err := json.Unmarshal(scanner.Bytes(), &req); err != nil {
				continue
			}

			var resp JSONRPCResponse
			resp.JSONRPC = "2.0"
			resp.ID = req.ID

			switch req.Method {
			case "initialize":
				resp.Result = json.RawMessage(`{"protocolVersion":"2024-11-05","serverInfo":{"name":"mock-fs","version":"1.0"}}`)
			case "tools/list":
				resp.Result = json.RawMessage(`{"tools":[{"name":"read_file","description":"Read content of a file","inputSchema":{}}]}`)
			case "tools/call":
				resp.Result = json.RawMessage(`{"content":[{"type":"text","text":"file contents mock"}],"isError":false}`)
			default:
				resp.Error = &JSONRPCError{Code: -32601, Message: "Method not found"}
			}

			payload, _ := json.Marshal(resp)
			payload = append(payload, '\n')
			_, _ = serverWriter.Write(payload)
		}
	}()

	client := NewStreamClient(clientReader, clientWriter)
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// 1. Initialize
	if err := client.Initialize(ctx); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	// 2. List tools
	tools, err := client.ListTools(ctx)
	if err != nil {
		t.Fatalf("ListTools failed: %v", err)
	}
	if len(tools) != 1 || tools[0].Name != "read_file" {
		t.Fatalf("unexpected tools returned: %+v", tools)
	}

	// 3. Call tool
	res, err := client.CallTool(ctx, "read_file", map[string]interface{}{"path": "test.txt"})
	if err != nil {
		t.Fatalf("CallTool failed: %v", err)
	}
	if len(res.Content) == 0 || res.Content[0].Text != "file contents mock" {
		t.Fatalf("unexpected tool call result: %+v", res)
	}
}

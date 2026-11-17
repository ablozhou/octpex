package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"sync/atomic"
)

// JSONRPCRequest represents a JSON-RPC 2.0 request payload.
type JSONRPCRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int64       `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

// JSONRPCResponse represents a JSON-RPC 2.0 response payload.
type JSONRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int64           `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *JSONRPCError   `json:"error,omitempty"`
}

// JSONRPCError represents a standard JSON-RPC error.
type JSONRPCError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func (e *JSONRPCError) Error() string {
	return fmt.Sprintf("MCP Error %d: %s", e.Code, e.Message)
}

// Tool represents a registered capability declared by an MCP server.
type Tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
}

// ListToolsResult represents the result payload for tools/list.
type ListToolsResult struct {
	Tools []Tool `json:"tools"`
}

// CallToolParams represents parameters for tools/call.
type CallToolParams struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

// CallToolResult represents the execution result of tools/call.
type CallToolResult struct {
	Content []struct {
		Type string `json:"type"` // "text", "image", "resource"
		Text string `json:"text,omitempty"`
	} `json:"content"`
	IsError bool `json:"isError,omitempty"`
}

// Client defines the contract for communicating with external MCP servers.
type Client interface {
	Initialize(ctx context.Context) error
	ListTools(ctx context.Context) ([]Tool, error)
	CallTool(ctx context.Context, name string, args map[string]interface{}) (*CallToolResult, error)
	Close() error
}

// StdioMCPClient runs an external MCP server process over standard input/output streams.
type StdioMCPClient struct {
	command    string
	args       []string
	cmd        *exec.Cmd
	stdin      io.WriteCloser
	stdout     *bufio.Reader
	seqID      int64
	mu         sync.Mutex
	pending    map[int64]chan *JSONRPCResponse
	stopCh     chan struct{}
	closeOnce  sync.Once
}

// NewStdioMCPClient creates a client configured to spawn an MCP server subprocess.
func NewStdioMCPClient(command string, args ...string) *StdioMCPClient {
	return &StdioMCPClient{
		command: command,
		args:    args,
		pending: make(map[int64]chan *JSONRPCResponse),
		stopCh:  make(chan struct{}),
	}
}

// Start launches the subprocess and begins reading incoming JSON-RPC lines.
func (c *StdioMCPClient) Start() error {
	c.cmd = exec.Command(c.command, c.args...)
	stdin, err := c.cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to open stdin pipe: %w", err)
	}
	stdout, err := c.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to open stdout pipe: %w", err)
	}

	if err := c.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start MCP subprocess (%s): %w", c.command, err)
	}

	c.stdin = stdin
	c.stdout = bufio.NewReader(stdout)

	go c.readLoop()
	return nil
}

func (c *StdioMCPClient) readLoop() {
	for {
		select {
		case <-c.stopCh:
			return
		default:
			line, err := c.stdout.ReadBytes('\n')
			if err != nil {
				return
			}
			var resp JSONRPCResponse
			if err := json.Unmarshal(line, &resp); err != nil {
				continue
			}

			c.mu.Lock()
			ch, ok := c.pending[resp.ID]
			if ok {
				delete(c.pending, resp.ID)
			}
			c.mu.Unlock()

			if ok && ch != nil {
				ch <- &resp
			}
		}
	}
}

// Send sends a request and awaits the corresponding response matching req.ID.
func (c *StdioMCPClient) Send(ctx context.Context, method string, params interface{}) (*JSONRPCResponse, error) {
	id := atomic.AddInt64(&c.seqID, 1)
	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}

	payload, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}
	payload = append(payload, '\n')

	ch := make(chan *JSONRPCResponse, 1)
	c.mu.Lock()
	c.pending[id] = ch
	c.mu.Unlock()

	c.mu.Lock()
	if c.stdin == nil {
		c.mu.Unlock()
		return nil, fmt.Errorf("client stdin is not open")
	}
	_, err = c.stdin.Write(payload)
	c.mu.Unlock()

	if err != nil {
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		return nil, fmt.Errorf("failed to write to MCP stdin: %w", err)
	}

	select {
	case <-ctx.Done():
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		return nil, ctx.Err()
	case resp := <-ch:
		if resp.Error != nil {
			return nil, resp.Error
		}
		return resp, nil
	}
}

// Initialize performs the standard MCP handshake.
func (c *StdioMCPClient) Initialize(ctx context.Context) error {
	params := map[string]interface{}{
		"protocolVersion": "2024-11-05",
		"clientInfo": map[string]string{
			"name":    "octpex-arm",
			"version": "1.0.0",
		},
		"capabilities": map[string]interface{}{
			"tools": map[string]interface{}{},
		},
	}
	_, err := c.Send(ctx, "initialize", params)
	return err
}

// ListTools retrieves all tools available on the MCP server.
func (c *StdioMCPClient) ListTools(ctx context.Context) ([]Tool, error) {
	resp, err := c.Send(ctx, "tools/list", map[string]interface{}{})
	if err != nil {
		return nil, err
	}

	var res ListToolsResult
	if err := json.Unmarshal(resp.Result, &res); err != nil {
		return nil, fmt.Errorf("failed to unmarshal tools/list result: %w", err)
	}
	return res.Tools, nil
}

// CallTool executes a tool on the MCP server.
func (c *StdioMCPClient) CallTool(ctx context.Context, name string, args map[string]interface{}) (*CallToolResult, error) {
	params := CallToolParams{
		Name:      name,
		Arguments: args,
	}
	resp, err := c.Send(ctx, "tools/call", params)
	if err != nil {
		return nil, err
	}

	var res CallToolResult
	if err := json.Unmarshal(resp.Result, &res); err != nil {
		return nil, fmt.Errorf("failed to unmarshal tools/call result: %w", err)
	}
	return &res, nil
}

// Close gracefully terminates the subprocess and cleanup channels.
func (c *StdioMCPClient) Close() error {
	var err error
	c.closeOnce.Do(func() {
		close(c.stopCh)
		if c.stdin != nil {
			c.stdin.Close()
		}
		if c.cmd != nil && c.cmd.Process != nil {
			_ = c.cmd.Process.Kill()
			err = c.cmd.Wait()
		}
	})
	return err
}

// StreamClient allows direct in-memory stream coupling (useful for unit testing or direct pipe).
type StreamClient struct {
	reader  *bufio.Reader
	writer  io.Writer
	seqID   int64
	mu      sync.Mutex
	pending map[int64]chan *JSONRPCResponse
	stopCh  chan struct{}
}

// NewStreamClient creates an MCP client over arbitrary io.Reader and io.Writer.
func NewStreamClient(r io.Reader, w io.Writer) *StreamClient {
	c := &StreamClient{
		reader:  bufio.NewReader(r),
		writer:  w,
		pending: make(map[int64]chan *JSONRPCResponse),
		stopCh:  make(chan struct{}),
	}
	go c.readLoop()
	return c
}

func (c *StreamClient) readLoop() {
	for {
		select {
		case <-c.stopCh:
			return
		default:
			line, err := c.reader.ReadBytes('\n')
			if err != nil {
				return
			}
			var resp JSONRPCResponse
			if err := json.Unmarshal(line, &resp); err != nil {
				continue
			}

			c.mu.Lock()
			ch, ok := c.pending[resp.ID]
			if ok {
				delete(c.pending, resp.ID)
			}
			c.mu.Unlock()

			if ok && ch != nil {
				ch <- &resp
			}
		}
	}
}

// Send sends a request over the stream.
func (c *StreamClient) Send(ctx context.Context, method string, params interface{}) (*JSONRPCResponse, error) {
	id := atomic.AddInt64(&c.seqID, 1)
	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}

	payload, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}
	payload = append(payload, '\n')

	ch := make(chan *JSONRPCResponse, 1)
	c.mu.Lock()
	c.pending[id] = ch
	c.mu.Unlock()

	c.mu.Lock()
	_, err = c.writer.Write(payload)
	c.mu.Unlock()

	if err != nil {
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		return nil, fmt.Errorf("failed to write to stream: %w", err)
	}

	select {
	case <-ctx.Done():
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		return nil, ctx.Err()
	case resp := <-ch:
		if resp.Error != nil {
			return nil, resp.Error
		}
		return resp, nil
	}
}

func (c *StreamClient) Initialize(ctx context.Context) error {
	_, err := c.Send(ctx, "initialize", map[string]interface{}{
		"protocolVersion": "2024-11-05",
	})
	return err
}

func (c *StreamClient) ListTools(ctx context.Context) ([]Tool, error) {
	resp, err := c.Send(ctx, "tools/list", map[string]interface{}{})
	if err != nil {
		return nil, err
	}
	var res ListToolsResult
	if err := json.Unmarshal(resp.Result, &res); err != nil {
		return nil, err
	}
	return res.Tools, nil
}

func (c *StreamClient) CallTool(ctx context.Context, name string, args map[string]interface{}) (*CallToolResult, error) {
	resp, err := c.Send(ctx, "tools/call", CallToolParams{Name: name, Arguments: args})
	if err != nil {
		return nil, err
	}
	var res CallToolResult
	if err := json.Unmarshal(resp.Result, &res); err != nil {
		return nil, err
	}
	return &res, nil
}

func (c *StreamClient) Close() error {
	close(c.stopCh)
	return nil
}

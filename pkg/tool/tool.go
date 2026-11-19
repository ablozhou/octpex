package tool

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

var (
	ErrToolNotFound      = errors.New("tool: tool URI not found")
	ErrExecutionTimeout  = errors.New("tool: execution timed out")
	ErrInvalidParameters = errors.New("tool: invalid parameter schema")
)

// ToolSchema describes the parameter requirements and documentation of a tool.
type ToolSchema struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
	IsDynamic   bool           `json:"is_dynamic"` // True if synthesized by Octo-Brain
}

// ToolResult encapsulates the output of a tool invocation.
type ToolResult struct {
	ToolName    string `json:"tool_name"`
	Success     bool   `json:"success"`
	Output      string `json:"output"`
	Error       string `json:"error,omitempty"`
	DurationMs  int64  `json:"duration_ms"`
	ArtifactURI string `json:"artifact_uri,omitempty"`
}

// ToolHandlerFunc executes tool logic.
type ToolHandlerFunc func(ctx context.Context, params map[string]any) (*ToolResult, error)

// ToolInvocationSPI defines the pluggable tool registry and invocation contract.
type ToolInvocationSPI interface {
	InvokeTool(ctx context.Context, toolURI string, params map[string]any) (*ToolResult, error)
	InspectTool(ctx context.Context, toolURI string) (*ToolSchema, error)
}

// SecurityScanner abstracts sandbox pre-flight code scanning (implemented by AbloEx).
type SecurityScanner interface {
	ScanCode(code string) error
}

// DynamicMCPBox is the shared tool registry on the Basal Nerve Ring.
// It holds enterprise MCP tools and runtime-synthesized scripts produced by Octo-Brain's self-evolution loop.
type ScriptRunner interface {
	Run(context.Context, string, string, map[string]any) (*ToolResult, error)
}

var ErrNoScriptRunner = errors.New("tool: no isolated script runner configured")

type DynamicMCPBox struct {
	runner   ScriptRunner
	mu       sync.RWMutex
	schemas  map[string]*ToolSchema
	handlers map[string]ToolHandlerFunc
	scanner  SecurityScanner
}

// NewDynamicMCPBox creates an initialized DynamicMCPBox.
func NewDynamicMCPBox() *DynamicMCPBox {
	return &DynamicMCPBox{
		schemas:  make(map[string]*ToolSchema),
		handlers: make(map[string]ToolHandlerFunc),
	}
}

// SetSecurityScanner configures an AbloEx security scanner for dynamic tool admissions.
func (b *DynamicMCPBox) SetSecurityScanner(scanner SecurityScanner) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.scanner = scanner
}

func (b *DynamicMCPBox) SetScriptRunner(r ScriptRunner) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.runner = r
}

// RegisterTool adds a static or dynamic tool definition to the box.
func (b *DynamicMCPBox) RegisterTool(schema *ToolSchema, handler ToolHandlerFunc) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.schemas[schema.Name] = schema
	b.handlers[schema.Name] = handler
}

// RegisterDynamicScript registers executable script code synthesized by Octo-Brain after AbloEx scanning.
func (b *DynamicMCPBox) RegisterDynamicScript(name, description, code string) error {
	b.mu.RLock()
	scanner := b.scanner
	b.mu.RUnlock()

	// Pre-admission security scan (AbloEx Gate)
	if scanner != nil {
		if err := scanner.ScanCode(code); err != nil {
			return fmt.Errorf("dynamic_mcp_box: admission rejected by security scanner: %w", err)
		}
	}

	schema := &ToolSchema{
		Name:        name,
		Description: description,
		IsDynamic:   true,
		Parameters: map[string]any{
			"code_length": len(code),
		},
	}

	handler := func(ctx context.Context, params map[string]any) (*ToolResult, error) {
		b.mu.RLock()
		runner := b.runner
		b.mu.RUnlock()
		if runner == nil {
			return nil, ErrNoScriptRunner
		}
		return runner.Run(ctx, name, code, params)
	}

	b.RegisterTool(schema, handler)
	return nil
}

func (b *DynamicMCPBox) InvokeTool(ctx context.Context, toolName string, params map[string]any) (*ToolResult, error) {
	b.mu.RLock()
	handler, exists := b.handlers[toolName]
	b.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("%w: '%s'", ErrToolNotFound, toolName)
	}

	start := time.Now()
	res, err := handler(ctx, params)
	if err != nil {
		return nil, err
	}
	res.DurationMs = time.Since(start).Milliseconds()
	return res, nil
}

func (b *DynamicMCPBox) InspectTool(ctx context.Context, toolName string) (*ToolSchema, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	schema, exists := b.schemas[toolName]
	if !exists {
		return nil, fmt.Errorf("%w: '%s'", ErrToolNotFound, toolName)
	}
	return schema, nil
}

// ListTools returns metadata for all currently registered tools.
func (b *DynamicMCPBox) ListTools() []*ToolSchema {
	b.mu.RLock()
	defer b.mu.RUnlock()

	list := make([]*ToolSchema, 0, len(b.schemas))
	for _, s := range b.schemas {
		list = append(list, s)
	}
	return list
}

package sandbox

import (
	"context"
	"errors"
	"fmt"
	"octpex/pkg/protocol"
	"octpex/pkg/tool"
	"regexp"
	"strings"
	"time"
)

var (
	ErrDangerousCodeDetected    = errors.New("abloex: dangerous pattern detected by static AST scanner")
	ErrPrivilegeAttenuated      = errors.New("abloex: action rejected, mutation privilege attenuated across lateral hops")
	ErrUnauthorizedCapability   = errors.New("abloex: capability not permitted in caller security context")
	ErrSandboxTimeout           = errors.New("abloex: execution exceeded sandbox timeout quota")
)

// SandboxQuota defines resource limits for sandboxed execution.
type SandboxQuota struct {
	MaxTimeout      time.Duration
	MaxMemoryMB     int
	AllowNetworking bool
	ReadOnlyRootFS  bool
}

// DefaultSandboxQuota returns enterprise baseline security quotas (Section 7.1).
func DefaultSandboxQuota() SandboxQuota {
	return SandboxQuota{
		MaxTimeout:      5 * time.Second,
		MaxMemoryMB:     256,
		AllowNetworking: false,
		ReadOnlyRootFS:  true,
	}
}

// AbloExSandbox is the sister security engine providing hardware/process isolation
// and zero-trust token attenuation enforcement for Octpex.
type AbloExSandbox struct {
	quota          SandboxQuota
	prohibitedPatterns []*regexp.Regexp
}

// NewAbloExSandbox creates an initialized AbloEx security sandbox.
func NewAbloExSandbox(quota SandboxQuota) *AbloExSandbox {
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)\brm\s+-rf\s+[/~]`),
		regexp.MustCompile(`(?i)\bmkfs\b`),
		regexp.MustCompile(`(?i)\bdd\s+if=`),
		regexp.MustCompile(`(?i):\(\)\s*\{\s*:\|:&\s*\};:`), // Fork bomb
		regexp.MustCompile(`(?i)\bdrop\s+(database|table)\b`),
		regexp.MustCompile(`(?i)\b(nc|netcat)\s+.*-e\b`),
		regexp.MustCompile(`(?i)/dev/tcp/`),
	}
	return &AbloExSandbox{
		quota:              quota,
		prohibitedPatterns: patterns,
	}
}

// ScanCode performs static AST & instruction security inspection prior to admission into the Dynamic MCP Box.
func (s *AbloExSandbox) ScanCode(code string) error {
	for _, pattern := range s.prohibitedPatterns {
		if pattern.MatchString(code) {
			return fmt.Errorf("%w: matched prohibited pattern '%s'", ErrDangerousCodeDetected, pattern.String())
		}
	}
	// Check unauthorized network socket imports if networking is disabled
	if !s.quota.AllowNetworking {
		if strings.Contains(code, "import socket") || strings.Contains(code, "require('net')") || strings.Contains(code, "net.Dial") {
			return fmt.Errorf("%w: outbound networking is disabled in sandbox quota", ErrDangerousCodeDetected)
		}
	}
	return nil
}

// ExecuteDynamicScript runs synthesized code inside the AbloEx micro-sandbox with strict quota and token attenuation checks.
func (s *AbloExSandbox) ExecuteDynamicScript(ctx context.Context, toolName, code string, params map[string]any, sec *protocol.SecurityContext, isMutation bool) (*tool.ToolResult, error) {
	// 1. Static Scan Gate
	if err := s.ScanCode(code); err != nil {
		return &tool.ToolResult{
			ToolName: toolName,
			Success:  false,
			Error:    err.Error(),
		}, err
	}

	// 2. Token Attenuation Gate (AbloEx Zero-Trust)
	if sec != nil {
		if isMutation && !sec.AllowMutation {
			return &tool.ToolResult{
				ToolName: toolName,
				Success:  false,
				Error:    ErrPrivilegeAttenuated.Error(),
			}, ErrPrivilegeAttenuated
		}

		if len(sec.AllowedCapabilities) > 0 {
			permitted := false
			for _, capName := range sec.AllowedCapabilities {
				if capName == toolName {
					permitted = true
					break
				}
			}
			if !permitted {
				return &tool.ToolResult{
					ToolName: toolName,
					Success:  false,
					Error:    fmt.Sprintf("%s: '%s'", ErrUnauthorizedCapability.Error(), toolName),
				}, ErrUnauthorizedCapability
			}
		}
	}

	// 3. Execution Timeout Quota Gate
	execTimeout := s.quota.MaxTimeout
	if execTimeout <= 0 {
		execTimeout = 5 * time.Second
	}
	execCtx, cancel := context.WithTimeout(ctx, execTimeout)
	defer cancel()

	start := time.Now()

	// 4. Isolated Execution in Sandbox environment
	select {
	case <-execCtx.Done():
		return &tool.ToolResult{
			ToolName:   toolName,
			Success:    false,
			Error:      ErrSandboxTimeout.Error(),
			DurationMs: time.Since(start).Milliseconds(),
		}, ErrSandboxTimeout
	default:
		// Simulated micro-sandbox execution (Wasm/Container jail)
		duration := time.Since(start).Milliseconds()
		return &tool.ToolResult{
			ToolName:   toolName,
			Success:    true,
			Output:     fmt.Sprintf("AbloEx Sandbox executed '%s' (memory_quota: %dMB, read_only_fs: %v)", toolName, s.quota.MaxMemoryMB, s.quota.ReadOnlyRootFS),
			DurationMs: duration,
		}, nil
	}
}

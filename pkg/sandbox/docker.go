package sandbox

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"octpex/pkg/tool"
	"os/exec"
	"strings"
	"time"
)

// cappedOutput drains the container process while bounding host memory use.
type cappedOutput struct {
	data      bytes.Buffer
	truncated bool
}

func (w *cappedOutput) Write(p []byte) (int, error) {
	n := len(p)
	if remaining := 65536 - w.data.Len(); n > remaining {
		p = p[:remaining]
		w.truncated = true
	}
	w.data.Write(p)
	return n, nil
}

type dockerRunner struct {
	quota SandboxQuota
	image string
}

func (d *dockerRunner) Run(ctx context.Context, name, code string, params map[string]any) (*tool.ToolResult, error) {
	if d.image == "" || strings.HasPrefix(d.image, "-") || d.quota.MaxMemoryMB <= 0 {
		return nil, fmt.Errorf("invalid sandbox image or memory quota")
	}
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		return nil, err
	}
	container := "octpex-" + hex.EncodeToString(id[:])
	payload, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}
	memory := fmt.Sprintf("%dm", d.quota.MaxMemoryMB)
	args := []string{"run", "--rm", "--pull=never", "-i", "--name", container, "--network=none", "--read-only", "--cap-drop=ALL", "--security-opt=no-new-privileges", "--pids-limit=32", "--cpus=1", "--user=65534:65534", "--memory", memory, "--memory-swap", memory}
	if strings.HasPrefix(code, "#!/bin/sh") || strings.HasPrefix(code, "#!/bin/bash") {
		// POSIX shell scripts use the shell provided by the pinned image.
		args = append(args, "--entrypoint", "/bin/sh", d.image, "-c", code, name)
	} else {
		args = append(args, "--entrypoint", "python3", d.image, "-I", "-B", "-c", code)
	}
	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Stdin = bytes.NewReader(payload)
	var stdout, stderr cappedOutput
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	started := time.Now()
	err = cmd.Run()
	if ctx.Err() != nil {
		cleanup, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = exec.CommandContext(cleanup, "docker", "rm", "-f", container).Run()
		return nil, ctx.Err()
	}
	if err != nil {
		return nil, fmt.Errorf("isolated tool %s: %w: %s", name, err, stderr.data.String())
	}
	if stdout.truncated {
		return nil, fmt.Errorf("tool output exceeded 64 KiB")
	}
	return &tool.ToolResult{ToolName: name, Success: true, Output: stdout.data.String(), DurationMs: time.Since(started).Milliseconds()}, nil
}

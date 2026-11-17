package tool

import (
	"context"
	"testing"
)

func TestDynamicMCPBoxRegistrationAndInvocation(t *testing.T) {
	ctx := context.Background()
	box := NewDynamicMCPBox()

	// 1. Register static tool
	schema := &ToolSchema{
		Name:        "k8s-pod-restart",
		Description: "Restarts a target deployment pod",
		IsDynamic:   false,
	}
	box.RegisterTool(schema, func(ctx context.Context, params map[string]any) (*ToolResult, error) {
		podName := params["pod"].(string)
		return &ToolResult{
			ToolName: "k8s-pod-restart",
			Success:  true,
			Output:   "restarted pod " + podName,
		}, nil
	})

	// 2. Invoke static tool
	res, err := box.InvokeTool(ctx, "k8s-pod-restart", map[string]any{"pod": "payment-api-7b9"})
	if err != nil {
		t.Fatalf("invoke failed: %v", err)
	}
	if !res.Success || res.Output != "restarted pod payment-api-7b9" {
		t.Fatalf("unexpected result: %+v", res)
	}

	// 3. Register runtime dynamic script synthesized by Octo-Brain
	if err := box.RegisterDynamicScript("dynamic-memory-patcher", "Patches deployment YAML memory limits", "#!/bin/bash\necho ok"); err != nil {
		t.Fatalf("register dynamic script failed: %v", err)
	}

	// 4. Inspect dynamic tool schema
	dynSchema, err := box.InspectTool(ctx, "dynamic-memory-patcher")
	if err != nil {
		t.Fatalf("inspect tool failed: %v", err)
	}
	if !dynSchema.IsDynamic {
		t.Error("expected dynamic tool to have IsDynamic = true")
	}

	// 5. Invoke dynamic tool
	dynRes, err := box.InvokeTool(ctx, "dynamic-memory-patcher", nil)
	if err != nil {
		t.Fatalf("invoke dynamic tool failed: %v", err)
	}
	if !dynRes.Success {
		t.Error("expected dynamic tool execution success")
	}

	// 6. Invoke unknown tool
	_, err = box.InvokeTool(ctx, "non-existent-tool", nil)
	if err == nil {
		t.Fatal("expected error on non-existent tool, got nil")
	}
}

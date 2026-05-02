package agent

import (
	"context"
	"testing"
)

func TestGetDeviceInfoTool_Interface(t *testing.T) {
	var _ Tool = (*GetDeviceInfoTool)(nil)
}

func TestGetDeviceInfoTool_Metadata(t *testing.T) {
	tool := NewGetDeviceInfoTool(nil)

	if tool.Name() != "get_device_info" {
		t.Errorf("got name %q, want %q", tool.Name(), "get_device_info")
	}
	if tool.Description() == "" {
		t.Error("description should not be empty")
	}

	schema := tool.InputSchema()
	if schema["type"] != "object" {
		t.Errorf("schema type = %v, want object", schema["type"])
	}
	props, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatal("schema properties missing or wrong type")
	}
	if _, ok := props["host"]; !ok {
		t.Error("schema missing 'host' property")
	}
	req, ok := schema["required"].([]string)
	if !ok || len(req) == 0 || req[0] != "host" {
		t.Error("schema required should contain 'host'")
	}
}

func TestGetDeviceInfoTool_MissingHost(t *testing.T) {
	tool := NewGetDeviceInfoTool(nil)
	result, err := tool.Execute(context.Background(), map[string]any{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("expected IsError=true for missing host")
	}
	if result.RiskLevel != RiskSafe {
		t.Errorf("expected RiskSafe, got %q", result.RiskLevel)
	}
}

func TestExecuteCLITool_Interface(t *testing.T) {
	var _ Tool = (*ExecuteCLITool)(nil)
}

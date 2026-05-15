package agent

import (
	"strings"
	"testing"
)

func TestBatchExecuteTool_InputSchema_HasIntent(t *testing.T) {
	tool := &BatchExecuteTool{}
	schema := tool.InputSchema()
	props, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatal("no properties in schema")
	}
	if _, ok := props["intent"]; !ok {
		t.Error("intent field missing from InputSchema")
	}
	required, _ := schema["required"].([]string)
	found := false
	for _, r := range required {
		if r == "intent" {
			found = true
			break
		}
	}
	if !found {
		t.Error("intent should be in required")
	}
}

func TestBatchExecuteTool_Description_MentionsIntent(t *testing.T) {
	tool := &BatchExecuteTool{}
	if !strings.Contains(tool.Description(), "intent") {
		t.Error("Description should mention intent field")
	}
}

func TestBatchExecuteTool_OutputFormat(t *testing.T) {
	results := []batchHostResult{
		{
			HostID:     "host1",
			HostName:   "server-1",
			Stdout:     "uptime output",
			Stderr:     "",
			ExitCode:   0,
			DurationMs: 123,
		},
		{
			HostID:     "host2",
			HostName:   "server-2",
			Stdout:     "",
			Stderr:     "",
			ExitCode:   0,
			DurationMs: 45,
		},
		{
			HostID:   "host3",
			HostName: "server-3",
			Error:    "connection timeout",
		},
	}

	var output strings.Builder
	for _, r := range results {
		output.WriteString("Host: " + r.HostName + " (" + r.HostID + ")\n")
		if r.Error != "" {
			output.WriteString("  Error: " + r.Error + "\n")
		} else {
			output.WriteString("  Exit Code: 0\n")
			output.WriteString("  Duration: 123ms\n")
			if r.Stdout != "" {
				output.WriteString("  Stdout:\n" + r.Stdout + "\n")
			}
			if r.Stderr != "" {
				output.WriteString("  Stderr:\n" + r.Stderr + "\n")
			}
		}
		output.WriteString("\n")
	}

	result := output.String()

	// Verify structure
	if !strings.Contains(result, "Host: server-1 (host1)") {
		t.Error("missing host1 header")
	}
	if !strings.Contains(result, "Exit Code: 0") {
		t.Error("missing exit code")
	}
	if !strings.Contains(result, "Stdout:\nuptime output") {
		t.Error("missing stdout section")
	}
	if !strings.Contains(result, "Error: connection timeout") {
		t.Error("missing error for host3")
	}

	// Verify no JSON artifacts
	if strings.Contains(result, "{") || strings.Contains(result, "}") {
		t.Error("output should not contain JSON braces")
	}
	if strings.Contains(result, `"host_id"`) {
		t.Error("output should not contain JSON field names")
	}
}

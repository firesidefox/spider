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
	for _, r := range required {
		if r == "intent" {
			t.Error("intent should NOT be in required")
		}
	}
}

func TestBatchExecuteTool_Description_MentionsIntent(t *testing.T) {
	tool := &BatchExecuteTool{}
	if !strings.Contains(tool.Description(), "intent") {
		t.Error("Description should mention intent field")
	}
}

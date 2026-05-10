package agent

import (
	"strings"
	"testing"
)

func TestCallRESTAPITool_InputSchema_HasIntent(t *testing.T) {
	tool := NewCallRESTAPITool(nil)
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

func TestCallRESTAPITool_Description_MentionsIntent(t *testing.T) {
	tool := NewCallRESTAPITool(nil)
	if !strings.Contains(tool.Description(), "intent") {
		t.Error("Description should mention intent field")
	}
}

package agent

import (
	"context"
	"encoding/json"

	"github.com/spiderai/spider/internal/store"
)

type ListDevicesTool struct {
	hosts *store.HostStore
}

func NewListDevicesTool(hosts *store.HostStore) *ListDevicesTool {
	return &ListDevicesTool{hosts: hosts}
}

func (t *ListDevicesTool) Name() string        { return "list_devices" }
func (t *ListDevicesTool) Description() string { return "List all managed devices, optionally filtered by tag" }

func (t *ListDevicesTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"tag": map[string]any{
				"type":        "string",
				"description": "Filter by tag (optional, empty = all devices)",
			},
		},
	}
}

func (t *ListDevicesTool) Execute(_ context.Context, input map[string]any) (*ToolResult, error) {
	tag, _ := input["tag"].(string)
	hosts, err := t.hosts.List(tag)
	if err != nil {
		return &ToolResult{Content: "failed to list devices: " + err.Error(), IsError: true, RiskLevel: RiskSafe}, nil
	}
	if len(hosts) == 0 {
		return &ToolResult{Content: "no devices found", RiskLevel: RiskSafe}, nil
	}

	type deviceSummary struct {
		ID        string   `json:"id"`
		Name      string   `json:"name"`
		IP        string   `json:"ip"`
		Vendor    string   `json:"vendor,omitempty"`
		Model     string   `json:"model,omitempty"`
		CLIType   string   `json:"cli_type,omitempty"`
		Tags      []string `json:"tags,omitempty"`
	}

	devices := make([]deviceSummary, len(hosts))
	for i, h := range hosts {
		devices[i] = deviceSummary{
			ID: h.ID, Name: h.Name, IP: h.IP,
			Vendor: h.Vendor, Model: h.Model,
			CLIType: h.CLIType, Tags: h.Tags,
		}
	}

	out, _ := json.Marshal(devices)
	return &ToolResult{Content: string(out), RiskLevel: RiskSafe}, nil
}

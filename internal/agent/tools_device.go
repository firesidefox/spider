package agent

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/spiderai/spider/internal/store"
)

type GetDeviceInfoTool struct {
	hosts *store.HostStore
}

func NewGetDeviceInfoTool(hosts *store.HostStore) *GetDeviceInfoTool {
	return &GetDeviceInfoTool{hosts: hosts}
}

func (t *GetDeviceInfoTool) Name() string { return "get_device_info" }

func (t *GetDeviceInfoTool) Description() string {
	return "Get device information by host ID or name"
}

func (t *GetDeviceInfoTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"host": map[string]any{
				"type":        "string",
				"description": "Host ID or name",
			},
		},
		"required": []string{"host"},
	}
}

func (t *GetDeviceInfoTool) Execute(_ context.Context, input map[string]any) (*ToolResult, error) {
	hostID, _ := input["host"].(string)
	if hostID == "" {
		return &ToolResult{Content: "missing required field: host", IsError: true, RiskLevel: RiskSafe}, nil
	}

	h, err := t.hosts.GetByIDOrName(hostID)
	if err != nil {
		return &ToolResult{Content: fmt.Sprintf("host not found: %v", err), IsError: true, RiskLevel: RiskSafe}, nil
	}

	info := map[string]any{
		"id":               h.ID,
		"name":             h.Name,
		"ip":               h.IP,
		"port":             h.Port,
		"vendor":           h.Vendor,
		"device_type":      h.DeviceType,
		"model":            h.Model,
		"cli_type":         h.CLIType,
		"firmware_version": h.FirmwareVersion,
		"tags":             h.Tags,
	}

	out, err := json.Marshal(info)
	if err != nil {
		return &ToolResult{Content: fmt.Sprintf("marshal error: %v", err), IsError: true, RiskLevel: RiskSafe}, nil
	}
	return &ToolResult{Content: string(out), RiskLevel: RiskSafe}, nil
}

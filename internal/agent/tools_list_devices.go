package agent

import (
	"context"
	"encoding/json"
	"log"

	"github.com/spiderai/spider/internal/models"
	"github.com/spiderai/spider/internal/store"
)

type ListDevicesTool struct {
	hosts *store.HostStore
	faces *store.AccessFaceStore
}

func NewListDevicesTool(hosts *store.HostStore, faces *store.AccessFaceStore) *ListDevicesTool {
	return &ListDevicesTool{hosts: hosts, faces: faces}
}

func (t *ListDevicesTool) DefaultRiskLevel() RiskLevel { return RiskL1 }
func (t *ListDevicesTool) Name() string                 { return "ListHosts" }
func (t *ListDevicesTool) Description() string {
	return "List all managed devices, optionally filtered by tag. Read-only. No side effects. Use freely in Explore phase."
}

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

const listDevicesPromptSection = `## ListHosts / SearchDocs (read-only, no side effects)

**When to use:** Call these freely at the start of any task to understand the environment.

<example>
User: Check disk usage on all web servers.
Assistant: Calls ListHosts to find web servers before running any commands.
</example>`

func (t *ListDevicesTool) SystemPromptSection() string {
	return listDevicesPromptSection
}

func (t *ListDevicesTool) Execute(_ context.Context, input map[string]any) (*ToolResult, error) {
	tag, _ := input["tag"].(string)
	hosts, err := t.hosts.List(tag)
	if err != nil {
		return &ToolResult{Content: "failed to list devices: " + err.Error(), IsError: true, RiskLevel: RiskL1}, nil
	}
	if len(hosts) == 0 {
		return &ToolResult{Content: "no devices found", RiskLevel: RiskL1}, nil
	}

	type deviceSummary struct {
		ID          string                   `json:"id"`
		Name        string                   `json:"name"`
		IP          string                   `json:"ip"`
		Vendor      string                   `json:"vendor,omitempty"`
		Tags        []string                 `json:"tags,omitempty"`
		AccessFaces []models.AccessFaceType  `json:"access_faces"`
	}

	hostIDs := make([]string, len(hosts))
	for i, h := range hosts {
		hostIDs[i] = h.ID
	}
	var faceMap map[string][]models.AccessFaceType
	if t.faces != nil {
		var err error
		faceMap, err = t.faces.FaceTypesByHostIDs(hostIDs)
		if err != nil {
			log.Printf("WARNING: FaceTypesByHostIDs failed: %v", err)
		}
	}

	devices := make([]deviceSummary, len(hosts))
	for i, h := range hosts {
		ft := faceMap[h.ID]
		if ft == nil {
			ft = []models.AccessFaceType{}
		}
		devices[i] = deviceSummary{
			ID: h.ID, Name: h.Name, IP: h.IP,
			Vendor: h.Vendor, Tags: h.Tags, AccessFaces: ft,
		}
	}

	out, _ := json.Marshal(devices)
	return &ToolResult{Content: string(out), RiskLevel: RiskL1}, nil
}

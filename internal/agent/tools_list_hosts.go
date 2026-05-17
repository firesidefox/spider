package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/spiderai/spider/internal/models"
	"github.com/spiderai/spider/internal/store"
)

type ListHostsTool struct {
	hosts           *store.HostStore
	faces           *store.AccessFaceStore
	selectedHostIDs []string
}

func NewListHostsTool(hosts *store.HostStore, faces *store.AccessFaceStore) *ListHostsTool {
	return &ListHostsTool{hosts: hosts, faces: faces}
}

func (t *ListHostsTool) DefaultRiskLevel() RiskLevel { return RiskL1 }
func (t *ListHostsTool) Name() string                { return "ListHosts" }
func (t *ListHostsTool) Description() string {
	return "List all managed hosts, optionally filtered by tag. Read-only. No side effects. Use freely in Explore phase."
}

func (t *ListHostsTool) InputSchema() map[string]any {
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

const listHostsPromptSection = `## ListHosts

**When to use:** At the start of any task that targets specific hosts — get host IDs and access face types before running commands or API calls.

**Access face types and tool mapping:**
- "ssh" face → use RunCommand / RunCommandBatch / PollUntil
- "api" face → use CallAPI (check face.knowledge_sources for API docs group_id)

<example>
User: Check disk usage on all web servers.
Assistant: ListHosts → find hosts with ssh face → RunCommandBatch "df -h"
</example>

<example>
User: Push a new ACL rule via the firewall API.
Assistant: ListHosts → find gateway host with api face → SearchDocs (group_id from face.knowledge_sources) → CallAPI POST
</example>`

func (t *ListHostsTool) SystemPromptSection() string {
	return listHostsPromptSection
}

func (t *ListHostsTool) Execute(_ context.Context, input map[string]any) (*ToolResult, error) {
	tag, _ := input["tag"].(string)
	hosts, err := t.hosts.List(tag)
	if err != nil {
		return &ToolResult{Content: "failed to list hosts: " + err.Error(), IsError: true, RiskLevel: RiskL1}, nil
	}

	// Restrict to selected hosts when user has narrowed the scope
	if len(t.selectedHostIDs) > 0 {
		allowed := make(map[string]bool, len(t.selectedHostIDs))
		for _, id := range t.selectedHostIDs {
			allowed[id] = true
		}
		filtered := hosts[:0]
		for _, h := range hosts {
			if allowed[h.ID] {
				filtered = append(filtered, h)
			}
		}
		hosts = filtered
	}

	if len(hosts) == 0 {
		return &ToolResult{Content: "no hosts found", RiskLevel: RiskL1}, nil
	}

	type hostSummary struct {
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

	hosts2 := make([]hostSummary, len(hosts))
	for i, h := range hosts {
		ft := faceMap[h.ID]
		if ft == nil {
			ft = []models.AccessFaceType{}
		}
		hosts2[i] = hostSummary{
			ID: h.ID, Name: h.Name, IP: h.IP,
			Vendor: h.Vendor, Tags: h.Tags, AccessFaces: ft,
		}
	}

	out, _ := json.Marshal(hosts2)
	return &ToolResult{Content: string(out), RiskLevel: RiskL1, Summary: fmt.Sprintf("%d hosts", len(hosts))}, nil
}

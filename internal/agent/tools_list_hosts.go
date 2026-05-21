package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/spiderai/spider/internal/models"
	"github.com/spiderai/spider/internal/store"
)

type GetHostsTool struct {
	hosts           *store.HostStore
	faces           *store.AccessFaceStore
	selectedHostIDs []string
}

func NewGetHostsTool(hosts *store.HostStore, faces *store.AccessFaceStore) *GetHostsTool {
	return &GetHostsTool{hosts: hosts, faces: faces}
}

func (t *GetHostsTool) DefaultRiskLevel() RiskLevel              { return RiskL1 }
func (t *GetHostsTool) IsConcurrencySafe(_ map[string]any) bool { return true }
func (t *GetHostsTool) Name() string                { return "GetHosts" }
func (t *GetHostsTool) Description() string {
	return "List all managed hosts, optionally filtered by tag. Read-only. No side effects. Use freely in Explore phase."
}

func (t *GetHostsTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"tag": map[string]any{
				"type":        "string",
				"description": "Filter by tag (optional, empty = all hosts)",
			},
			"name": map[string]any{
				"type":        "string",
				"description": "Filter by exact host name (optional)",
			},
		},
	}
}

const getHostsPromptSection = `## GetHosts

**When to use:** At the start of any task that targets specific hosts — get host IDs and access face info before running commands or API calls.

**Output format:** Each host has access_faces array with {id, type, knowledge_sources}.
- Use face.id (not host.id) when calling CallAPI
- face.type: "ssh" → RunCommand/RunCommandBatch/PollUntil; "restapi" → CallAPI
- face.knowledge_sources: [{type:"group",id:N}] → pass group id=N to SearchDocs for API docs

<example>
User: Check disk usage on all web servers.
Assistant: GetHosts → find hosts with ssh face → RunCommandBatch "df -h"
</example>

<example>
User: Push a new ACL rule via the firewall API.
Assistant: GetHosts → find gateway with restapi face → SearchDocs group_id=face.knowledge_sources[0].id → CallAPI face_id=face.id
</example>`

func (t *GetHostsTool) SystemPromptSection() string {
	return getHostsPromptSection
}

func (t *GetHostsTool) Execute(_ context.Context, input map[string]any) (*ToolResult, error) {
	tag, _ := input["tag"].(string)
	name, _ := input["name"].(string)
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

	// Filter by name if specified
	if name != "" {
		filtered := hosts[:0]
		for _, h := range hosts {
			if h.Name == name {
				filtered = append(filtered, h)
			}
		}
		hosts = filtered
	}

	if len(hosts) == 0 {
		return &ToolResult{Content: "no hosts found", RiskLevel: RiskL1}, nil
	}

	type faceSummary struct {
		ID               string                      `json:"id"`
		Type             models.AccessFaceType       `json:"type"`
		KnowledgeSources []models.KnowledgeSourceRef `json:"knowledge_sources,omitempty"`
	}

	type hostSummary struct {
		ID          string        `json:"id"`
		Name        string        `json:"name"`
		IP          string        `json:"ip"`
		Vendor      string        `json:"vendor,omitempty"`
		Tags        []string      `json:"tags,omitempty"`
		AccessFaces []faceSummary `json:"access_faces"`
	}

	hosts2 := make([]hostSummary, len(hosts))
	for i, h := range hosts {
		var faces []faceSummary
		if t.faces != nil {
			faceList, err := t.faces.ListByHost(h.ID)
			if err != nil {
				log.Printf("WARNING: ListByHost(%s) failed: %v", h.ID, err)
			} else {
				faces = make([]faceSummary, len(faceList))
				for j, f := range faceList {
					faces[j] = faceSummary{
						ID:               f.ID,
						Type:             f.Type,
						KnowledgeSources: f.KnowledgeSources,
					}
				}
			}
		}
		if faces == nil {
			faces = []faceSummary{}
		}
		hosts2[i] = hostSummary{
			ID: h.ID, Name: h.Name, IP: h.IP,
			Vendor: h.Vendor, Tags: h.Tags, AccessFaces: faces,
		}
	}

	out, _ := json.Marshal(hosts2)
	return &ToolResult{Content: string(out), RiskLevel: RiskL1, Summary: fmt.Sprintf("%d hosts", len(hosts2))}, nil
}

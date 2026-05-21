package agent

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/spiderai/spider/internal/store"
)

type GetTopologyTool struct {
	topos *store.TopologyStore
}

func NewGetTopologyTool(topos *store.TopologyStore) *GetTopologyTool {
	return &GetTopologyTool{topos: topos}
}

func (t *GetTopologyTool) DefaultRiskLevel() RiskLevel              { return RiskL1 }
func (t *GetTopologyTool) IsConcurrencySafe(_ map[string]any) bool { return true }
func (t *GetTopologyTool) Name() string                { return "GetTopology" }
func (t *GetTopologyTool) Description() string {
	return "Get topology data including groups, nodes, and edges. Read-only. Use in Explore phase."
}

func (t *GetTopologyTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"topology_id": map[string]any{
				"type":        "string",
				"description": "Topology ID (use if known)",
			},
			"topology_name": map[string]any{
				"type":        "string",
				"description": "Topology name (used if topology_id not provided)",
			},
		},
	}
}

const getTopologyPrompt = `## GetTopology

**When to use:** When you need the full topology structure — listing all nodes, understanding the overall network layout, or when you don't know the target host name yet.

**When NOT to use:** When you already know the host name and need its network position or upstream path — use GetTopologyContext instead, it returns a focused result without noise.`

func (t *GetTopologyTool) SystemPromptSection() string { return getTopologyPrompt }

func (t *GetTopologyTool) Execute(ctx context.Context, input map[string]any) (*ToolResult, error) {
	if t.topos == nil {
		return &ToolResult{Content: "topology store not configured", IsError: true, RiskLevel: RiskL1}, nil
	}
	id, _ := input["topology_id"].(string)
	name, _ := input["topology_name"].(string)

	if id == "" && name == "" {
		list, err := t.topos.List()
		if err != nil {
			return nil, err
		}
		b, _ := json.Marshal(list)
		return &ToolResult{Content: string(b)}, nil
	}

	if id == "" {
		list, err := t.topos.List()
		if err != nil {
			return nil, err
		}
		for _, topo := range list {
			if topo.Name == name {
				id = topo.ID
				break
			}
		}
		if id == "" {
			return nil, fmt.Errorf("topology %q not found", name)
		}
	}

	full, err := t.topos.GetFull(id)
	if err != nil {
		return nil, err
	}
	b, _ := json.Marshal(full)
	return &ToolResult{Content: string(b)}, nil
}

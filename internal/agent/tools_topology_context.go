package agent

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/spiderai/spider/internal/models"
	"github.com/spiderai/spider/internal/store"
)

type GetTopologyContextTool struct {
	topos *store.TopologyStore
}

func NewGetTopologyContextTool(topos *store.TopologyStore) *GetTopologyContextTool {
	return &GetTopologyContextTool{topos: topos}
}

func (t *GetTopologyContextTool) DefaultRiskLevel() RiskLevel              { return RiskL1 }
func (t *GetTopologyContextTool) IsConcurrencySafe(_ map[string]any) bool { return true }
func (t *GetTopologyContextTool) Name() string                { return "GetTopologyContext" }

func (t *GetTopologyContextTool) Description() string {
	return "Query a host's position in the network topology: its layer, role, direct upstream/downstream neighbors, and full path to the root. Read-only. Use in Explore phase when diagnosing connectivity or understanding network path."
}

func (t *GetTopologyContextTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"host_name": map[string]any{
				"type":        "string",
				"description": "Node name or bound host name to look up",
			},
			"host_id": map[string]any{
				"type":        "string",
				"description": "Bound host ID to look up (alternative to host_name)",
			},
			"topology_id": map[string]any{
				"type":        "string",
				"description": "Topology ID to search in (optional; searches all topologies if omitted)",
			},
		},
	}
}

const topologyContextPrompt = `## GetTopologyContext

**When to use:** When diagnosing network issues, understanding a host's network path, or identifying which upstream devices to check. Call this before running commands on network devices to understand the blast radius.

**When NOT to use:** When you need the full topology structure — use GetTopology instead.

**Output fields:**
- node: the matched node (name, layer, role, host_id, ip)
- upstream: direct upstream neighbors (one hop)
- downstream: direct downstream neighbors (one hop)
- path_to_root: ordered list from this node up to the root (useful for tracing network path)`

func (t *GetTopologyContextTool) SystemPromptSection() string { return topologyContextPrompt }

type nodeInfo struct {
	Name   string `json:"name"`
	Layer  string `json:"layer"`
	Role   string `json:"role"`
	HostID string `json:"host_id,omitempty"`
	IP     string `json:"ip,omitempty"`
}

type topologyContextResult struct {
	Node       nodeInfo   `json:"node"`
	Upstream   []nodeInfo `json:"upstream"`
	Downstream []nodeInfo `json:"downstream"`
	PathToRoot []nodeInfo `json:"path_to_root"`
}

func (t *GetTopologyContextTool) Execute(_ context.Context, input map[string]any) (*ToolResult, error) {
	if t.topos == nil {
		return &ToolResult{Content: "topology store not configured", IsError: true, RiskLevel: RiskL1}, nil
	}
	hostName, _ := input["host_name"].(string)
	hostID, _ := input["host_id"].(string)
	topoID, _ := input["topology_id"].(string)

	if hostName == "" && hostID == "" {
		return &ToolResult{Content: "host_name or host_id is required", IsError: true, RiskLevel: RiskL1}, nil
	}

	// collect topologies to search
	var topoIDs []string
	if topoID != "" {
		topoIDs = []string{topoID}
	} else {
		list, err := t.topos.List()
		if err != nil {
			return nil, err
		}
		for _, topo := range list {
			topoIDs = append(topoIDs, topo.ID)
		}
	}

	for _, tid := range topoIDs {
		full, err := t.topos.GetFull(tid)
		if err != nil {
			continue
		}
		node := findNode(full.Nodes, hostName, hostID)
		if node == nil {
			continue
		}

		nodeByID := make(map[string]*models.TopologyNode, len(full.Nodes))
		for _, n := range full.Nodes {
			nodeByID[n.ID] = n
		}

		upstream := neighbors(full.Edges, node.ID, nodeByID, true)
		downstream := neighbors(full.Edges, node.ID, nodeByID, false)
		path := pathToRoot(full.Edges, node, nodeByID)

		result := topologyContextResult{
			Node:       toNodeInfo(node),
			Upstream:   upstream,
			Downstream: downstream,
			PathToRoot: path,
		}
		b, _ := json.Marshal(result)
		return &ToolResult{Content: string(b), RiskLevel: RiskL1}, nil
	}

	return &ToolResult{Content: fmt.Sprintf("node not found: host_name=%q host_id=%q", hostName, hostID), IsError: true, RiskLevel: RiskL1}, nil
}

func findNode(nodes []*models.TopologyNode, name, hostID string) *models.TopologyNode {
	for _, n := range nodes {
		if hostID != "" && n.HostID == hostID {
			return n
		}
		if name != "" && (n.Name == name || n.HostName == name) {
			return n
		}
	}
	return nil
}

func toNodeInfo(n *models.TopologyNode) nodeInfo {
	return nodeInfo{Name: n.Name, Layer: n.Layer, Role: n.Role, HostID: n.HostID, IP: n.IP}
}

func neighbors(edges []*models.TopologyEdge, nodeID string, byID map[string]*models.TopologyNode, upstream bool) []nodeInfo {
	var result []nodeInfo
	for _, e := range edges {
		var neighborID string
		if upstream && e.ToNode == nodeID {
			neighborID = e.FromNode
		} else if !upstream && e.FromNode == nodeID {
			neighborID = e.ToNode
		}
		if neighborID != "" {
			if n, ok := byID[neighborID]; ok {
				result = append(result, toNodeInfo(n))
			}
		}
	}
	return result
}

func pathToRoot(edges []*models.TopologyEdge, start *models.TopologyNode, byID map[string]*models.TopologyNode) []nodeInfo {
	var path []nodeInfo
	visited := map[string]bool{start.ID: true}
	cur := start
	for {
		var parentID string
		for _, e := range edges {
			if e.ToNode == cur.ID {
				parentID = e.FromNode
				break
			}
		}
		if parentID == "" || visited[parentID] {
			break
		}
		parent, ok := byID[parentID]
		if !ok {
			break
		}
		path = append(path, toNodeInfo(parent))
		visited[parentID] = true
		cur = parent
	}
	return path
}

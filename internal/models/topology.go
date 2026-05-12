package models

import "time"

type Topology struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Notes     string    `json:"notes"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type TopologyGroup struct {
	ID         string    `json:"id"`
	TopologyID string    `json:"topology_id"`
	Name       string    `json:"name"`
	Color      string    `json:"color"`
	SortOrder  int       `json:"sort_order"`
	CreatedAt  time.Time `json:"created_at"`
}

type TopologyNode struct {
	ID         string    `json:"id"`
	TopologyID string    `json:"topology_id"`
	GroupID    string    `json:"group_id"`
	Name       string    `json:"name"`
	Role       string    `json:"role"`
	HostID     string    `json:"host_id,omitempty"`
	Notes      string    `json:"notes"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
	HostName   string    `json:"host_name,omitempty"`
	IP         string    `json:"ip,omitempty"`
}

type TopologyEdge struct {
	ID         string    `json:"id"`
	TopologyID string    `json:"topology_id"`
	FromNode   string    `json:"from_node"`
	ToNode     string    `json:"to_node"`
	CreatedAt  time.Time `json:"created_at"`
}

type TopologyFull struct {
	Topology
	Groups []*TopologyGroup `json:"groups"`
	Nodes  []*TopologyNode  `json:"nodes"`
	Edges  []*TopologyEdge  `json:"edges"`
}

type CreateTopologyRequest struct {
	Name  string `json:"name"`
	Notes string `json:"notes"`
}

type UpdateTopologyRequest struct {
	Name  string `json:"name"`
	Notes string `json:"notes"`
}

type CreateGroupRequest struct {
	Name      string `json:"name"`
	Color     string `json:"color"`
	SortOrder int    `json:"sort_order"`
}

type UpdateGroupRequest struct {
	Name      string `json:"name"`
	Color     string `json:"color"`
	SortOrder int    `json:"sort_order"`
}

type CreateNodeRequest struct {
	GroupID string `json:"group_id"`
	Name    string `json:"name"`
	Role    string `json:"role"`
	HostID  string `json:"host_id"`
	Notes   string `json:"notes"`
}

type UpdateNodeRequest struct {
	GroupID string `json:"group_id"`
	Name    string `json:"name"`
	Role    string `json:"role"`
	HostID  string `json:"host_id"`
	Notes   string `json:"notes"`
}

type CreateEdgeRequest struct {
	FromNode string `json:"from_node"`
	ToNode   string `json:"to_node"`
}

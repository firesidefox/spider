package models

import "time"

type Topology struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Notes     string    `json:"notes"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type TopologyNode struct {
	ID         string    `json:"id"`
	TopologyID string    `json:"topology_id"`
	Layer      string    `json:"layer"`
	Name       string    `json:"name"`
	Role       string    `json:"role"`
	HostID     string    `json:"host_id,omitempty"`
	Notes      string    `json:"notes"`
	PosX       float64   `json:"pos_x"`
	PosY       float64   `json:"pos_y"`
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
	Nodes []*TopologyNode `json:"nodes"`
	Edges []*TopologyEdge `json:"edges"`
}

type CreateTopologyRequest struct {
	Name  string `json:"name"`
	Notes string `json:"notes"`
}

type UpdateTopologyRequest struct {
	Name  string `json:"name"`
	Notes string `json:"notes"`
}

type CreateNodeRequest struct {
	Layer  string `json:"layer"`
	Name   string `json:"name"`
	Role   string `json:"role"`
	HostID string `json:"host_id"`
	Notes  string `json:"notes"`
}

type UpdateNodeRequest struct {
	Layer  string   `json:"layer"`
	Name   string   `json:"name"`
	Role   string   `json:"role"`
	HostID string   `json:"host_id"`
	Notes  string   `json:"notes"`
	PosX   *float64 `json:"pos_x,omitempty"`
	PosY   *float64 `json:"pos_y,omitempty"`
}

type CreateEdgeRequest struct {
	FromNode string `json:"from_node"`
	ToNode   string `json:"to_node"`
}

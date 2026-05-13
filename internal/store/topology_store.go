package store

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/spiderai/spider/internal/models"
)

type TopologyStore struct {
	db *sql.DB
}

func NewTopologyStore(db *sql.DB) *TopologyStore {
	return &TopologyStore{db: db}
}

func (s *TopologyStore) List() ([]*models.Topology, error) {
	rows, err := s.db.Query(`SELECT id, name, notes, created_at, updated_at FROM topologies ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []*models.Topology
	for rows.Next() {
		var t models.Topology
		if err := rows.Scan(&t.ID, &t.Name, &t.Notes, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		list = append(list, &t)
	}
	return list, nil
}

func (s *TopologyStore) Create(req *models.CreateTopologyRequest) (*models.Topology, error) {
	if req.Name == "" {
		return nil, fmt.Errorf("name is required")
	}
	now := time.Now().UTC()
	id := uuid.New().String()
	_, err := s.db.Exec(
		`INSERT INTO topologies (id, name, notes, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`,
		id, req.Name, req.Notes, now, now,
	)
	if err != nil {
		return nil, fmt.Errorf("create topology: %w", err)
	}
	return s.GetByID(id)
}

func (s *TopologyStore) GetByID(id string) (*models.Topology, error) {
	var t models.Topology
	err := s.db.QueryRow(
		`SELECT id, name, notes, created_at, updated_at FROM topologies WHERE id = ?`, id,
	).Scan(&t.ID, &t.Name, &t.Notes, &t.CreatedAt, &t.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	return &t, err
}

func (s *TopologyStore) Update(id string, req *models.UpdateTopologyRequest) (*models.Topology, error) {
	now := time.Now().UTC()
	_, err := s.db.Exec(
		`UPDATE topologies SET name = ?, notes = ?, updated_at = ? WHERE id = ?`,
		req.Name, req.Notes, now, id,
	)
	if err != nil {
		return nil, fmt.Errorf("update topology: %w", err)
	}
	return s.GetByID(id)
}

func (s *TopologyStore) Delete(id string) error {
	res, err := s.db.Exec(`DELETE FROM topologies WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *TopologyStore) ListNodes(topologyID string) ([]*models.TopologyNode, error) {
	rows, err := s.db.Query(
		`SELECT n.id, n.topology_id, n.layer, n.name, n.role,
		        COALESCE(n.host_id,''), n.notes, n.created_at, n.updated_at,
		        COALESCE(h.name,''), COALESCE(h.ip,'')
		 FROM topology_nodes n
		 LEFT JOIN hosts h ON h.id = n.host_id
		 WHERE n.topology_id = ? ORDER BY n.name`,
		topologyID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []*models.TopologyNode
	for rows.Next() {
		var n models.TopologyNode
		if err := rows.Scan(&n.ID, &n.TopologyID, &n.Layer, &n.Name, &n.Role,
			&n.HostID, &n.Notes, &n.CreatedAt, &n.UpdatedAt, &n.HostName, &n.IP); err != nil {
			return nil, err
		}
		list = append(list, &n)
	}
	return list, nil
}

func (s *TopologyStore) CreateNode(topologyID string, req *models.CreateNodeRequest) (*models.TopologyNode, error) {
	id := uuid.New().String()
	now := time.Now().UTC()
	var hostID *string
	if req.HostID != "" {
		hostID = &req.HostID
	}
	_, err := s.db.Exec(
		`INSERT INTO topology_nodes (id, topology_id, layer, name, role, host_id, notes, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, topologyID, req.Layer, req.Name, req.Role, hostID, req.Notes, now, now,
	)
	if err != nil {
		return nil, fmt.Errorf("create node: %w", err)
	}
	var n models.TopologyNode
	err = s.db.QueryRow(
		`SELECT n.id, n.topology_id, n.layer, n.name, n.role,
		        COALESCE(n.host_id,''), n.notes, n.created_at, n.updated_at,
		        COALESCE(h.name,''), COALESCE(h.ip,'')
		 FROM topology_nodes n LEFT JOIN hosts h ON h.id = n.host_id WHERE n.id = ?`, id,
	).Scan(&n.ID, &n.TopologyID, &n.Layer, &n.Name, &n.Role,
		&n.HostID, &n.Notes, &n.CreatedAt, &n.UpdatedAt, &n.HostName, &n.IP)
	return &n, err
}

func (s *TopologyStore) UpdateNode(id string, req *models.UpdateNodeRequest) (*models.TopologyNode, error) {
	now := time.Now().UTC()
	var hostID *string
	if req.HostID != "" {
		hostID = &req.HostID
	}
	_, err := s.db.Exec(
		`UPDATE topology_nodes SET layer=?, name=?, role=?, host_id=?, notes=?, updated_at=? WHERE id=?`,
		req.Layer, req.Name, req.Role, hostID, req.Notes, now, id,
	)
	if err != nil {
		return nil, fmt.Errorf("update node: %w", err)
	}
	var n models.TopologyNode
	err = s.db.QueryRow(
		`SELECT n.id, n.topology_id, n.layer, n.name, n.role,
		        COALESCE(n.host_id,''), n.notes, n.created_at, n.updated_at,
		        COALESCE(h.name,''), COALESCE(h.ip,'')
		 FROM topology_nodes n LEFT JOIN hosts h ON h.id = n.host_id WHERE n.id = ?`, id,
	).Scan(&n.ID, &n.TopologyID, &n.Layer, &n.Name, &n.Role,
		&n.HostID, &n.Notes, &n.CreatedAt, &n.UpdatedAt, &n.HostName, &n.IP)
	return &n, err
}

func (s *TopologyStore) DeleteNode(id string) error {
	res, err := s.db.Exec(`DELETE FROM topology_nodes WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *TopologyStore) ListEdges(topologyID string) ([]*models.TopologyEdge, error) {
	rows, err := s.db.Query(
		`SELECT id, topology_id, from_node, to_node, created_at FROM topology_edges WHERE topology_id = ?`,
		topologyID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []*models.TopologyEdge
	for rows.Next() {
		var e models.TopologyEdge
		if err := rows.Scan(&e.ID, &e.TopologyID, &e.FromNode, &e.ToNode, &e.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, &e)
	}
	return list, nil
}

func (s *TopologyStore) CreateEdge(topologyID string, req *models.CreateEdgeRequest) (*models.TopologyEdge, error) {
	// Verify both endpoints belong to this topology
	var count int
	err := s.db.QueryRow(
		`SELECT COUNT(*) FROM topology_nodes WHERE topology_id = ? AND id IN (?, ?)`,
		topologyID, req.FromNode, req.ToNode,
	).Scan(&count)
	if err != nil {
		return nil, err
	}
	if count != 2 {
		return nil, fmt.Errorf("edge endpoints must both belong to this topology")
	}
	id := uuid.New().String()
	now := time.Now().UTC()
	_, err = s.db.Exec(
		`INSERT INTO topology_edges (id, topology_id, from_node, to_node, created_at) VALUES (?, ?, ?, ?, ?)`,
		id, topologyID, req.FromNode, req.ToNode, now,
	)
	if err != nil {
		return nil, fmt.Errorf("create edge: %w", err)
	}
	var e models.TopologyEdge
	err = s.db.QueryRow(
		`SELECT id, topology_id, from_node, to_node, created_at FROM topology_edges WHERE id = ?`, id,
	).Scan(&e.ID, &e.TopologyID, &e.FromNode, &e.ToNode, &e.CreatedAt)
	return &e, err
}

func (s *TopologyStore) DeleteEdge(id string) error {
	res, err := s.db.Exec(`DELETE FROM topology_edges WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *TopologyStore) GetFull(topologyID string) (*models.TopologyFull, error) {
	topo, err := s.GetByID(topologyID)
	if err != nil {
		return nil, err
	}
	nodes, err := s.ListNodes(topologyID)
	if err != nil {
		return nil, err
	}
	edges, err := s.ListEdges(topologyID)
	if err != nil {
		return nil, err
	}
	if nodes == nil {
		nodes = []*models.TopologyNode{}
	}
	if edges == nil {
		edges = []*models.TopologyEdge{}
	}
	return &models.TopologyFull{
		Topology: *topo,
		Nodes:    nodes,
		Edges:    edges,
	}, nil
}

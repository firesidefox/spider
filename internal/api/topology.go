package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	mcppkg "github.com/spiderai/spider/internal/mcp"
	"github.com/spiderai/spider/internal/models"
	"github.com/spiderai/spider/internal/store"
	"gopkg.in/yaml.v3"
)

func listTopologies(app *mcppkg.App, w http.ResponseWriter, r *http.Request) {
	list, err := app.TopologyStore.List()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if list == nil {
		list = []*models.Topology{}
	}
	writeJSON(w, http.StatusOK, list)
}

func createTopology(app *mcppkg.App, w http.ResponseWriter, r *http.Request) {
	var req models.CreateTopologyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	t, err := app.TopologyStore.Create(&req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, t)
}

func getTopology(app *mcppkg.App, w http.ResponseWriter, r *http.Request, id string) {
	full, err := app.TopologyStore.GetFull(id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "topology not found")
		} else {
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, full)
}

func updateTopology(app *mcppkg.App, w http.ResponseWriter, r *http.Request, id string) {
	var req models.UpdateTopologyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	t, err := app.TopologyStore.Update(id, &req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, t)
}

func deleteTopology(app *mcppkg.App, w http.ResponseWriter, r *http.Request, id string) {
	if err := app.TopologyStore.Delete(id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func listTopoGroups(app *mcppkg.App, w http.ResponseWriter, r *http.Request, topoID string) {
	list, err := app.TopologyStore.ListGroups(topoID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if list == nil {
		list = []*models.TopologyGroup{}
	}
	writeJSON(w, http.StatusOK, list)
}

func createTopoGroup(app *mcppkg.App, w http.ResponseWriter, r *http.Request, topoID string) {
	var req models.CreateGroupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	g, err := app.TopologyStore.CreateGroup(topoID, &req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, g)
}

func updateTopoGroup(app *mcppkg.App, w http.ResponseWriter, r *http.Request, gid string) {
	var req models.UpdateGroupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	g, err := app.TopologyStore.UpdateGroup(gid, &req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, g)
}

func deleteTopoGroup(app *mcppkg.App, w http.ResponseWriter, r *http.Request, gid string) {
	if err := app.TopologyStore.DeleteGroup(gid); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func listTopoNodes(app *mcppkg.App, w http.ResponseWriter, r *http.Request, topoID string) {
	list, err := app.TopologyStore.ListNodes(topoID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if list == nil {
		list = []*models.TopologyNode{}
	}
	writeJSON(w, http.StatusOK, list)
}

func createTopoNode(app *mcppkg.App, w http.ResponseWriter, r *http.Request, topoID string) {
	var req models.CreateNodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	n, err := app.TopologyStore.CreateNode(topoID, &req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, n)
}

func updateTopoNode(app *mcppkg.App, w http.ResponseWriter, r *http.Request, nid string) {
	var req models.UpdateNodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	n, err := app.TopologyStore.UpdateNode(nid, &req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, n)
}

func deleteTopoNode(app *mcppkg.App, w http.ResponseWriter, r *http.Request, nid string) {
	if err := app.TopologyStore.DeleteNode(nid); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func listTopoEdges(app *mcppkg.App, w http.ResponseWriter, r *http.Request, topoID string) {
	list, err := app.TopologyStore.ListEdges(topoID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if list == nil {
		list = []*models.TopologyEdge{}
	}
	writeJSON(w, http.StatusOK, list)
}

func createTopoEdge(app *mcppkg.App, w http.ResponseWriter, r *http.Request, topoID string) {
	var req models.CreateEdgeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	e, err := app.TopologyStore.CreateEdge(topoID, &req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, e)
}

func deleteTopoEdge(app *mcppkg.App, w http.ResponseWriter, r *http.Request, eid string) {
	if err := app.TopologyStore.DeleteEdge(eid); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// idFromTopoPath extracts the last path segment
func idFromTopoPath(path string) string {
	parts := strings.Split(strings.TrimSuffix(path, "/"), "/")
	return parts[len(parts)-1]
}

type topoYAML struct {
	Name    string       `yaml:"name"`
	Layers  []layerYAML  `yaml:"layers"`
	Devices []deviceYAML `yaml:"devices"`
}

type layerYAML struct {
	Name  string `yaml:"name"`
	Color string `yaml:"color"`
}

type deviceYAML struct {
	Name     string   `yaml:"name"`
	Layer    string   `yaml:"layer"`
	Role     string   `yaml:"role"`
	IP       string   `yaml:"ip"`
	Upstream []string `yaml:"upstream"`
}

func importTopologyYAML(app *mcppkg.App, w http.ResponseWriter, r *http.Request, topoID string) {
	var payload topoYAML
	if err := yaml.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid YAML: "+err.Error())
		return
	}
	topo, err := app.TopologyStore.GetByID(topoID)
	if err != nil {
		writeError(w, http.StatusNotFound, "topology not found")
		return
	}
	existingGroups, _ := app.TopologyStore.ListGroups(topoID)
	groupByName := map[string]*models.TopologyGroup{}
	for _, g := range existingGroups {
		groupByName[g.Name] = g
	}
	for _, layer := range payload.Layers {
		if _, ok := groupByName[layer.Name]; !ok {
			color := layer.Color
			if color == "" {
				color = "#3b82f6"
			}
			g, err := app.TopologyStore.CreateGroup(topoID, &models.CreateGroupRequest{Name: layer.Name, Color: color})
			if err != nil {
				writeError(w, http.StatusInternalServerError, "create group: "+err.Error())
				return
			}
			groupByName[layer.Name] = g
		}
	}
	existingNodes, _ := app.TopologyStore.ListNodes(topoID)
	nodeByName := map[string]*models.TopologyNode{}
	for _, n := range existingNodes {
		nodeByName[n.Name] = n
	}
	hosts, _ := app.HostStore.List("")
	hostByIP := map[string]string{}
	for _, h := range hosts {
		hostByIP[h.IP] = h.ID
	}
	for _, dev := range payload.Devices {
		grp, ok := groupByName[dev.Layer]
		if !ok {
			writeError(w, http.StatusBadRequest, "unknown layer: "+dev.Layer)
			return
		}
		if _, exists := nodeByName[dev.Name]; !exists {
			n, err := app.TopologyStore.CreateNode(topoID, &models.CreateNodeRequest{
				GroupID: grp.ID, Name: dev.Name, Role: dev.Role, HostID: hostByIP[dev.IP],
			})
			if err != nil {
				writeError(w, http.StatusInternalServerError, "create node: "+err.Error())
				return
			}
			nodeByName[dev.Name] = n
		}
	}
	existingEdges, _ := app.TopologyStore.ListEdges(topoID)
	edgeKey := func(from, to string) string { return from + "->" + to }
	edgeExists := map[string]bool{}
	for _, e := range existingEdges {
		edgeExists[edgeKey(e.FromNode, e.ToNode)] = true
	}
	for _, dev := range payload.Devices {
		toNode, ok := nodeByName[dev.Name]
		if !ok {
			continue
		}
		for _, upName := range dev.Upstream {
			fromNode, ok := nodeByName[upName]
			if !ok {
				continue
			}
			key := edgeKey(fromNode.ID, toNode.ID)
			if !edgeExists[key] {
				_, _ = app.TopologyStore.CreateEdge(topoID, &models.CreateEdgeRequest{
					FromNode: fromNode.ID, ToNode: toNode.ID,
				})
				edgeExists[key] = true
			}
		}
	}
	full, err := app.TopologyStore.GetFull(topoID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	_ = topo
	writeJSON(w, http.StatusOK, full)
}

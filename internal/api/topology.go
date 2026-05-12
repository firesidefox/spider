package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	mcppkg "github.com/spiderai/spider/internal/mcp"
	"github.com/spiderai/spider/internal/models"
	"github.com/spiderai/spider/internal/store"
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

// importTopologyYAML is implemented in topology_import.go (Task 6)
func importTopologyYAML(app *mcppkg.App, w http.ResponseWriter, r *http.Request, topoID string) {
	writeError(w, http.StatusNotImplemented, "not implemented")
}

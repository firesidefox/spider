package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/spiderai/spider/internal/config"
	mcppkg "github.com/spiderai/spider/internal/mcp"
)

func listRules(app *mcppkg.App, w http.ResponseWriter, _ *http.Request) {
	rules := app.Config.Agent.Rules
	if rules == nil {
		rules = []config.RuleConfig{}
	}
	writeJSON(w, http.StatusOK, rules)
}

func addRule(app *mcppkg.App, w http.ResponseWriter, r *http.Request) {
	var rc config.RuleConfig
	if err := json.NewDecoder(r.Body).Decode(&rc); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if err := validateRule(rc); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	app.Config.Agent.Rules = append(app.Config.Agent.Rules, rc)
	if err := saveAndReload(app); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, rc)
}

func updateRule(app *mcppkg.App, w http.ResponseWriter, r *http.Request, idx int) {
	if idx < 0 || idx >= len(app.Config.Agent.Rules) {
		writeError(w, http.StatusNotFound, "rule index out of range")
		return
	}
	var rc config.RuleConfig
	if err := json.NewDecoder(r.Body).Decode(&rc); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if err := validateRule(rc); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	app.Config.Agent.Rules[idx] = rc
	if err := saveAndReload(app); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, rc)
}

func deleteRule(app *mcppkg.App, w http.ResponseWriter, _ *http.Request, idx int) {
	if idx < 0 || idx >= len(app.Config.Agent.Rules) {
		writeError(w, http.StatusNotFound, "rule index out of range")
		return
	}
	app.Config.Agent.Rules = append(
		app.Config.Agent.Rules[:idx],
		app.Config.Agent.Rules[idx+1:]...,
	)
	if err := saveAndReload(app); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func listBuiltinRules(app *mcppkg.App, w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, app.Classifier.BuiltinRules())
}

func validateRule(r config.RuleConfig) error {
	if strings.TrimSpace(r.Pattern) == "" {
		return fmt.Errorf("pattern must not be empty")
	}
	if _, err := regexp.Compile(r.Pattern); err != nil {
		return fmt.Errorf("invalid regex pattern: %w", err)
	}
	switch r.Level {
	case "L1", "L2", "L3", "L4":
	default:
		return fmt.Errorf("level must be L1, L2, L3, or L4")
	}
	return nil
}

func saveAndReload(app *mcppkg.App) error {
	if err := saveConfig(app); err != nil {
		return err
	}
	app.Classifier.Reload(app.Config.Agent.Rules)
	return nil
}

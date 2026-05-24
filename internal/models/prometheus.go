package models

import "time"

type PrometheusAuthType string

const (
	PrometheusAuthNone   PrometheusAuthType = "none"
	PrometheusAuthBasic  PrometheusAuthType = "basic"
	PrometheusAuthBearer PrometheusAuthType = "bearer"
)

type PrometheusSource struct {
	ID                string             `json:"id"`
	Name              string             `json:"name"`
	BaseURL           string             `json:"base_url"`
	TimeoutSeconds    int                `json:"timeout_seconds"`
	AuthType          PrometheusAuthType `json:"auth_type"`
	Username          string             `json:"username,omitempty"`
	EncryptedPassword string             `json:"-"`
	EncryptedToken    string             `json:"-"`
	SkipTLSVerify     bool               `json:"skip_tls_verify"`
	CreatedAt         time.Time          `json:"created_at"`
	UpdatedAt         time.Time          `json:"updated_at"`
}

type PrometheusScopeType string

const (
	ScopeTopologyLayer PrometheusScopeType = "topology_layer"
	ScopeHost          PrometheusScopeType = "host"
)

type PrometheusBinding struct {
	ID         string              `json:"id"`
	SourceID   string              `json:"source_id"`
	ScopeType  PrometheusScopeType `json:"scope_type"`
	TopologyID string              `json:"topology_id,omitempty"`
	Layer      string              `json:"layer,omitempty"`
	HostID     string              `json:"host_id,omitempty"`
	CreatedAt  time.Time           `json:"created_at"`
}

type AddPrometheusSourceRequest struct {
	Name           string             `json:"name"`
	BaseURL        string             `json:"base_url"`
	TimeoutSeconds int                `json:"timeout_seconds"`
	AuthType       PrometheusAuthType `json:"auth_type"`
	Username       string             `json:"username"`
	Password       string             `json:"password"`
	Token          string             `json:"token"`
	SkipTLSVerify  bool               `json:"skip_tls_verify"`
}

type UpdatePrometheusSourceRequest struct {
	Name           *string             `json:"name"`
	BaseURL        *string             `json:"base_url"`
	TimeoutSeconds *int                `json:"timeout_seconds"`
	AuthType       *PrometheusAuthType `json:"auth_type"`
	Username       *string             `json:"username"`
	Password       *string             `json:"password"`
	Token          *string             `json:"token"`
	SkipTLSVerify  *bool               `json:"skip_tls_verify"`
}

type AddPrometheusBindingRequest struct {
	SourceID   string              `json:"source_id"`
	ScopeType  PrometheusScopeType `json:"scope_type"`
	TopologyID string              `json:"topology_id"`
	Layer      string              `json:"layer"`
	HostID     string              `json:"host_id"`
}

package models

import "time"

type AccessFaceType string

const (
	FaceSSH     AccessFaceType = "ssh"
	FaceRESTAPI AccessFaceType = "restapi"
)

type SSHAuthType string

const (
	SSHAuthPassword    SSHAuthType = "password"
	SSHAuthKey         SSHAuthType = "key"
	SSHAuthKeyPassword SSHAuthType = "key_password"
)

type RESTAuthType string

const (
	RESTAuthBearer    RESTAuthType = "bearer"
	RESTAuthBasic     RESTAuthType = "basic"
	RESTAuthAPIKey    RESTAuthType = "apikey"
	RESTAuthNone      RESTAuthType = "none"
	RESTAuthHMACAKSK  RESTAuthType = "hmac_aksk"
)

type KnowledgeSourceRef struct {
	Type  string `json:"type"` // "group" | "doc"
	ID    int    `json:"id"`
}

type AccessFace struct {
	ID               string               `json:"id"`
	HostID           string               `json:"host_id"`
	Type             AccessFaceType       `json:"type"`
	IP               string               `json:"ip"`
	Port             int                  `json:"port"`
	Username         string               `json:"username,omitempty"`
	SSHAuthType      SSHAuthType          `json:"ssh_auth_type,omitempty"`
	EncryptedCred    string               `json:"-"`
	EncryptedPass    string               `json:"-"`
	SSHKeyID         string               `json:"ssh_key_id,omitempty"`
	SSHLegacy        bool                 `json:"ssh_legacy,omitempty"`
	SSHLoginInput    string               `json:"ssh_login_input,omitempty"`
	BaseURL          string               `json:"base_url,omitempty"`
	RESTAuthType     RESTAuthType         `json:"rest_auth_type,omitempty"`
	RESTUsername     string               `json:"rest_username,omitempty"`
	HeaderName       string               `json:"header_name,omitempty"`
	HMACAlgo         string               `json:"hmac_algo,omitempty"`
	KnowledgeSources []KnowledgeSourceRef `json:"knowledge_sources"`
	ProbePort     int                  `json:"probe_port,omitempty"`
	ProbeInterval int                  `json:"probe_interval,omitempty"`
	CreatedAt        time.Time            `json:"created_at"`
	UpdatedAt        time.Time            `json:"updated_at"`
}

type FingerprintStatus string

const (
	FingerprintOK         FingerprintStatus = "ok"
	FingerprintChanged    FingerprintStatus = "changed"
	FingerprintUnverified FingerprintStatus = "unverified"
)

type Fingerprint struct {
	HostID        string            `json:"host_id"`
	SSHHostKey    string            `json:"ssh_host_key,omitempty"`
	SystemVersion string            `json:"system_version,omitempty"`
	HardwareID    string            `json:"hardware_id,omitempty"`
	APISignature  string            `json:"api_signature,omitempty"`
	Status        FingerprintStatus `json:"status"`
	SnapshotAt    *time.Time        `json:"snapshot_at,omitempty"`
}

type Memory struct {
	ID        int       `json:"id"`
	HostID    string    `json:"host_id"`
	Content   string    `json:"content"`
	CreatedBy string    `json:"created_by"` // "user" | "agent"
	CreatedAt time.Time `json:"created_at"`
}

type Host struct {
	ID               string               `json:"id"`
	Name             string               `json:"name"`
	IP               string               `json:"ip"`
	Notes            string               `json:"notes,omitempty"`
	Tags             []string             `json:"tags"`
	Vendor           string               `json:"vendor,omitempty"`
	ProductName      string               `json:"product_name,omitempty"`
	ProductVersion   string               `json:"product_version,omitempty"`
	CreatedAt        time.Time            `json:"created_at"`
	UpdatedAt        time.Time            `json:"updated_at"`
	KnowledgeSources []KnowledgeSourceRef `json:"knowledge_sources,omitempty"`
	AccessFaces      []AccessFace         `json:"access_faces,omitempty"`
	Fingerprint      *Fingerprint         `json:"fingerprint,omitempty"`
	Memories         []Memory             `json:"memories,omitempty"`
}

type AddHostRequest struct {
	Name           string   `json:"name"`
	IP             string   `json:"ip"`
	Notes          string   `json:"notes"`
	Tags           []string `json:"tags"`
	Vendor         string   `json:"vendor"`
	ProductName    string   `json:"product_name"`
	ProductVersion string   `json:"product_version"`
}

type UpdateHostRequest struct {
	Name           *string  `json:"name"`
	IP             *string  `json:"ip"`
	Notes          *string  `json:"notes"`
	Tags           []string `json:"tags"`
	Vendor         *string  `json:"vendor"`
	ProductName    *string  `json:"product_name"`
	ProductVersion *string  `json:"product_version"`
}

type AddAccessFaceRequest struct {
	Type             AccessFaceType       `json:"type"`
	IP               string               `json:"ip"`
	Port             int                  `json:"port"`
	Username         string               `json:"username"`
	SSHAuthType      SSHAuthType          `json:"ssh_auth_type"`
	Credential       string               `json:"credential"`
	Passphrase       string               `json:"passphrase"`
	SSHKeyID         string               `json:"ssh_key_id"`
	SSHLegacy        bool                 `json:"ssh_legacy"`
	SSHLoginInput    string               `json:"ssh_login_input"`
	BaseURL          string               `json:"base_url"`
	RESTAuthType     RESTAuthType         `json:"rest_auth_type"`
	RESTUsername     string               `json:"rest_username"`
	HeaderName       string               `json:"header_name"`
	HMACAlgo         string               `json:"hmac_algo"`
	KnowledgeSources []KnowledgeSourceRef `json:"knowledge_sources"`
	ProbePort        int                  `json:"probe_port,omitempty"`
	ProbeInterval    int                  `json:"probe_interval,omitempty"`
}

type UpdateAccessFaceRequest struct {
	IP               *string              `json:"ip"`
	Port             *int                 `json:"port"`
	Username         *string              `json:"username"`
	SSHAuthType      *SSHAuthType         `json:"ssh_auth_type"`
	Credential       *string              `json:"credential"`
	Passphrase       *string              `json:"passphrase"`
	SSHKeyID         *string              `json:"ssh_key_id"`
	SSHLegacy        *bool                `json:"ssh_legacy"`
	SSHLoginInput    *string              `json:"ssh_login_input"`
	BaseURL          *string              `json:"base_url"`
	RESTAuthType     *RESTAuthType        `json:"rest_auth_type"`
	RESTUsername     *string              `json:"rest_username"`
	HeaderName       *string              `json:"header_name"`
	HMACAlgo         *string              `json:"hmac_algo"`
	KnowledgeSources []KnowledgeSourceRef `json:"knowledge_sources"`
	ProbePort        *int                 `json:"probe_port,omitempty"`
	ProbeInterval    *int                 `json:"probe_interval,omitempty"`
}

type AddMemoryRequest struct {
	Content   string `json:"content"`
	CreatedBy string `json:"created_by"`
}

type UpdateFingerprintRequest struct {
	SSHHostKey    string `json:"ssh_host_key"`
	SystemVersion string `json:"system_version"`
	HardwareID    string `json:"hardware_id"`
	APISignature  string `json:"api_signature"`
}

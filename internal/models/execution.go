package models

import "time"

// ExecutionLog 记录每次命令执行的历史。
type ExecutionLog struct {
	ID             string    `json:"id"`
	HostID         string    `json:"host_id"`
	HostName       string    `json:"host_name,omitempty"` // JOIN 填充，不存库
	Command        string    `json:"command"`
	Stdout         string    `json:"stdout"`
	Stderr         string    `json:"stderr"`
	ExitCode       int       `json:"exit_code"`
	DurationMs     int64     `json:"duration_ms"`
	TriggeredBy    string    `json:"triggered_by"` // mcp | cli
	UserID         string    `json:"user_id,omitempty"`
	RiskLevel      string    `json:"risk_level,omitempty"`
	PermissionMode string    `json:"permission_mode,omitempty"`
	ApprovalID     string    `json:"approval_id,omitempty"`
	ApprovedBy     string    `json:"approved_by,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
}

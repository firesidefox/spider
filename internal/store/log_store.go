package store

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/spiderai/spider/internal/models"
)

// LogStore 提供执行日志的存储操作。
type LogStore struct {
	db *sql.DB
}

// NewLogStore 创建一个新的 LogStore。
func NewLogStore(db *sql.DB) *LogStore {
	return &LogStore{db: db}
}

// Save 保存一条执行日志。
func (s *LogStore) Save(log *models.ExecutionLog) error {
	if log.ID == "" {
		log.ID = uuid.New().String()
	}
	if log.CreatedAt.IsZero() {
		log.CreatedAt = time.Now().UTC()
	}
	_, err := s.db.Exec(
		`INSERT INTO execution_logs (id, host_id, command, stdout, stderr, exit_code,
		 duration_ms, triggered_by, user_id, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		log.ID, log.HostID, log.Command, log.Stdout, log.Stderr,
		log.ExitCode, log.DurationMs, log.TriggeredBy, log.UserID, log.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("保存执行日志失败: %w", err)
	}
	return nil
}

// List 查询执行历史，可按 hostID 过滤。
func (s *LogStore) List(hostID string, limit, offset int) ([]*models.ExecutionLog, error) {
	if limit <= 0 {
		limit = 20
	}
	var rows *sql.Rows
	var err error
	if hostID == "" {
		rows, err = s.db.Query(
			`SELECT l.id, l.host_id, h.name, l.command, l.stdout, l.stderr,
			 l.exit_code, l.duration_ms, l.triggered_by, l.created_at
			 FROM execution_logs l
			 LEFT JOIN hosts h ON h.id = l.host_id
			 ORDER BY l.created_at DESC LIMIT ? OFFSET ?`,
			limit, offset,
		)
	} else {
		rows, err = s.db.Query(
			`SELECT l.id, l.host_id, h.name, l.command, l.stdout, l.stderr,
			 l.exit_code, l.duration_ms, l.triggered_by, l.created_at
			 FROM execution_logs l
			 LEFT JOIN hosts h ON h.id = l.host_id
			 WHERE l.host_id = ?
			 ORDER BY l.created_at DESC LIMIT ? OFFSET ?`,
			hostID, limit, offset,
		)
	}
	if err != nil {
		return nil, fmt.Errorf("查询执行历史失败: %w", err)
	}
	defer rows.Close()

	var logs []*models.ExecutionLog
	for rows.Next() {
		var log models.ExecutionLog
		var hostName sql.NullString
		err := rows.Scan(
			&log.ID, &log.HostID, &hostName, &log.Command,
			&log.Stdout, &log.Stderr, &log.ExitCode,
			&log.DurationMs, &log.TriggeredBy, &log.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("扫描执行日志失败: %w", err)
		}
		log.HostName = hostName.String
		logs = append(logs, &log)
	}
	return logs, rows.Err()
}

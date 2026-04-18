package store

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/spiderai/spider/internal/models"
)

var ErrInvalidCredentials = errors.New("invalid credentials")
var ErrUserNotFound = errors.New("user not found")

// UserStore 提供用户的 CRUD 操作。
type UserStore struct {
	db *sql.DB
}

// NewUserStore 创建一个新的 UserStore。
func NewUserStore(db *sql.DB) *UserStore {
	return &UserStore{db: db}
}

// Create 创建用户（password 为明文，内部 bcrypt 哈希）。
func (s *UserStore) Create(username, password string, role models.Role) (*models.User, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return nil, fmt.Errorf("哈希密码失败: %w", err)
	}
	id := uuid.New().String()
	now := time.Now().UTC()
	_, err = s.db.Exec(
		`INSERT INTO users (id, username, password, role, enabled, created_at)
		 VALUES (?, ?, ?, ?, 1, ?)`,
		id, username, string(hash), string(role), now,
	)
	if err != nil {
		return nil, fmt.Errorf("创建用户失败: %w", err)
	}
	return s.GetByID(id)
}

// GetByUsername 按用户名查询。
func (s *UserStore) GetByUsername(username string) (*models.User, error) {
	row := s.db.QueryRow(
		`SELECT id, username, password, role, enabled, created_at, last_login
		 FROM users WHERE username = ?`, username,
	)
	return scanUser(row)
}

// GetByID 按 ID 查询。
func (s *UserStore) GetByID(id string) (*models.User, error) {
	row := s.db.QueryRow(
		`SELECT id, username, password, role, enabled, created_at, last_login
		 FROM users WHERE id = ?`, id,
	)
	return scanUser(row)
}

// List 列出所有用户。
func (s *UserStore) List() ([]*models.User, error) {
	rows, err := s.db.Query(
		`SELECT id, username, password, role, enabled, created_at, last_login
		 FROM users ORDER BY username`,
	)
	if err != nil {
		return nil, fmt.Errorf("查询用户列表失败: %w", err)
	}
	defer rows.Close()
	var users []*models.User
	for rows.Next() {
		u, err := scanUserRows(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

// Update 更新用户字段（role/enabled/password 均可选，nil 表示不更新）。
func (s *UserStore) Update(id string, role *models.Role, enabled *bool, password *string) (*models.User, error) {
	u, err := s.GetByID(id)
	if err != nil {
		return nil, err
	}
	if role != nil {
		u.Role = *role
	}
	if enabled != nil {
		u.Enabled = *enabled
	}
	if password != nil {
		hash, err := bcrypt.GenerateFromPassword([]byte(*password), 12)
		if err != nil {
			return nil, fmt.Errorf("哈希密码失败: %w", err)
		}
		u.Password = string(hash)
	}
	_, err = s.db.Exec(
		`UPDATE users SET role=?, enabled=?, password=? WHERE id=?`,
		string(u.Role), boolToInt(u.Enabled), u.Password, id,
	)
	if err != nil {
		return nil, fmt.Errorf("更新用户失败: %w", err)
	}
	return s.GetByID(id)
}

// Delete 删除用户。
func (s *UserStore) Delete(id string) error {
	res, err := s.db.Exec(`DELETE FROM users WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("删除用户失败: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrUserNotFound
	}
	return nil
}

// Authenticate 验证用户名密码，返回用户。
func (s *UserStore) Authenticate(username, password string) (*models.User, error) {
	u, err := s.GetByUsername(username)
	if err != nil {
		return nil, ErrInvalidCredentials
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(password)); err != nil {
		return nil, ErrInvalidCredentials
	}
	return u, nil
}

// UpdateLastLogin 更新最后登录时间。
func (s *UserStore) UpdateLastLogin(id string) error {
	now := time.Now().UTC()
	_, err := s.db.Exec(`UPDATE users SET last_login=? WHERE id=?`, now, id)
	return err
}

// EnsureDefaultAdmin 若 users 表为空，创建默认 admin 并打印密码到 stdout。
func (s *UserStore) EnsureDefaultAdmin() error {
	var count int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Errorf("生成随机密码失败: %w", err)
	}
	password := hex.EncodeToString(buf)
	if _, err := s.Create("admin", password, models.RoleAdmin); err != nil {
		return fmt.Errorf("创建默认 admin 失败: %w", err)
	}
	fmt.Printf("Spider: default admin created — username: admin, password: %s\n", password)
	return nil
}

// scanUser 从 *sql.Row 扫描一个 User。
func scanUser(row *sql.Row) (*models.User, error) {
	var u models.User
	var enabled int
	var role string
	err := row.Scan(&u.ID, &u.Username, &u.Password, &role, &enabled, &u.CreatedAt, &u.LastLogin)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("扫描用户数据失败: %w", err)
	}
	u.Role = models.Role(role)
	u.Enabled = enabled != 0
	return &u, nil
}

// scanUserRows 从 *sql.Rows 扫描一个 User。
func scanUserRows(rows *sql.Rows) (*models.User, error) {
	var u models.User
	var enabled int
	var role string
	err := rows.Scan(&u.ID, &u.Username, &u.Password, &role, &enabled, &u.CreatedAt, &u.LastLogin)
	if err != nil {
		return nil, fmt.Errorf("扫描用户数据失败: %w", err)
	}
	u.Role = models.Role(role)
	u.Enabled = enabled != 0
	return &u, nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

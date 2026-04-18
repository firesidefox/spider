package ssh

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"time"

	gossh "golang.org/x/crypto/ssh"

	"github.com/spiderai/spider/internal/models"
	"github.com/spiderai/spider/internal/store"
)

// ExecResult 是命令执行结果。
type ExecResult struct {
	Stdout     string
	Stderr     string
	ExitCode   int
	Duration   time.Duration
}

// Client 封装一个 SSH 连接。
type Client struct {
	conn *gossh.Client
	host *models.Host
}

// NewClient 根据主机信息建立 SSH 连接（支持密码、私钥）。
func NewClient(host *models.Host, hs *store.HostStore) (*Client, error) {
	credential, passphrase, err := hs.DecryptCredential(host)
	if err != nil {
		return nil, err
	}

	authMethods, err := buildAuthMethods(host.AuthType, credential, passphrase)
	if err != nil {
		return nil, err
	}

	cfg := &gossh.ClientConfig{
		User:            host.Username,
		Auth:            authMethods,
		HostKeyCallback: gossh.InsecureIgnoreHostKey(),
		Timeout:         15 * time.Second,
	}

	addr := fmt.Sprintf("%s:%d", host.IP, host.Port)
	conn, err := gossh.Dial("tcp", addr, cfg)
	if err != nil {
		return nil, fmt.Errorf("SSH 连接 %s 失败: %w", addr, err)
	}
	return &Client{conn: conn, host: host}, nil
}

// buildAuthMethods 根据认证类型构建 SSH 认证方法列表。
func buildAuthMethods(authType models.AuthType, credential, passphrase string) ([]gossh.AuthMethod, error) {
	switch authType {
	case models.AuthPassword:
		return []gossh.AuthMethod{gossh.Password(credential)}, nil

	case models.AuthKey:
		signer, err := gossh.ParsePrivateKey([]byte(credential))
		if err != nil {
			return nil, fmt.Errorf("解析私钥失败: %w", err)
		}
		return []gossh.AuthMethod{gossh.PublicKeys(signer)}, nil

	case models.AuthKeyPassword:
		signer, err := gossh.ParsePrivateKeyWithPassphrase([]byte(credential), []byte(passphrase))
		if err != nil {
			return nil, fmt.Errorf("解析带 passphrase 的私钥失败: %w", err)
		}
		return []gossh.AuthMethod{gossh.PublicKeys(signer)}, nil

	default:
		return nil, fmt.Errorf("不支持的认证类型: %s", authType)
	}
}

// Execute 在远程主机上执行命令，返回 stdout/stderr/exit_code。
func (c *Client) Execute(ctx context.Context, command string) (*ExecResult, error) {
	session, err := c.conn.NewSession()
	if err != nil {
		return nil, fmt.Errorf("创建 SSH session 失败: %w", err)
	}
	defer session.Close()

	var stdout, stderr bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = &stderr

	start := time.Now()

	// 通过 context 实现超时控制
	done := make(chan error, 1)
	go func() {
		done <- session.Run(command)
	}()

	var runErr error
	select {
	case <-ctx.Done():
		session.Signal(gossh.SIGKILL)
		return nil, fmt.Errorf("命令执行超时或被取消: %w", ctx.Err())
	case runErr = <-done:
	}

	duration := time.Since(start)
	exitCode := 0
	if runErr != nil {
		if exitErr, ok := runErr.(*gossh.ExitError); ok {
			exitCode = exitErr.ExitStatus()
		} else {
			return nil, fmt.Errorf("执行命令失败: %w", runErr)
		}
	}

	return &ExecResult{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: exitCode,
		Duration: duration,
	}, nil
}

// CheckConnectivity 测试 SSH 连通性，返回延迟。
func CheckConnectivity(host *models.Host, hs *store.HostStore) (latency time.Duration, err error) {
	start := time.Now()
	// 先测试 TCP 连通性
	addr := fmt.Sprintf("%s:%d", host.IP, host.Port)
	conn, err := net.DialTimeout("tcp", addr, 10*time.Second)
	if err != nil {
		return 0, fmt.Errorf("TCP 连接失败: %w", err)
	}
	conn.Close()
	tcpLatency := time.Since(start)

	// 再测试 SSH 握手
	client, err := NewClient(host, hs)
	if err != nil {
		return tcpLatency, fmt.Errorf("SSH 握手失败: %w", err)
	}
	client.Close()
	return time.Since(start), nil
}

// Close 关闭 SSH 连接。
func (c *Client) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

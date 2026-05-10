package ssh

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"time"

	gossh "golang.org/x/crypto/ssh"

	"github.com/spiderai/spider/internal/logger"
	"github.com/spiderai/spider/internal/models"
	"github.com/spiderai/spider/internal/store"
)

// ExecResult 是命令执行结果。
type ExecResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
	Duration time.Duration
}

// Client 封装一个 SSH 连接。
type Client struct {
	conn *gossh.Client
	face *models.AccessFace
}

func newSSHConfig(face *models.AccessFace, authMethods []gossh.AuthMethod) *gossh.ClientConfig {
	cfg := &gossh.ClientConfig{
		User:            face.Username,
		Auth:            authMethods,
		HostKeyCallback: gossh.InsecureIgnoreHostKey(),
		Timeout:         15 * time.Second,
	}
	if face.SSHLegacy {
		cfg.Config = gossh.Config{
			KeyExchanges: []string{
				"curve25519-sha256", "curve25519-sha256@libssh.org",
				"ecdh-sha2-nistp256", "ecdh-sha2-nistp384", "ecdh-sha2-nistp521",
				"diffie-hellman-group14-sha256", "diffie-hellman-group14-sha1",
				"diffie-hellman-group1-sha1",
			},
			Ciphers: []string{
				"aes128-gcm@openssh.com", "chacha20-poly1305@openssh.com",
				"aes128-ctr", "aes192-ctr", "aes256-ctr",
				"aes128-cbc", "3des-cbc",
			},
		}
	}
	return cfg
}

// NewClientFromFace 根据 AccessFace 建立 SSH 连接（解密凭据）。
func NewClientFromFace(face *models.AccessFace, afs *store.AccessFaceStore, ks *store.SSHKeyStore) (*Client, error) {
	var credential, passphrase string
	var err error
	if face.SSHKeyID != "" && ks != nil {
		key, kerr := ks.GetByID(face.SSHKeyID)
		if kerr != nil {
			return nil, fmt.Errorf("获取 SSH key 失败: %w", kerr)
		}
		credential, passphrase, err = ks.DecryptKey(key)
	} else {
		credential, passphrase, err = afs.DecryptCredential(face)
	}
	if err != nil {
		return nil, err
	}
	return NewClientWithCredential(face, credential, passphrase)
}

// NewClientWithCredential 根据预解密的凭据建立 SSH 连接。
func NewClientWithCredential(face *models.AccessFace, credential, passphrase string) (*Client, error) {
	authMethods, err := buildAuthMethods(face.SSHAuthType, credential, passphrase)
	if err != nil {
		return nil, err
	}
	addr := fmt.Sprintf("%s:%d", face.IP, face.Port)
	logger.Global().Debug().Str("host", face.IP).Str("user", face.Username).Msg("ssh connecting")
	conn, err := gossh.Dial("tcp", addr, newSSHConfig(face, authMethods))
	if err != nil {
		logger.Global().Error().Err(err).Str("host", face.IP).Msg("ssh connect failed")
		return nil, fmt.Errorf("SSH 连接 %s 失败: %w", addr, err)
	}
	logger.Global().Info().Str("host", face.IP).Msg("ssh connected")
	return &Client{conn: conn, face: face}, nil
}

// buildAuthMethods 根据认证类型构建 SSH 认证方法列表。
func buildAuthMethods(authType models.SSHAuthType, credential, passphrase string) ([]gossh.AuthMethod, error) {
	switch authType {
	case models.SSHAuthPassword:
		return []gossh.AuthMethod{gossh.Password(credential)}, nil
	case models.SSHAuthKey:
		signer, err := gossh.ParsePrivateKey([]byte(credential))
		if err != nil {
			return nil, fmt.Errorf("解析私钥失败: %w", err)
		}
		return []gossh.AuthMethod{gossh.PublicKeys(signer)}, nil
	case models.SSHAuthKeyPassword:
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
	log := logger.FromContext(ctx)
	log.Debug().Str("host", c.face.IP).Str("cmd", command).Msg("ssh execute start")

	session, err := c.conn.NewSession()
	if err != nil {
		return nil, fmt.Errorf("创建 SSH session 失败: %w", err)
	}
	defer session.Close()

	var stdout, stderr bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = &stderr

	start := time.Now()
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
			log.Error().Err(runErr).Str("host", c.face.IP).Str("cmd", command).Msg("ssh execute error")
			return nil, fmt.Errorf("执行命令失败: %w", runErr)
		}
	}

	log.Debug().Str("host", c.face.IP).Int("exit_code", exitCode).Int64("duration_ms", duration.Milliseconds()).Msg("ssh execute done")
	return &ExecResult{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: exitCode,
		Duration: duration,
	}, nil
}

// CheckConnectivity 测试 SSH 连通性，返回延迟。
func CheckConnectivity(face *models.AccessFace, afs *store.AccessFaceStore, ks *store.SSHKeyStore) (latency time.Duration, err error) {
	start := time.Now()
	addr := fmt.Sprintf("%s:%d", face.IP, face.Port)
	conn, err := net.DialTimeout("tcp", addr, 10*time.Second)
	if err != nil {
		return 0, fmt.Errorf("TCP 连接失败: %w", err)
	}
	conn.Close()
	tcpLatency := time.Since(start)

	var credential, passphrase string
	if face.SSHKeyID != "" && ks != nil {
		key, kerr := ks.GetByID(face.SSHKeyID)
		if kerr != nil {
			return tcpLatency, fmt.Errorf("获取 SSH key 失败: %w", kerr)
		}
		credential, passphrase, err = ks.DecryptKey(key)
	} else {
		credential, passphrase, err = afs.DecryptCredential(face)
	}
	if err != nil {
		return tcpLatency, fmt.Errorf("解密凭据失败: %w", err)
	}

	client, err := NewClientWithCredential(face, credential, passphrase)
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

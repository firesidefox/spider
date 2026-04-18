package ssh

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	gossh "golang.org/x/crypto/ssh"
)

// Upload 通过 SCP 协议将本地文件上传到远程主机。
func (c *Client) Upload(ctx context.Context, localPath, remotePath string) error {
	session, err := c.conn.NewSession()
	if err != nil {
		return fmt.Errorf("创建 session 失败: %w", err)
	}
	defer session.Close()

	f, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("打开本地文件失败: %w", err)
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		return fmt.Errorf("获取文件信息失败: %w", err)
	}

	w, err := session.StdinPipe()
	if err != nil {
		return fmt.Errorf("获取 stdin pipe 失败: %w", err)
	}

	errCh := make(chan error, 1)
	go func() {
		defer w.Close()
		fmt.Fprintf(w, "C0644 %d %s\n", stat.Size(), filepath.Base(remotePath))
		if _, err := io.Copy(w, f); err != nil {
			errCh <- fmt.Errorf("传输文件内容失败: %w", err)
			return
		}
		fmt.Fprint(w, "\x00")
		errCh <- nil
	}()

	remoteDir := filepath.Dir(remotePath)
	if err := session.Run(fmt.Sprintf("scp -t %s", remoteDir)); err != nil {
		return fmt.Errorf("远程 scp 执行失败: %w", err)
	}

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Download 从远程主机下载文件到本地。
func (c *Client) Download(ctx context.Context, remotePath, localPath string) error {
	session, err := c.conn.NewSession()
	if err != nil {
		return fmt.Errorf("创建 session 失败: %w", err)
	}
	defer session.Close()

	if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
		return fmt.Errorf("创建本地目录失败: %w", err)
	}

	r, err := session.StdoutPipe()
	if err != nil {
		return fmt.Errorf("获取 stdout pipe 失败: %w", err)
	}

	if err := session.Start(fmt.Sprintf("cat %q", remotePath)); err != nil {
		return fmt.Errorf("启动远程 cat 失败: %w", err)
	}

	f, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("创建本地文件失败: %w", err)
	}
	defer f.Close()

	done := make(chan error, 1)
	go func() {
		_, err := io.Copy(f, r)
		done <- err
	}()

	select {
	case err := <-done:
		if err != nil {
			return fmt.Errorf("写入本地文件失败: %w", err)
		}
	case <-ctx.Done():
		session.Signal(gossh.SIGKILL)
		return ctx.Err()
	}

	return session.Wait()
}

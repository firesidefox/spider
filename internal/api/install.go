package api

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"text/template"
)

const installScriptTmpl = `#!/bin/sh
SPIDER_URL="{{.BaseURL}}"
SKILLS_DIR="$HOME/.claude/plugins/spider"
SETTINGS="$HOME/.claude/settings.json"

set -e

RED='\033[31m'; GREEN='\033[32m'; YELLOW='\033[33m'
BLUE='\033[34m'; DIM='\033[2m'; RESET='\033[0m'

h1()      { printf "\n${BLUE}══ %s ══${RESET}\n" "$*"; }
step()    { printf "  ${BLUE}▶ %s...${RESET}\n" "$*"; }
success() { printf "  ${GREEN}✔ %s${RESET}\n" "$*"; }
warn()    { printf "  ${YELLOW}⚠ %s${RESET}\n" "$*"; }
error()   { printf "  ${RED}✖ %s${RESET}\n" "$*" >&2; }

h1 "Spider 安装"

step "下载 Skills"
mkdir -p "$SKILLS_DIR"
curl -fsSL "$SPIDER_URL/api/v1/install/skills.tar.gz" | tar -xz -C "$SKILLS_DIR"
success "Skills 已安装到 $SKILLS_DIR"

step "注册 MCP 服务器"
if command -v claude >/dev/null 2>&1; then
  claude mcp add --transport http spider "$SPIDER_URL/mcp"
  success "已通过 claude CLI 注册"
elif command -v node >/dev/null 2>&1; then
  node -e "
    const fs=require('fs'),p='$SETTINGS';
    const c=fs.existsSync(p)?JSON.parse(fs.readFileSync(p,'utf8')):{};
    c.mcpServers=Object.assign({},c.mcpServers,{spider:{type:'http',url:'$SPIDER_URL/mcp'}});
    fs.writeFileSync(p,JSON.stringify(c,null,2));
  "
  success "已写入 $SETTINGS"
elif command -v python3 >/dev/null 2>&1; then
  python3 -c "
import json,os
p='$SETTINGS'
c=json.load(open(p)) if os.path.exists(p) else {}
c.setdefault('mcpServers',{})['spider']={'type':'http','url':'$SPIDER_URL/mcp'}
json.dump(c,open(p,'w'),indent=2)
  "
  success "已写入 $SETTINGS"
else
  error "需要 claude CLI、node 或 python3"; exit 1
fi

h1 "安装完成 — 重启 Claude Code 即可使用 Spider"
`

var installTmpl = template.Must(template.New("install").Parse(installScriptTmpl))

const serverInstallScript = `#!/bin/sh
# Spider 服务端安装脚本
# 在目标 Linux 服务器上运行：curl -fsSL {{.BaseURL}}/server-install.sh | sh
SPIDER_URL="{{.BaseURL}}"
INSTALL_DIR="/usr/local/bin"
DATA_DIR="/var/lib/spider"
SERVICE_USER="spider"
SYSTEMD_UNIT="/etc/systemd/system/spider.service"

set -e

RED='\033[31m'; GREEN='\033[32m'; YELLOW='\033[33m'
BLUE='\033[34m'; DIM='\033[2m'; RESET='\033[0m'

h1()      { printf "\n${BLUE}══ %s ══${RESET}\n" "$*"; }
step()    { printf "  ${BLUE}▶ %s...${RESET}\n" "$*"; }
success() { printf "  ${GREEN}✔ %s${RESET}\n" "$*"; }
warn()    { printf "  ${YELLOW}⚠ %s${RESET}\n" "$*"; }
error()   { printf "  ${RED}✖ %s${RESET}\n" "$*" >&2; }

# 检测架构
ARCH=$(uname -m)
case "$ARCH" in
  x86_64)  ARCH_SUFFIX="amd64" ;;
  aarch64) ARCH_SUFFIX="arm64" ;;
  *) error "不支持的架构: $ARCH"; exit 1 ;;
esac

h1 "Spider 服务端安装"

step "下载 spider 二进制 (linux/$ARCH_SUFFIX)"
curl -fsSL "$SPIDER_URL/api/v1/install/spider?arch=$ARCH_SUFFIX" -o /tmp/spider
chmod +x /tmp/spider
mv /tmp/spider "$INSTALL_DIR/spider"
success "spider → $INSTALL_DIR/spider"

step "创建系统用户 $SERVICE_USER"
if ! id "$SERVICE_USER" >/dev/null 2>&1; then
  useradd --system --no-create-home --shell /usr/sbin/nologin "$SERVICE_USER"
  success "用户 $SERVICE_USER 已创建"
else
  warn "用户 $SERVICE_USER 已存在，跳过"
fi

step "初始化数据目录 $DATA_DIR"
mkdir -p "$DATA_DIR"
chown "$SERVICE_USER:$SERVICE_USER" "$DATA_DIR"
chmod 700 "$DATA_DIR"
success "数据目录 $DATA_DIR 已就绪"

step "写入 systemd unit"
cat > "$SYSTEMD_UNIT" <<EOF
[Unit]
Description=Spider MCP Server
Documentation=$SPIDER_URL
After=network.target

[Service]
Type=simple
User=$SERVICE_USER
Group=$SERVICE_USER
ExecStart=$INSTALL_DIR/spider
Restart=on-failure
RestartSec=5s
Environment=SPIDER_DATA_DIR=$DATA_DIR

[Install]
WantedBy=multi-user.target
EOF
success "systemd unit → $SYSTEMD_UNIT"

step "启用并启动服务"
systemctl daemon-reload
systemctl enable spider
systemctl restart spider
success "spider 服务已启动"

h1 "安装完成"
printf "  ${DIM}状态：${RESET}  systemctl status spider\n"
printf "  ${DIM}日志：${RESET}  journalctl -u spider -f\n"
printf "  ${DIM}数据：${RESET}  $DATA_DIR\n"
`

var serverInstallTmpl = template.Must(template.New("server-install").Parse(serverInstallScript))

func scriptHandler(tmpl *template.Template, baseURL string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		if err := tmpl.Execute(w, struct{ BaseURL string }{baseURL}); err != nil {
			http.Error(w, "template error: "+err.Error(), http.StatusInternalServerError)
		}
	}
}

// InstallScriptHandler serves the Claude Code client install script.
func InstallScriptHandler(baseURL string) http.HandlerFunc {
	return scriptHandler(installTmpl, baseURL)
}

// ServerInstallScriptHandler serves the Linux server install script.
func ServerInstallScriptHandler(baseURL string) http.HandlerFunc {
	return scriptHandler(serverInstallTmpl, baseURL)
}

// BinaryDownloadHandler serves the spider linux binary from <dataDir>/bin/.
// Query param: arch=amd64 (default) | arm64
func BinaryDownloadHandler(dataDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		arch := r.URL.Query().Get("arch")
		if arch == "" {
			arch = "amd64"
		}
		if arch != "amd64" && arch != "arm64" {
			http.Error(w, "unsupported arch", http.StatusBadRequest)
			return
		}
		binPath := filepath.Join(dataDir, "bin", "spider-linux-"+arch)
		f, err := os.Open(binPath)
		if err != nil {
			http.Error(w, "binary not found: run 'make build-linux' and copy to "+binPath, http.StatusNotFound)
			return
		}
		defer f.Close()
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Disposition", `attachment; filename="spider"`)
		io.Copy(w, f)
	}
}

// SkillsTarGzHandler streams all skills from <dataDir>/skills/ as a tar.gz.
func SkillsTarGzHandler(dataDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		diskDir := filepath.Join(dataDir, "skills")

		if _, err := os.Stat(diskDir); os.IsNotExist(err) {
			http.Error(w, "skills directory not found: "+diskDir, http.StatusNotFound)
			return
		}

		files := map[string][]byte{}
		if err := filepath.WalkDir(diskDir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			rel, _ := filepath.Rel(diskDir, path)
			data, readErr := os.ReadFile(path)
			if readErr != nil {
				return readErr
			}
			files[rel] = data
			return nil
		}); err != nil {
			http.Error(w, "failed to read skills: "+err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/gzip")
		gw := gzip.NewWriter(w)
		tw := tar.NewWriter(gw)
		for name, data := range files {
			hdr := &tar.Header{Name: name, Mode: 0644, Size: int64(len(data))}
			if err := tw.WriteHeader(hdr); err != nil {
				return
			}
			_, _ = io.Copy(tw, bytes.NewReader(data))
		}
		_ = tw.Close()
		_ = gw.Close()
	}
}

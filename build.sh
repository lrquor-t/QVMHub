#!/bin/bash
# ============================================================
# QVMHub 本地打包脚本（精简版 · 纯 HTTP 控制器）
# 构建前端 + 后端（原生编译），产物:
#   - 根目录二进制:  ./qvmhub            （可直接运行）
#   - 发行包:        release/qvmhub-linux-{amd64|arm64}.tar.gz
# ============================================================

set -Eeuo pipefail

# 颜色定义
RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; CYAN='\033[0;36m'; NC='\033[0m'
info()    { echo -e "${GREEN}[INFO]${NC} $1"; }
warn()    { echo -e "${YELLOW}[WARN]${NC} $1"; }
error()   { echo -e "${RED}[ERROR]${NC} $1"; exit 1; }
success() { echo -e "${GREEN}[✓]${NC} $1"; }

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
SERVER_DIR="$SCRIPT_DIR/server"
WEB_DIR="$SCRIPT_DIR/web"
RELEASE_DIR="$SCRIPT_DIR/release"
ROOT_BIN="$SCRIPT_DIR/qvmhub"

# 宿主机架构（仅用于发行包命名；本脚本只做原生编译）
HOST_ARCH=$(uname -m)
case "$HOST_ARCH" in
    x86_64|amd64)   HOST_ARCH="amd64" ;;
    aarch64|arm64)  HOST_ARCH="arm64" ;;
    *)              HOST_ARCH="amd64" ;;
esac
OUTPUT_NAME="qvmhub-linux-${HOST_ARCH}"

# ==================== 参数解析 ====================
VERSION=""
SKIP_FRONTEND=false
SKIP_BACKEND=false

usage() {
    cat <<EOF
用法: $0 [选项]
选项:
  -v, --version VERSION   指定版本号 (例: 1.0.0)，默认 dev
  --skip-frontend         跳过前端构建（需已存在 web/dist）
  --skip-backend          跳过后端构建（需已存在根目录 ./qvmhub）
  -h, --help              显示帮助
示例:
  $0                      构建，版本 dev
  $0 -v 1.0.0             指定版本号构建
EOF
}

while [[ $# -gt 0 ]]; do
    case "$1" in
        -v|--version)     VERSION="$2"; shift 2 ;;
        --skip-frontend)  SKIP_FRONTEND=true; shift ;;
        --skip-backend)   SKIP_BACKEND=true; shift ;;
        -h|--help)        usage; exit 0 ;;
        *)                error "未知参数: $1（用 -h 查看帮助）" ;;
    esac
done

# 版本号：去 v 前缀，统一加 v
if [ -n "$VERSION" ]; then VERSION="${VERSION#v}"; else VERSION="dev"; fi
BUILD_VERSION="v${VERSION}"
BUILD_TIME=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

echo ""
echo -e "${CYAN}╔══════════════════════════════════════════╗${NC}"
echo -e "${CYAN}║         QVMHub 构建打包脚本               ║${NC}"
echo -e "${CYAN}╠══════════════════════════════════════════╣${NC}"
echo -e "${CYAN}║${NC}  版本:   ${GREEN}${BUILD_VERSION}${NC}"
echo -e "${CYAN}║${NC}  时间:   ${GREEN}${BUILD_TIME}${NC}"
echo -e "${CYAN}║${NC}  架构:   ${GREEN}${HOST_ARCH} (原生)${NC}"
echo -e "${CYAN}╚══════════════════════════════════════════╝${NC}"
echo ""

# ==================== 构建前端 ====================
if [ "$SKIP_FRONTEND" = false ]; then
    command -v npm &>/dev/null || error "npm 未安装，请先安装 Node.js (推荐 v20+)"
    info "构建前端..."
    cd "$WEB_DIR"
    if [ -f package-lock.json ]; then npm ci; else npm install; fi
    npm run build
    [ -d "$WEB_DIR/dist" ] || error "前端构建失败，未生成 dist 目录"
    success "前端构建完成"
else
    warn "跳过前端构建"
    [ -d "$WEB_DIR/dist" ] || error "web/dist 不存在，无法跳过前端构建"
fi

# ==================== 构建后端 → 根目录二进制 ====================
if [ "$SKIP_BACKEND" = false ]; then
    command -v go &>/dev/null || error "Go 未安装（参考 server/go.mod 版本要求）"
    info "构建后端 → ./qvmhub (根目录)..."
    cd "$SERVER_DIR"
    CGO_ENABLED=${CGO_ENABLED:-1} go build \
        -ldflags="-s -w \
            -X main.Version=${BUILD_VERSION} \
            -X qvmhub/handler.Version=${BUILD_VERSION} \
            -X qvmhub/handler.BuildTime=${BUILD_TIME}" \
        -o "$ROOT_BIN" \
        .
    [ -f "$ROOT_BIN" ] || error "后端构建失败，未生成二进制"
    chmod +x "$ROOT_BIN"
    success "后端构建完成 → ./qvmhub"
else
    warn "跳过后端构建"
    [ -f "$ROOT_BIN" ] || error "根目录 ./qvmhub 不存在，无法跳过后端构建"
fi

# ==================== 打包发行文件 ====================
info "打包发行文件..."
rm -rf "$RELEASE_DIR"
STAGE="$RELEASE_DIR/${OUTPUT_NAME}"
mkdir -p "$STAGE"

cp "$ROOT_BIN" "$STAGE/qvmhub"
chmod +x "$STAGE/qvmhub"
cp -r "$WEB_DIR/dist" "$STAGE/web-dist"
cp "$SCRIPT_DIR/install.sh" "$STAGE/install.sh"
chmod +x "$STAGE/install.sh"

cd "$RELEASE_DIR"
tar -czf "${OUTPUT_NAME}.tar.gz" "${OUTPUT_NAME}/"
PACKAGE_SIZE=$(du -sh "${OUTPUT_NAME}.tar.gz" | cut -f1)

echo ""
echo -e "${CYAN}╔══════════════════════════════════════════╗${NC}"
echo -e "${CYAN}║         构建完成！                        ║${NC}"
echo -e "${CYAN}╠══════════════════════════════════════════╣${NC}"
echo -e "${CYAN}║${NC}  二进制: ${GREEN}./qvmhub${NC}"
echo -e "${CYAN}║${NC}  发行包: ${GREEN}release/${OUTPUT_NAME}.tar.gz${NC}  (${PACKAGE_SIZE})"
echo -e "${CYAN}║${NC}  版本:   ${GREEN}${BUILD_VERSION}${NC}"
echo -e "${CYAN}║${NC}  内容:   qvmhub / web-dist/ / install.sh${NC}"
echo -e "${CYAN}╚══════════════════════════════════════════╝${NC}"
echo ""
success "直接运行: ./qvmhub    或部署: tar -xzf release/${OUTPUT_NAME}.tar.gz && cd ${OUTPUT_NAME} && sudo ./install.sh"

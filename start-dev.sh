#!/bin/bash
# ============================================================
# QVMHub 一键开发启动脚本
# 同时启动后端（air 热重载）和前端（vite dev）
# ============================================================

set -e

# 确保 GOPATH/bin 在 PATH 中
export PATH="$PATH:$(go env GOPATH)/bin:/usr/local/go/bin"

# 颜色定义
GREEN='\033[0;32m'; YELLOW='\033[1;33m'; CYAN='\033[0;36m'; RED='\033[0;31m'; NC='\033[0m'

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
SERVER_DIR="$SCRIPT_DIR/server"
WEB_DIR="$SCRIPT_DIR/web"

info()  { echo -e "${GREEN}[INFO]${NC} $1"; }
warn()  { echo -e "${YELLOW}[WARN]${NC} $1"; }
error() { echo -e "${RED}[ERROR]${NC} $1"; }

# 清理函数：退出时杀掉所有子进程
cleanup() {
    echo ""
    info "正在停止所有服务..."
    kill $BACKEND_PID $FRONTEND_PID 2>/dev/null || true
    wait $BACKEND_PID $FRONTEND_PID 2>/dev/null || true
    info "服务已停止"
}
trap cleanup EXIT INT TERM

# ==================== 检查依赖 ====================
info "检查依赖..."

# air（热重载，兼容 Go 1.22~1.26，使用 v1.61.7）
if ! command -v air &>/dev/null; then
    if [ -x "$(go env GOPATH)/bin/air" ]; then
        info "air 已存在于 $(go env GOPATH)/bin/air"
    else
        warn "air 未安装，正在安装 v1.61.7 ..."
        go install github.com/air-verse/air@v1.61.7
        [ -x "$(go env GOPATH)/bin/air" ] || { error "air 安装失败，请手动: go install github.com/air-verse/air@v1.61.7"; exit 1; }
    fi
fi

command -v npm &>/dev/null || { error "npm 未安装，请先安装 Node.js"; exit 1; }

# ==================== 安装前端依赖 ====================
if [ ! -d "$WEB_DIR/node_modules" ]; then
    info "安装前端依赖..."
    cd "$WEB_DIR" && npm install
fi

# ==================== 启动 ====================
echo ""
echo -e "${CYAN}╔══════════════════════════════════════╗${NC}"
echo -e "${CYAN}║     QVMHub 开发环境启动中            ║${NC}"
echo -e "${CYAN}╚══════════════════════════════════════╝${NC}"
echo ""

# ==================== 日志 / 运行时环境变量 ====================
# 注：二进制仍读取 KVM_ 前缀的环境变量（见 server/config/config.go），此处沿用。
# KVM_LOG_DIR           日志目录（默认 ./log）
# KVM_LOG_LEVEL         文件日志级别: debug/info/warn/error（默认 info）
# KVM_LOG_MAX_DAYS      日志保留天数（默认 7）
# KVM_LOG_CONSOLE       是否输出到终端: true/false（默认 true）
# KVM_LOG_CONSOLE_TYPES 终端日志类型，逗号分隔: app, request, all（默认 app）
# KVM_LOG_CONSOLE_LEVEL 终端独立级别（留空则跟随 KVM_LOG_LEVEL），示例: "warn"
export KVM_LOG_CONSOLE_TYPES=""
export KVM_LOG_CONSOLE_LEVEL="warn"
# 开发模式：放宽错误回包、跳过部分严格校验
export KVM_DEVELOPMENT_MODE=true
# ====================================================

info "启动后端 (air 热重载)..."
cd "$SERVER_DIR" && air &
BACKEND_PID=$!

info "启动前端 (vite dev, 0.0.0.0)..."
cd "$WEB_DIR" && npx vite --host 0.0.0.0 &
FRONTEND_PID=$!

echo ""
info "========================================="
info "  后端: http://localhost:8088"
info "  前端: http://0.0.0.0:8089"
info "  按 Ctrl+C 停止所有服务"
info "========================================="
echo ""

# 等待任一进程退出
wait -n $BACKEND_PID $FRONTEND_PID 2>/dev/null || true

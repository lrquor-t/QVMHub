#!/bin/bash
# ============================================================
# QVMHub 安装 / 更新 / 卸载脚本（纯 HTTP 控制器）
#
# 安装内容:
#   二进制:   /opt/qvmhub/qvmhub
#   前端:     /opt/qvmhub/web-dist/
#   配置:     /etc/qvmhub/env          (KVM_* 环境变量)
#   数据:     /opt/qvmhub/data/
#   日志:     /var/log/qvmhub/
#   服务:     qvmhub.service (systemd，以 qvmhub 用户运行)
#
# 用法:
#   ./install.sh                       安装/更新（交互，默认端口 8088）
#   ./install.sh install -y            非交互安装
#   ./install.sh install --port 9000   指定端口
#   ./install.sh uninstall             卸载（保留数据/配置）
#   ./install.sh uninstall --purge     彻底清除（含数据/配置/日志）
# ============================================================

set -Eeuo pipefail

# 颜色定义
RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; CYAN='\033[0;36m'; NC='\033[0m'
info()    { echo -e "${GREEN}[INFO]${NC} $1"; }
warn()    { echo -e "${YELLOW}[WARN]${NC} $1"; }
error()   { echo -e "${RED}[ERROR]${NC} $1"; exit 1; }
success() { echo -e "${GREEN}[✓]${NC} $1"; }

# ==================== 常量 ====================
APP_NAME="QVMHub"
INSTALL_DIR="/opt/qvmhub"
CONFIG_DIR="/etc/qvmhub"
ENV_FILE="$CONFIG_DIR/env"
DATA_DIR="$INSTALL_DIR/data"
LOG_DIR="/var/log/qvmhub"
SERVICE_NAME="qvmhub"
SERVICE_USER="qvmhub"
SERVICE_FILE="/etc/systemd/system/${SERVICE_NAME}.service"

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
DEFAULT_PORT=8088

# ==================== 参数解析 ====================
ACTION="install"
ASSUME_YES=false
PURGE=false
OPT_PORT=""
OPT_BIN=""

usage() {
    cat <<EOF
QVMHub 安装/卸载脚本（纯 HTTP 控制器）

用法:
  $0 [install] [选项]         安装或更新（默认动作）
  $0 uninstall [--purge]      卸载（--purge 同时清除数据/配置/日志）

选项:
  --port N        网页端口（默认 ${DEFAULT_PORT}）
  --bin PATH      本地二进制路径（默认 ./qvmhub，即与本脚本同目录）
  -y, --yes       非交互模式，全部使用默认值
  -h, --help      显示本帮助
EOF
}

while [[ $# -gt 0 ]]; do
    case "$1" in
        install|uninstall) ACTION="$1"; shift ;;
        --port)   OPT_PORT="$2"; shift 2 ;;
        --bin)    OPT_BIN="$2"; shift 2 ;;
        -y|--yes) ASSUME_YES=true; shift ;;
        --purge)  PURGE=true; shift ;;
        -h|--help) usage; exit 0 ;;
        *) error "未知参数: $1（用 -h 查看帮助）" ;;
    esac
done

# ==================== root 检查 ====================
[ "$(id -u)" -eq 0 ] || error "请以 root 运行: sudo $0 $*"

# ==================== 辅助函数 ====================
random_secret() { head -c 32 /dev/urandom | base64 | tr -d '/+=\n' | head -c 40; }

# env 文件为 KEY=VALUE 简单格式（二进制通过 os.Getenv 读取）
env_get() {
    [ -f "$ENV_FILE" ] || return 0
    grep -E "^${1}=" "$ENV_FILE" 2>/dev/null | head -1 | cut -d= -f2- | tr -d '"' || true
}
env_set() {  # 覆盖写入
    local key="$1" val="$2"
    if [ -f "$ENV_FILE" ] && grep -qE "^${key}=" "$ENV_FILE"; then
        sed -i "s|^${key}=.*|${key}=${val}|" "$ENV_FILE"
    else
        printf '%s=%s\n' "$key" "$val" >> "$ENV_FILE"
    fi
}
env_default() {  # 仅在不存在时写入（更新时保留已有值）
    local cur; cur="$(env_get "$1" || true)"
    [ -n "$cur" ] || env_set "$1" "$2"
}

ensure_user() {
    if ! id "$SERVICE_USER" &>/dev/null; then
        info "创建系统用户 ${SERVICE_USER}..."
        useradd --system --no-create-home --shell /usr/sbin/nologin "$SERVICE_USER"
    fi
}

write_service_unit() {
    cat > "$SERVICE_FILE" <<EOF
[Unit]
Description=${APP_NAME} Controller
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=${SERVICE_USER}
Group=${SERVICE_USER}
WorkingDirectory=${INSTALL_DIR}
EnvironmentFile=${ENV_FILE}
ExecStart=${INSTALL_DIR}/${SERVICE_NAME}
Restart=on-failure
RestartSec=5
LimitNOFILE=65535

[Install]
WantedBy=multi-user.target
EOF
}

# ==================== 安装 / 更新 ====================
do_install() {
    # 定位本地二进制（打包产物目录中应含 qvmhub + web-dist + install.sh）
    LOCAL_BIN="${OPT_BIN:-$SCRIPT_DIR/qvmhub}"
    [ -f "$LOCAL_BIN" ] || error "未找到二进制: ${LOCAL_BIN}\n  请在打包产物目录中运行（含 qvmhub + web-dist + install.sh），或用 --bin 指定路径。"

    # 端口确定
    if [ -n "$OPT_PORT" ]; then
        PORT="$OPT_PORT"
    elif [ "$ASSUME_YES" = true ] || [ ! -t 0 ]; then
        PORT="$DEFAULT_PORT"
    else
        read -rp "网页端口 [${DEFAULT_PORT}]: " input_port
        PORT="${input_port:-$DEFAULT_PORT}"
    fi
    [[ "$PORT" =~ ^[0-9]+$ ]] && [ "$PORT" -ge 1 ] && [ "$PORT" -le 65535 ] || error "无效端口: ${PORT}"

    info "停止已有服务（如有）..."
    systemctl stop "$SERVICE_NAME" 2>/dev/null || true

    ensure_user

    info "创建目录..."
    mkdir -p "$INSTALL_DIR" "$CONFIG_DIR" "$DATA_DIR" "$LOG_DIR"

    info "安装二进制与前端..."
    install -m 0755 "$LOCAL_BIN" "$INSTALL_DIR/$SERVICE_NAME"
    if [ -d "$SCRIPT_DIR/web-dist" ]; then
        rm -rf "$INSTALL_DIR/web-dist"
        cp -r "$SCRIPT_DIR/web-dist" "$INSTALL_DIR/web-dist"
    else
        warn "未找到 web-dist/，前端不可用（仅 API 模式）"
    fi

    # 写配置：env_default 保留已有值；端口始终以本次输入为准
    info "写入配置 ${ENV_FILE} ..."
    touch "$ENV_FILE"
    chmod 640 "$ENV_FILE"

    local admin_pass jwt_secret sec_secret vmcred_secret new_creds=false
    admin_pass="$(env_get KVM_ADMIN_PASS || true)"
    jwt_secret="$(env_get KVM_JWT_SECRET || true)"
    sec_secret="$(env_get KVM_SECURITY_SECRET || true)"
    vmcred_secret="$(env_get KVM_VM_CREDENTIAL_SECRET || true)"
    [ -n "$admin_pass" ]    || { admin_pass="$(random_secret)"; new_creds=true; }
    [ -n "$jwt_secret" ]    || jwt_secret="$(random_secret)"
    [ -n "$sec_secret" ]    || sec_secret="$(random_secret)"
    [ -n "$vmcred_secret" ] || vmcred_secret="$(random_secret)"

    env_default "KVM_DB_PATH"                  "${DATA_DIR}/qvmhub.db"
    env_default "KVM_JWT_SECRET"               "$jwt_secret"
    env_default "KVM_SECURITY_SECRET"          "$sec_secret"
    env_default "KVM_VM_CREDENTIAL_SECRET"     "$vmcred_secret"
    env_default "KVM_JWT_SECRET_ROTATE_HOURS"  "24"
    env_default "KVM_JWT_EXPIRE_HOURS"         "24"
    env_default "KVM_ADMIN_USER"               "admin"
    env_default "KVM_ADMIN_PASS"               "$admin_pass"
    env_default "KVM_SITE_TITLE"              "QVMHub"
    env_default "KVM_DEVELOPMENT_MODE"        "false"
    env_default "KVM_LOG_DIR"                 "$LOG_DIR"
    env_default "KVM_LOG_LEVEL"               "info"
    env_default "KVM_LOG_MAX_DAYS"            "7"
    env_default "KVM_PUBLIC_BASE_URL"         ""
    env_set   "KVM_PORT"                      "$PORT"

    chown root:"$SERVICE_USER" "$ENV_FILE"
    # 服务以 qvmhub 用户运行，需对安装/数据/日志目录可读写
    chown -R "$SERVICE_USER":"$SERVICE_USER" "$INSTALL_DIR" "$DATA_DIR" "$LOG_DIR"

    info "生成 systemd 服务..."
    write_service_unit

    systemctl daemon-reload
    systemctl enable "$SERVICE_NAME" >/dev/null
    systemctl restart "$SERVICE_NAME"

    # 结果输出
    local host_ip; host_ip="$(hostname -I 2>/dev/null | awk '{print $1}')"
    [ -n "$host_ip" ] || host_ip="localhost"
    echo ""
    echo -e "${CYAN}╔══════════════════════════════════════════════════╗${NC}"
    echo -e "${CYAN}║         ${APP_NAME} 安装完成！                       ║${NC}"
    echo -e "${CYAN}╠══════════════════════════════════════════════════╣${NC}"
    echo -e "${CYAN}║${NC}  访问:   ${GREEN}http://${host_ip}:${PORT}${NC}"
    echo -e "${CYAN}║${NC}  账号:   ${GREEN}$(env_get KVM_ADMIN_USER || echo admin)${NC}"
    if [ "$new_creds" = true ]; then
        echo -e "${CYAN}║${NC}  密码:   ${GREEN}${admin_pass}${NC}  ${YELLOW}(首次生成，请妥善保存)${NC}"
    else
        echo -e "${CYAN}║${NC}  密码:   ${YELLOW}(沿用已有，未改动)${NC}"
    fi
    echo -e "${CYAN}║${NC}  配置:   ${GREEN}${ENV_FILE}${NC}"
    echo -e "${CYAN}║${NC}  日志:   journalctl -u ${SERVICE_NAME} -f"
    echo -e "${CYAN}╚══════════════════════════════════════════════════╝${NC}"
    echo ""
    success "服务状态: systemctl status ${SERVICE_NAME}"
}

# ==================== 卸载 ====================
do_uninstall() {
    info "停止并禁用服务..."
    systemctl stop "$SERVICE_NAME" 2>/dev/null || true
    systemctl disable "$SERVICE_NAME" 2>/dev/null || true

    info "移除 systemd 单元..."
    rm -f "$SERVICE_FILE"
    systemctl daemon-reload

    if [ "$PURGE" = true ]; then
        warn "彻底清除: 删除 ${INSTALL_DIR} / ${CONFIG_DIR} / ${LOG_DIR}"
        rm -rf "$INSTALL_DIR" "$CONFIG_DIR" "$LOG_DIR"
        if id "$SERVICE_USER" &>/dev/null; then
            userdel "$SERVICE_USER" 2>/dev/null || true
        fi
        success "${APP_NAME} 已彻底卸载"
    else
        success "${APP_NAME} 已卸载（保留 ${DATA_DIR} 与 ${ENV_FILE}；加 --purge 可彻底清除）"
    fi
}

# ==================== 入口 ====================
case "$ACTION" in
    install)   do_install ;;
    uninstall) do_uninstall ;;
    *)         error "未知动作: $ACTION" ;;
esac

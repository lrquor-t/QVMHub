# QVMHub - 多节点 QVMConsole 管控网关

<div align="center">

<img width="2403" height="1257" alt="QVMHub" src="https://github.com/user-attachments/assets/965011d7-9cf3-4ef4-b39e-7b22fe99a1c8" />

[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)

**GitHub:** https://github.com/lrquor-t/QVMHub &nbsp;_(仓库即将公开)_

</div>

> QVMHub 是一个**纯 HTTP 控制器**:它把多台分散部署的 [QVMConsole](https://github.com/QVMConsole/QVMConsole) 节点统一接入，提供一个 Web 入口、一套账户与权限、一张总览大屏，所有针对虚拟机的操作都由 QVMHub **反向代理透传**到对应节点的原生 API 执行。

## 它解决什么问题

每台宿主机上运行的 QVMConsole 本身就是一个完整的虚拟机管理控制台。当你的 QVMConsole 节点不止一台（多台宿主机、多个机房、多套环境）时，逐个登录每个节点去操作、看监控、管账号会非常割裂。

QVMHub 在这些节点**之上**加一层网关：

- **一个入口** 管理所有节点，按节点切换上下文；
- **一套账户** 统一登录、角色权限（RBAC）、API Key、2FA；
- **一张大屏** 聚合各节点健康度与资源用量；
- **一条通道** 把浏览器的每一个操作请求透传到目标节点，包括 VNC / LXC 终端 / SSE 长任务流。

## QVMHub 与 QVMConsole 的关系

| | QVMConsole（节点） | QVMHub（控制器） |
|---|---|---|
| 角色 | 单节点虚拟机管理控制台 | 多节点中央管控网关 |
| 是否操作虚拟化 | 是，直接调用 libvirt / KVM | 否，自身**不跑虚拟化**，纯 HTTP |
| 部署位置 | 每台宿主机一台 | 任一台可联网服务器（一台即可） |
| 数据面 | VM 生命周期 / 网络 / 存储 / 快照 / VNC … | 注册节点 / 探活 / 反向代理 / 终端中继 / RBAC |

二者**互补而非替代**：QVMConsole 提供单节点的虚拟化能力，QVMHub 负责把多个 QVMConsole 编织成一个统一的管理平面。接入一个节点，只需要该节点的 **API 地址 + 一个 admin 级 API Key**。

## 核心特性

- **节点注册与凭据保管**：以 `API Base URL + API Key（加密存储）` 登记每个 QVMConsole 节点，可选填 SSH 信息用于跨节点运维；凭据集中保管于内置密钥库，并记录最近使用时间。
- **反向代理透传**：浏览器请求打到 `/api/n/{nodeId}/...`，控制器剥离前缀、附上节点 API Key 后转发到节点原生 API，响应原样回传。QVMHub 不实现也不重复造任何虚拟机功能。
- **流式与终端中继**：
  - **SSE** 长任务实时推送（无超时客户端 + 分块 Flush，不被切断）；
  - **Blob / 大文件** 缓冲透传（模板包上传下载）；
  - **VNC 与 LXC Web 终端** 经 WebSocket 跨节点中继。
- **节点健康监控**：后台探活调度器（约 15s 探版本、60s 探资源），纯 HTTP 探测不打 SSH，结果写入内存健康缓存并回写节点表；总览页只读缓存，零请求时不向节点扇出。
- **统一账户与 RBAC**：登录 / 注册 / 邀请 / 找回密码 / 2FA / 安全初始化；网关层基于角色对节点与路径做访问控制。
- **API Key 自动化**：除账户安全流程外，接口默认兼容 API Key 调用，便于外部系统集成。
- **多节点总览**：聚合各节点 CPU / 内存 / 磁盘 / VM 数等指标，统一大屏。
- **跨节点迁移**：在登记的节点之间迁移虚拟机（经节点 SSH 信息执行运维步骤）。

## 架构概览

```
                       ┌──────────────────────────────────────────────┐
   浏览器 ────────────▶│              QVMHub 控制器（纯 HTTP）          │
  (Web 控制台)         │  统一登录 · RBAC · API Key · 2FA              │
                       │  节点注册 · 探活调度 · 健康缓存 · 多节点总览   │
                       │  反向代理 · SSE/Blob 透传 · VNC/终端 WS 中继  │
                       │  密钥保管库                                   │
                       └────────┬───────────┬───────────┬──────────────┘
                                │ /api/n/1  │ /api/n/2  │ /api/n/3
                       ┌────────▼─────┐ ┌───▼────────┐ ┌──▼──────────┐
                       │  QVMConsole  │ │ QVMConsole │ │ QVMConsole  │
                       │    节点 A    │ │   节点 B   │ │   节点 C    │
                       │  (libvirt)   │ │ (libvirt)  │ │  (libvirt)  │
                       └──────────────┘ └────────────┘ └─────────────┘
```

## 技术栈

**后端**（`server/`，Go）
- 语言：Go 1.26
- Web 框架：Gin v1.12
- 数据库：SQLite + GORM v1.31
- 鉴权：JWT v5（golang-jwt/jwt v5.3）、TOTP 2FA（pquerna/otp）
- 通信：gorilla/websocket（控制台 / 终端中继）、creack/pty（终端 PTY）
- 日志：lumberjack v2.2

**前端**（`web/`，Vue 3）
- 框架：Vue 3.5 + Element Plus 2.13
- 构建：Vite 8
- 状态 / 路由：Pinia、Vue Router
- 可视化：ECharts 6
- 控制台：@novnc/novnc（VNC）、xterm（LXC / Web 终端）

## 快速开始

### 一、生产部署（`install.sh`）

在打包产物目录（含 `qvmhub` 二进制 + `web-dist/` + `install.sh`）下以 root 运行：

```bash
sudo ./install.sh                       # 交互安装，默认端口 8088
sudo ./install.sh install -y            # 非交互
sudo ./install.sh install --port 9000   # 指定端口
sudo ./install.sh uninstall             # 卸载（保留数据 / 配置）
sudo ./install.sh uninstall --purge     # 彻底清除
```

安装会：
- 部署二进制到 `/opt/qvmhub/qvmhub`、前端到 `/opt/qvmhub/web-dist/`；
- 写配置到 `/etc/qvmhub/env`（`KVM_*` 变量，自动生成 JWT / 凭据加密密钥 / admin 密码）；
- 注册 systemd 服务 `qvmhub.service`（以 `qvmhub` 系统用户运行）并启动。

完成后访问 `http://<主机IP>:8088`，使用默认账号 `admin` 登录（首次安装时脚本会打印随机生成的密码，请妥善保存）。

### 二、开发模式（`start-dev.sh`）

```bash
./start-dev.sh
```

同时拉起后端（`air` 热重载）与前端（`vite dev`）：

- 后端 API：`http://localhost:8088`
- 前端开发服务器：`http://0.0.0.0:8089`（已配置 `/api` 代理到后端，含 WebSocket）

> 二进制读取的仍是 `KVM_*` 前缀环境变量（见 `server/config/config.go`）。

### 三、接入第一个 QVMConsole 节点

1. 在某台已部署 QVMConsole 的宿主机上，生成一个 **admin 级 API Key**，记下其 `API Base URL`、`Key ID` 与 `Key`。
2. 用 admin 登录 QVMHub，进入「节点管理」，新增节点并填入：名称、API Base URL、API Key ID、API Key（加密入库），以及（可选，用于跨节点迁移的）SSH 信息。
3. 保存后 QVMHub 立即开始探活，节点状态变为 `online` 后，即可在总览与各功能页对该节点进行操作。

## 配置

运行时配置集中于 `/etc/qvmhub/env`（开发模式下由 `server/.env` 或环境变量提供），统一使用 `KVM_` 前缀。常用项：

| 变量 | 说明 | 默认 |
|---|---|---|
| `KVM_PORT` | Web / API 监听端口 | `8088` |
| `KVM_DB_PATH` | SQLite 数据库路径 | `/opt/qvmhub/data/qvmhub.db` |
| `KVM_ADMIN_USER` / `KVM_ADMIN_PASS` | 初始管理员账号 / 密码 | `admin` / 首次随机 |
| `KVM_JWT_SECRET` | JWT 签名密钥 | 首次随机 |
| `KVM_VM_CREDENTIAL_SECRET` | 节点凭据 / 密钥库加密密钥 | 首次随机 |
| `KVM_JWT_EXPIRE_HOURS` | JWT 有效期（小时） | `24` |
| `KVM_LOG_DIR` / `KVM_LOG_LEVEL` | 日志目录 / 级别 | `/var/log/qvmhub` / `info` |

部署后修改端口也可用运维脚本：`./qvmc-manage.sh`（同步更新 env 与防火墙规则并重启服务）。

## 项目结构

```
qvmhub/
├── server/                 # Go 后端（控制器）
│   ├── main.go             # 入口：配置 / 日志 / DB / 探活调度器 / 路由
│   ├── config/             # 配置加载（KVM_* 环境变量 + DB 持久化设置）
│   ├── router/             # 路由注册（/api/auth、/api/nodes、/api/n/:nodeId/* …）
│   ├── handler/            # proxy / console_relay / overview / nodes / auth / api_key …
│   ├── middleware/         # rbac（网关层权限）/ ratelimit
│   ├── model/              # GORM 模型（host_nodes、user、user_api_key …）
│   ├── service/nodereg/    # 节点探活：probe / scheduler / cache（纯 HTTP）
│   └── web-dist/           # 前端构建产物
├── web/                    # Vue 3 前端
│   └── src/views/          # node / dashboard / vm / lxc / network / storage …
├── install.sh              # 生产部署（systemd）
├── start-dev.sh            # 开发模式（air + vite）
├── build.sh                # 打包二进制 + 前端
└── qvmc-manage.sh          # 运维管理（改端口等）
```

## 贡献

欢迎提 Issue 与 PR。开发约定见 [`AGENTS.md`](AGENTS.md)。

> 说明：`AGENTS.md` 中的部分内容继承自上游 QVMConsole，主要面向**节点侧**虚拟机功能的开发；QVMHub **网关层**的开发关注点在于节点注册 / 探活、反向代理与流式中继、网关 RBAC、密钥库等。

## 安全漏洞报告

如发现安全漏洞，请勿在公开 Issue 中提交，以免被恶意利用。请通过 GitHub 私信或仓库公布的私密渠道联系维护者。

## License

[Apache License 2.0](LICENSE)

## 致谢

QVMHub 基于 [QVMConsole](https://github.com/QVMConsole/QVMConsole) 二次开发 —— 感谢上游项目及其贡献者。

维护者：**lrquor-t** · https://github.com/lrquor-t/QVMHub

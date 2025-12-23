# Bastion - Go Implementation

[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)
[![Platform](https://img.shields.io/badge/platform-Windows%20%7C%20Linux%20%7C%20macOS-lightgrey.svg)](https://github.com/wildking996/bastion)

English | [简体中文](#简体中文)

Bastion is a secure SSH bastion host with local/dynamic forwarding, HTTP auditing, and a web UI for managing mappings.

## Features

- SSH connection pooling and session management
- Dynamic port forwarding (SOCKS5) and local port forwarding
- HTTP traffic auditing with in-memory logs
- HTTP forward proxy supports WebSocket Upgrade (frames are tunneled; audit covers the initial HTTP handshake only)
- Real-time traffic chart in the Web UI (polls `/api/stats`)
- Self-update from the Web UI (tracks GitHub "Latest Release")
- Web-based management interface served from `/web`
- CLI client mode to control a running server (`--cli --server <url>`)
- Multi-platform builds (Windows, Linux, macOS; GUI and console variants on Windows)

## Requirements

- Go 1.21 or higher

## Installation

```bash
# Install dependencies
go mod download

# Build for current platform with version info injected
make build

# Build for all platforms
make build-all

# Linux/macOS build script
./build.sh

# Windows build script
build.bat
```

Build artifacts are placed in `dist/` when using multi-platform targets.

## Usage

### Run server (default)

```bash
make run
# or
./bastion
```

- Default HTTP port is `7788`; the server auto-selects the next free port if it is busy and logs the chosen port.
- The UI is available at `http://127.0.0.1:<port>/` and redirects to `/web/index.html`.
- A SQLite database is created automatically if missing.
- The browser opens automatically after startup (best effort).

### CLI mode (talk to a remote Bastion)

```bash
./bastion --cli --server http://your-server:7788
```

CLI mode runs without a local database and proxies API calls to the specified server.

### Configuration

Environment variables (overridden by flags where available):
- `PORT` (default `7788`): HTTP server port.
- `LOG_LEVEL` (`DEBUG|INFO|WARN|ERROR`, default `INFO`): global log verbosity.
- `LOG_FILE` (default `./bastion.log`): file-only logging target; startup rotates previous file to `bastion.log.1`.
- `DATABASE_URL` (default `bastion.db`): SQLite database file path.
- `SQLITE_PRAGMAS_ENABLED` (default `true`): enable SQLite PRAGMA defaults.
- `SQLITE_BUSY_TIMEOUT_MS` (default `5000`): PRAGMA `busy_timeout` in milliseconds.
- `SQLITE_JOURNAL_MODE` (default `WAL`): PRAGMA `journal_mode`.
- `SQLITE_SYNCHRONOUS` (default `NORMAL`): PRAGMA `synchronous`.
- `SQLITE_FOREIGN_KEYS` (default `true`): PRAGMA `foreign_keys`.
- `SQLITE_MAX_OPEN_CONNS` (default `1`): SQLite `MaxOpenConns`.
- `SQLITE_MAX_IDLE_CONNS` (default `1`): SQLite `MaxIdleConns`.
- `SQLITE_CONN_MAX_IDLE_SECONDS` (default `300`): SQLite `ConnMaxIdleTime` in seconds.
- `SQLITE_CONN_MAX_LIFETIME_SECONDS` (default `0`): SQLite `ConnMaxLifetime` in seconds.
- `AUDIT_ENABLED` (default `true`): enable HTTP audit logging.
- `MAX_SESSION_CONNECTIONS` (default `1000`): max concurrent connections per mapping.
- `FORWARD_BUFFER_SIZE` (default `32768`): maximum forward buffer size in bytes (adaptive pooled buffers use multiple size classes up to this value; buffers >64KiB are not pooled).
- `AUDIT_QUEUE_SIZE` (default `1000`): asynchronous audit queue length; when full, audit messages are dropped to prioritize forwarding performance.
- `MAX_HTTP_LOGS` (default `1000`): in-memory HTTP log cap.
- `HTTP_PAIR_CLEANUP_INTERVAL_MINUTES` (default `5`): stale HTTP pair cleanup interval.
- `HTTP_PAIR_MAX_AGE_MINUTES` (default `10`): max age before pairing is considered stale.
- `HTTP_GZIP_DECODE_MAX_BYTES` (default `1048576`): max decompressed bytes for on-demand gzip decode preview.
- `HTTP_GZIP_DECODE_TIMEOUT_MS` (default `500`): timeout for on-demand gzip decode preview.
- `HTTP_GZIP_DECODE_CACHE_SECONDS` (default `60`): sliding cache TTL for decoded previews (0 disables cache).
- `GOROUTINE_MONITOR_INTERVAL_SECONDS` (default `30`): goroutine monitor interval.
- `GOROUTINE_WARN_THRESHOLD` (default `1000`): goroutine warning threshold.
- `SOCKS5_HANDSHAKE_TIMEOUT_SECONDS` (default `30`): legacy SOCKS5 handshake timeout (kept for compatibility).
- `SOCKS5_HANDSHAKE_READ_TIMEOUT_SECONDS` (default `30`): SOCKS5 handshake read timeout (per read).
- `SOCKS5_HANDSHAKE_WRITE_TIMEOUT_SECONDS` (default `30`): SOCKS5 handshake write timeout (per write).
- `SESSION_IDLE_TIMEOUT_HOURS` (default `24`): idle timeout for sessions.
- `TRANSFER_READ_TIMEOUT_SECONDS` (default `86400`): data transfer read timeout (per read).
- `TRANSFER_WRITE_TIMEOUT_SECONDS` (default `86400`): data transfer write timeout (per write).
- `SSH_CONNECT_TIMEOUT` (default `15`): SSH dial timeout.
- `SSH_KEEPALIVE_INTERVAL` (default `30`): SSH keepalive interval.
- `SSH_CONNECT_MAX_RETRIES` (default `3`): retries per SSH hop.
- `SSH_CONNECT_RETRY_DELAY_SECONDS` (default `2`): delay between SSH retries.
- `SSH_POOL_MAX_CONNS` (default `64`): maximum pooled SSH connections (per bastion chain).
- `SSH_POOL_IDLE_TIMEOUT_SECONDS` (default `900`): close pooled SSH connections idle for this duration.
- `SSH_POOL_KEEPALIVE_INTERVAL_SECONDS` (default `30`): interval for pooled SSH keepalive probes (0 disables).
- `SSH_POOL_KEEPALIVE_TIMEOUT_MS` (default `500`): timeout for a single pooled SSH keepalive probe.
- `GITHUB_TOKEN` (optional): GitHub token used by the self-update feature to increase GitHub API rate limits (recommended when running behind shared IP / CI / proxy).
- CLI-only: `CLI_MODE` (`false`) to force CLI client mode; use `--server` flag for target URL.

Key flags (see `./bastion --help` for full list):
- `--port` HTTP server port.
- `--db` SQLite database path.
- `--log-level` `DEBUG|INFO|WARN|ERROR`.
- `--log-file` log file path (file-only; rotates previous to `.1`).
- `--audit` enable/disable HTTP audit logging.
- `--cli` run in CLI client mode (no local DB).
- `--server` target server URL for CLI mode.
- `--max-session-connections` per-mapping connection cap.
- `--max-http-logs` in-memory HTTP log cap.
- `--socks5-handshake-read-timeout-seconds`, `--socks5-handshake-write-timeout-seconds`, `--transfer-read-timeout-seconds`, `--transfer-write-timeout-seconds` fine-grained stage read/write timeouts.
- `--ssh-pool-max-conns`, `--ssh-pool-idle-timeout-seconds`, `--ssh-pool-keepalive-interval-seconds`, `--ssh-pool-keepalive-timeout-ms` SSH pool lifecycle settings.
- `--version` show build/version info and exit.

## API Endpoints

- Bastions: `GET /api/bastions`, `POST /api/bastions`, `PUT /api/bastions/:id`, `DELETE /api/bastions/:id`
- Mappings: `GET /api/mappings`, `POST /api/mappings` (create only), `PUT /api/mappings/:id` (update when stopped), `DELETE /api/mappings/:id`, `POST /api/mappings/:id/start`, `POST /api/mappings/:id/stop`
  - Optional mapping access control: `allow_cidrs` / `deny_cidrs` (CIDR or single IP; deny wins; allow non-empty means allow-only)
- Statistics: `GET /api/stats`
- HTTP audit logs: `GET /api/http-logs` (supports `q/regex/method/host/url/local_port/bastion/status/since/until`), `GET /api/http-logs/:id`, `DELETE /api/http-logs`
  - Log detail parts: `GET /api/http-logs/:id?part=request_header|request_body|response_header|response_body`
  - On-demand gzip decode: `GET /api/http-logs/:id?part=response_body&decode=gzip`
- Error logs: `GET /api/error-logs`, `DELETE /api/error-logs`
- Shutdown (confirmation code): `POST /api/shutdown/generate-code`, `POST /api/shutdown/verify`
- Self-update: `GET /api/update/check`, `GET /api/update/proxy`, `POST /api/update/proxy`, `POST /api/update/generate-code`, `POST /api/update/apply` (requires the confirmation code; downloads the matching asset from GitHub "Latest Release" and restarts)
- Health/metrics: `GET /api/health`, `GET /api/metrics`
- Prometheus: `GET /metrics`

## Project Structure

```
bastion/
├── cli/              # CLI mode client
├── config/           # Settings and flag/env parsing
├── core/             # Forwarding, pooling, audit, error logging
├── database/         # Database initialization
├── handlers/         # HTTP API handlers
├── models/           # Data models
├── service/          # Bastion, mapping, audit service layers
├── state/            # Global session state
├── static/           # Web UI assets (served at /web)
├── version/          # Version info injected via ldflags
├── main.go           # Application entry point
├── Makefile          # Build/test/lint targets
├── build.sh / build.bat # Multi-platform builds
└── dist/             # Build artifacts (generated)
```

## Development

- Tests: `make test`
- Format: `make fmt`
- Lint: `make lint`

## Screenshots

> Place screenshots under `static/img/` (or adjust paths) with these filenames to display them below.

- `Dashboard` & `Bastion list` & `Mapping management`: ![Dashboard](static/img/dashboard.png)
- HTTP audit log: ![HTTP logs](static/img/http-logs.png)

## License

See `LICENSE` for details.

---

## 简体中文

[![Go 版本](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![许可证](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)
[![平台](https://img.shields.io/badge/平台-Windows%20%7C%20Linux%20%7C%20macOS-lightgrey.svg)](https://github.com/yourusername/bastion)

[English](#bastion---go-implementation) | 简体中文

Bastion 是一个安全的 SSH 跳板机，支持本地/动态转发、HTTP 审计，并提供 Web 界面管理映射。

### 功能

- SSH 连接池与会话管理
- 动态端口转发（SOCKS5）与本地端口转发
- HTTP 流量审计与内存日志
- HTTP 正向代理支持 WebSocket Upgrade（升级后按原始 TCP 转发；审计仅覆盖升级前的 HTTP 握手）
- `/web` 提供的 Web 管理界面
- Web UI 一键自更新（跟随 GitHub Latest Release）
- CLI 模式远程控制运行中的服务（`--cli --server <url>`）
- 跨平台构建（Windows/Linux/macOS，Windows 同时提供 GUI 与控制台版本）

### 运行

```bash
make run
# 或
./bastion
```

- 默认端口 `7788`，被占用时会自动向上寻找可用端口并打印日志。
- UI 地址：`http://127.0.0.1:<端口>/`，自动重定向到 `/web/index.html`。
- 如果数据库文件不存在会自动创建；启动后会尝试自动打开浏览器。

CLI 模式：`./bastion --cli --server http://your-server:7788`

### 配置（环境变量，可被同名 flag 覆盖）

- `PORT`（默认 `7788`）：HTTP 服务端口。
- `LOG_LEVEL`（`DEBUG|INFO|WARN|ERROR`，默认 `INFO`）：日志级别。
- `LOG_FILE`（默认 `./bastion.log`）：仅写文件日志，启动时将旧日志轮转到 `bastion.log.1`。
- `DATABASE_URL`（默认 `bastion.db`）：SQLite 数据库文件路径。
- `SQLITE_PRAGMAS_ENABLED`（默认 `true`）：启用 SQLite PRAGMA 默认值。
- `SQLITE_BUSY_TIMEOUT_MS`（默认 `5000`）：PRAGMA `busy_timeout`（毫秒）。
- `SQLITE_JOURNAL_MODE`（默认 `WAL`）：PRAGMA `journal_mode`。
- `SQLITE_SYNCHRONOUS`（默认 `NORMAL`）：PRAGMA `synchronous`。
- `SQLITE_FOREIGN_KEYS`（默认 `true`）：PRAGMA `foreign_keys`。
- `SQLITE_MAX_OPEN_CONNS`（默认 `1`）：SQLite `MaxOpenConns`。
- `SQLITE_MAX_IDLE_CONNS`（默认 `1`）：SQLite `MaxIdleConns`。
- `SQLITE_CONN_MAX_IDLE_SECONDS`（默认 `300`）：SQLite `ConnMaxIdleTime`（秒）。
- `SQLITE_CONN_MAX_LIFETIME_SECONDS`（默认 `0`）：SQLite `ConnMaxLifetime`（秒）。
- `AUDIT_ENABLED`（默认 `true`）：启用 HTTP 审计日志。
- `MAX_SESSION_CONNECTIONS`（默认 `1000`）：单映射最大并发连接数。
- `FORWARD_BUFFER_SIZE`（默认 `32768`）：转发缓冲区最大大小（字节；转发会使用多档可复用 buffer，按需增长至该上限；>64KiB 的 buffer 不会进入对象池）。
- `AUDIT_QUEUE_SIZE`（默认 `1000`）：异步审计队列长度；满时将丢弃审计消息以优先保障转发性能。
- `MAX_HTTP_LOGS`（默认 `1000`）：HTTP 日志内存上限。
- `HTTP_PAIR_CLEANUP_INTERVAL_MINUTES`（默认 `5`）：清理未配对 HTTP 请求的间隔分钟数。
- `HTTP_PAIR_MAX_AGE_MINUTES`（默认 `10`）：未配对请求的最大保留分钟数。
- `HTTP_GZIP_DECODE_MAX_BYTES`（默认 `1048576`）：按需解压 gzip 的最大解压后字节数（预览）。
- `HTTP_GZIP_DECODE_TIMEOUT_MS`（默认 `500`）：按需解压 gzip 的超时时间（毫秒）。
- `HTTP_GZIP_DECODE_CACHE_SECONDS`（默认 `60`）：解压预览的短缓存 TTL（滑动过期；0 表示禁用缓存）。
- `GOROUTINE_MONITOR_INTERVAL_SECONDS`（默认 `30`）：goroutine 监控间隔。
- `GOROUTINE_WARN_THRESHOLD`（默认 `1000`）：goroutine 警告阈值。
- `SOCKS5_HANDSHAKE_TIMEOUT_SECONDS`（默认 `30`）：旧的 SOCKS5 握手超时（为兼容保留）。
- `SOCKS5_HANDSHAKE_READ_TIMEOUT_SECONDS`（默认 `30`）：SOCKS5 握手读超时（每次 Read 续期）。
- `SOCKS5_HANDSHAKE_WRITE_TIMEOUT_SECONDS`（默认 `30`）：SOCKS5 握手写超时（每次 Write 续期）。
- `SESSION_IDLE_TIMEOUT_HOURS`（默认 `24`）：会话空闲超时（小时）。
- `TRANSFER_READ_TIMEOUT_SECONDS`（默认 `86400`）：数据转发读超时（每次 Read 续期）。
- `TRANSFER_WRITE_TIMEOUT_SECONDS`（默认 `86400`）：数据转发写超时（每次 Write 续期）。
- `SSH_CONNECT_TIMEOUT`（默认 `15`）：SSH 连接超时。
- `SSH_KEEPALIVE_INTERVAL`（默认 `30`）：SSH keepalive 间隔。
- `SSH_CONNECT_MAX_RETRIES`（默认 `3`）：SSH 每跳重试次数。
- `SSH_CONNECT_RETRY_DELAY_SECONDS`（默认 `2`）：SSH 重试间隔秒数。
- `SSH_POOL_MAX_CONNS`（默认 `64`）：SSH 连接池最大连接数（按 bastion chain 计）。
- `SSH_POOL_IDLE_TIMEOUT_SECONDS`（默认 `900`）：空闲超过该秒数的池连接将被主动关闭。
- `SSH_POOL_KEEPALIVE_INTERVAL_SECONDS`（默认 `30`）：池连接 keepalive 探测间隔（0 表示禁用）。
- `SSH_POOL_KEEPALIVE_TIMEOUT_MS`（默认 `500`）：单次池连接 keepalive 探测超时（毫秒）。
- CLI：`CLI_MODE`（默认 `false`）强制使用 CLI 客户端模式，目标地址使用 `--server`。

常用标志：
- `--port`：HTTP 服务端口。
- `--db`：SQLite 数据库路径。
- `--log-level`：`DEBUG|INFO|WARN|ERROR`。
- `--log-file`：日志文件路径（仅写文件，旧日志轮转为 `.1`）。
- `--audit`：启用/禁用 HTTP 审计日志。
- `--cli`：以 CLI 客户端模式运行（不加载本地数据库）。
- `--server`：CLI 模式下的目标服务器地址。
- `--max-session-connections`：单映射最大连接数。
- `--max-http-logs`：HTTP 日志内存上限。
- `--socks5-handshake-read-timeout-seconds` / `--socks5-handshake-write-timeout-seconds` / `--transfer-read-timeout-seconds` / `--transfer-write-timeout-seconds`：分阶段读写超时配置。
- `--ssh-pool-max-conns` / `--ssh-pool-idle-timeout-seconds` / `--ssh-pool-keepalive-interval-seconds` / `--ssh-pool-keepalive-timeout-ms`：SSH 连接池生命周期设置。
- `--version`：输出版本/构建信息后退出。

### API

- 跳板机：`GET/POST/PUT/DELETE /api/bastions`
- 映射：`GET /api/mappings`、`POST /api/mappings`（仅创建）、`PUT /api/mappings/:id`（停止状态可更新）、`DELETE /api/mappings/:id`、`POST /api/mappings/:id/start`、`POST /api/mappings/:id/stop`
- 统计：`GET /api/stats`
- HTTP 审计日志：`GET /api/http-logs`（支持 `q/regex/method/host/url/local_port/bastion/status/since/until`），`GET /api/http-logs/:id`，`DELETE /api/http-logs`
  - 详情分片：`GET /api/http-logs/:id?part=request_header|request_body|response_header|response_body`
  - 按需 gzip 解压：`GET /api/http-logs/:id?part=response_body&decode=gzip`
- 错误日志：`GET /api/error-logs`，`DELETE /api/error-logs`
- 关闭：`POST /api/shutdown/generate-code`，`POST /api/shutdown/verify`
- 健康/指标：`GET /api/health`，`GET /api/metrics`
- Prometheus：`GET /metrics`

### 结构

`cli/`、`config/`、`core/`、`database/`、`handlers/`、`models/`、`service/`、`state/`、`static/`、`version/`、`main.go`、`Makefile`、`build.sh`、`build.bat`、`dist/`（构建生成）。

### 开发

`make test`（测试）、`make fmt`（格式化）、`make lint`（静态检查）。

### 截图

> 请将截图放到 `static/img/`（或调整路径），并使用以下文件名以显示：

- 仪表盘 & 跳板机列表 & 映射管理：![Dashboard](static/img/dashboard.png)
- HTTP 审计日志：![HTTP logs](static/img/http-logs.png)

### 许可

参见 `LICENSE`。

### Web UI development

The production UI is bundled into `static/` and served from `/web`.

```bash
cd web
pnpm install
+pnpm run build
```

For local HMR during development:

```bash
# terminal 1
cd web
pnpm run dev

# terminal 2
go run .
```

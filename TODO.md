# Bastion 功能扩展路线图

> 基于项目分析提出的功能增强建议，分为个人使用优先级和未来发展方向。

## 📋 目录
- [当前功能概述](#当前功能概述)
- [个人使用推荐 (轻量级)](#个人使用推荐-轻量级)
- [详细功能规划](#详细功能规划)
- [其他扩展方向](#其他扩展方向)

---

## 当前功能概述

Bastion 是一个用 Go 编写的轻量级 SSH 跳板机/代理工具，核心功能包括：
- ✅ SSH 连接池与多层跳转
- ✅ TCP 隧道和 SOCKS5 代理
- ✅ HTTP 流量审计
- ✅ Web UI 管理界面
- ✅ CLI 控制台
- ✅ 连接健康检查与重试

---

## 个人使用推荐 (轻量级)

### 🎯 P0 - 立即实现 (High Impact, Low Effort)

#### 1. 运行中配置不可修改 (Immutable While Running)
- **状态**: ✅ 已实现
- **描述**: 启用中(运行中)的 Mapping/Bastion 不允许被修改，避免运行态与 SQLite 数据不一致（禁止绕过接口直接改 SQLite）
- **约束**:
  - `POST /api/mappings` 仅允许创建，不允许 upsert（存在即冲突）
  - `PUT /api/mappings/:id` 仅允许在停止状态更新；更新时禁止修改 `local_host/local_port/remote_host/remote_port/type`，仅允许调整 `chain`、`auto_start` 等
  - Bastion 被任何运行中的 Mapping 引用时，禁止更新（返回 `409` 并提示先 stop 相关 mapping）
  - Bastion 更新时禁止修改 `host/port/name`，仅允许更新 `username/password/pkey_path/pkey_passphrase` 等凭据字段
  - Mapping 运行中禁止删除，必须先 stop（返回 `409`）
- **操作流程**: stop → 修改(通过 API) → start（若需改 IP/端口/类型：stop → delete → create → start）
- **影响**: 一致性与可维护性 ⭐⭐⭐⭐⭐
- **复杂度**: 低

#### 2. Docker 支持 (Containerization)
- **描述**: 提供官方 Docker 镜像和 docker-compose
- **实现方式**:
  - 编写 Dockerfile (多阶段构建)
  - docker-compose.yml 示例
  - .dockerignore 优化构建
- **影响**: 部署便捷性 ⭐⭐⭐⭐⭐
- **复杂度**: 低

#### 3. 暗色主题 (Dark Mode)
- **描述**: Web UI 支持暗色/亮色主题切换
- **实现方式**:
  - CSS 变量定义主题
  - 本地存储用户偏好
  - 系统主题自动适配
- **影响**: 个人使用体验 ⭐⭐⭐⭐
- **复杂度**: 低

#### 4. Prometheus 指标 (Metrics Export)
- **状态**: ✅ 已实现（基础版）
- **描述**: 已提供 `/metrics`（Prometheus exposition）；`/api/metrics` 仍为 JSON
- **实现方式**:
  - 提供 `/metrics`（Prometheus exposition）
  - 需要补充时可扩展为标准 `go_*/process_*` 全量指标
  - 关键指标: 连接数、流量、错误率、会话状态
  - Go runtime 指标
- **影响**: 可观测性 ⭐⭐⭐⭐⭐
- **复杂度**: 低

#### 5. WebSocket 支持 (WS Proxy)
- **状态**: ✅ 已实现（基础版：代理 Upgrade；不解析 WS 帧）
- **描述**: 代理支持 WebSocket 协议升级
- **实现方式**:
  - HTTP 代理支持 Upgrade: websocket
  - 升级后切换为原始 TCP 双向转发（不解析 WS 帧，避免影响现有 HTTP 审计解析）
- **影响**: 现代应用兼容性 ⭐⭐⭐⭐⭐
- **复杂度**: 中

---

### 🎯 P1 - 逐步增强 (Medium Impact)

#### 6. 日志搜索与过滤 (Log Search & Filter)
- **描述**: Web UI 和 CLI 增强日志检索能力
- **实现方式**:
  - 关键词搜索 (URL, Host, Method)
  - 时间范围筛选
  - HTTP 状态码过滤
  - 正则表达式支持
- **影响**: 问题排查效率 ⭐⭐⭐⭐
- **复杂度**: 中

#### 7. IP 白名单/黑名单 (Access Control)
- **描述**: 基于 IP 的访问控制
- **实现方式**:
  - 配置文件/数据库存储规则
  - 支持 CIDR 格式
  - 在 accept 阶段检查
- **影响**: 安全性增强 ⭐⭐⭐⭐
- **复杂度**: 低

#### 8. 配置导入/导出 (Config Migration)
- **描述**: 导出/导入整个配置
- **实现方式**:
  - JSON/YAML 格式
  - 命令行参数: `--export-config`, `--import-config`
  - Web UI 导出按钮
  - 包含: Bastions + Mappings + 设置
- **影响**: 多设备同步 ⭐⭐⭐⭐
- **复杂度**: 低

---

### 🎯 P2 - 可选增强 (Lower Priority, Optional)

#### 9. 自动备份 (Auto Backup)
- **描述**: 定时备份配置和审计数据
- **实现方式**:
  - Cron 表达式配置
  - 备份到本地/S3/文件
  - 保留策略 (N 天/个)
- **影响**: 数据安全 ⭐⭐⭐
- **复杂度**: 中

#### 10. 邮件/Webhook 告警 (Alerting)
- **描述**: 异常事件通知
- **实现方式**:
  - SMTP 邮件通知
  - Webhook (钉钉/飞书/Slack)
  - 告警规则: 连接失败、高并发、资源超限
- **影响**: 运维自动化 ⭐⭐⭐
- **复杂度**: 中

#### 11. 实时流量图表 (Traffic Dashboard)
- **描述**: Web UI 实时显示流量统计
- **实现方式**:
  - WebSocket 推送指标
  - ECharts/Chart.js 可视化
  - 速率、连接数、错误率图表
- **影响**: 可视化监控 ⭐⭐⭐
- **复杂度**: 中

---

## 详细功能规划

### 🔐 安全增强

| 功能 | 描述 | 复杂度 | 状态 |
|------|------|--------|------|
| **2FA 认证** | 管理界面 TOTP 支持 | 中 | 计划中 |
| **访问日志审计** | 记录管理界面操作 | 低 | 待实施 |
| **HTTPS 管理界面** | TLS 加密 Web UI | 低 | 待实施 |
| **会话超时强制断开** | 配置化强制超时 | 低 | 待实施 |

### 🌐 代理协议扩展

| 功能 | 描述 | 复杂度 | 状态 |
|------|------|--------|------|
| **HTTP/2 支持** | H2 协议解析 | 中 | 远期 |
| **HTTPS MITM 解密** | SSL 拦截审计 | 高 | 远期 |
| **UDP 转发** | SOCKS5 UDP 关联 | 中 | 计划中 |
| **QUIC 支持** | HTTP/3 代理 | 高 | 探索中 |

### ⚙️ 运维增强

| 功能 | 描述 | 复杂度 | 状态 |
|------|------|--------|------|
| **健康检查 API** | 详细的健康报告 | 低 | 已实现 |
| **连接池可视化** | 显示 SSH 连接状态 | 中 | 待实施 |
| **日志轮转** | 支持 logrotate | 低 | 待实施 |
| **配置校验** | 预检配置语法 | 低 | 待实施 |

### 🚀 高级特性

| 功能 | 描述 | 复杂度 | 状态 |
|------|------|--------|------|
| **连接多路复用** | 单 SSH 连接多路复用 | 高 | 远期 |
| **智能路由** | 基于域名/IP 的路由 | 中 | 远期 |
| **负载均衡** | 后端节点负载均衡 | 中 | 远期 |
| **插件系统** | 外部扩展支持 | 高 | 探索中 |

---

## 其他扩展方向

### 企业级集成
- **LDAP / Active Directory** 用户认证
- **OAuth2** (GitHub/Google/GitLab)
- **SAML** SSO 支持
- **Vault** 密钥管理

### 云原生支持
- **Kubernetes Operator**
- **Helm Chart** 部署
- **Service Mesh** 集成 (Istio)
- **云存储导出** (S3/Azure/GCS)

### 数据持久化
- **SQLite → PostgreSQL/MySQL** 多用户支持
- **审计日志归档** 到对象存储
- **日志分析** (Elasticsearch 集成)

### 开发者体验
- **OpenAPI/Swagger** 文档
- **SDK** (Python/Go/Node.js)
- **测试覆盖率** > 80%
- **CI/CD** GitHub Actions

---

## 实施建议

### 短期 (1-2 周)
1. ✅ 运行中配置不可修改（API 409 + 先 stop 再改）
2. ⏳ Docker 支持
3. ⏳ 暗色主题

### 中期 (1 个月)
4. ✅ Metrics：Prometheus `/metrics`
5. ⏳ WebSocket 支持
6. ⏳ 日志搜索过滤

### 长期 (3 个月+)
7. ⏳ 高级安全特性
8. ⏳ 企业集成
9. ⏳ 插件系统

---

## 贡献指南

### 分支策略
- `main` - 稳定版本
- `feature/xxx` - 功能开发分支
- `hotfix/xxx` - 紧急修复

### 开发流程
1. Fork 项目
2. 创建 feature 分支
3. 编写代码和测试
4. 提交 PR
5. Code Review
6. 合并

### 测试要求
- 单元测试覆盖率 > 80%
- 集成测试 (核心流程)
- 手动测试 (Web UI/CLI)

---

## 📝 更新日志

| 日期 | 修改人 | 内容 |
|------|--------|------|
| 2025-12-17 | Initial | 创建 TODO.md |

---

**状态说明**:
- ⏳ 待实施
- 🚧 开发中
- ✅ 已完成
- 📅 计划中
- 🕐 暂停

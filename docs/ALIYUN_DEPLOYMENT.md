# Aliyun Deployment Guide

阿里云服务器上的 New API 部署文档，包含架构、配置、更新流程和常见问题。

## 服务器信息

| 项目 | 值 |
|------|-----|
| IP | 47.117.247.155 |
| 系统 | Debian 13 |
| 内存 | 1.6GB（无 swap） |
| 磁盘 | 40GB |
| SSH | ssh aliyun |

## 架构

systemd 直运行二进制 + Docker(PG/Redis)，通过 bridge 网络互联。

- new-api: systemd, port 3000, /opt/new-api/new-api-bin (88M static)
- PostgreSQL 15: Docker, 172.19.0.2:5432, root/newapi2025
- Redis 8.8: Docker, 172.19.0.3:6379, 密码 newapi2025
- 网络: Docker bridge new-api-network

> Docker 网络 IP 硬编码，重建容器可能漂移。

## 目录结构

- /opt/new-api/new-api-bin — 主程序
- /opt/new-api/data — 数据目录
- /opt/new-api/logs — 日志目录
- /etc/systemd/system/new-api.service — systemd 服务
- /opt/new-api.bak.20260613 — 旧部署备份

## Systemd 配置

```ini
[Unit]
Description=New API Service
After=network.target

[Service]
Type=simple
WorkingDirectory=/opt/new-api
ExecStart=/opt/new-api/new-api-bin
Restart=always
RestartSec=5
Environment=SQL_DSN=root:newapi2025@tcp(172.19.0.2:5432)/new-api
Environment=REDIS_CONN_STRING=redis://:newapi2025@172.19.0.3:6379
Environment=SESSION_SECRET=random_secret_here
# Environment=ERROR_LOG_ENABLED=true

[Install]
WantedBy=multi-user.target
```

常用命令: systemctl start/stop/restart/status new-api

## 渠道配置

- 渠道 2/4: 讯飞CodingPlan, Type 8 (Custom)
- base_url: https://maas-coding-api.cn-huabei-1.xf-yun.com/v2
- 模型: glm-5.1, kimi-k2.6, claude-haiku-4-5, claude-sonnet-4-5
- 映射: glm-5.1->xopglm51, kimi-k2.6->xopkimik26, claude-*->xopkimik26

> Custom 类型自动追加 /chat/completions，base_url 不要包含此路径。

## 协议适配端点

| 端点 | 用途 |
|------|------|
| POST /v1/codex/responses | Codex CLI (Responses API) |
| GET /v1/codex/models | Codex CLI 模型发现 |
| POST /v1/messages | Claude Code CLI (Anthropic Messages) |

### Codex CLI 配置

```toml
# ~/.codex/config.toml
base_url = "http://47.117.247.155:3000/v1/codex"
wire_api = "responses"
model = "glm-5.1"
model_context_window = 200000
```

### Claude Code 配置

```json
{
  "model": "sonnet",
  "ANTHROPIC_BASE_URL": "http://47.117.247.155:3000",
  "ANTHROPIC_API_KEY": "sk-xxx",
  "ANTHROPIC_CUSTOM_MODEL_OPTION": "glm-5.1"
}
```

> ANTHROPIC_BASE_URL 必须是根 URL，Claude Code 自动追加 /v1/messages。

## 自定义扩展模块

custom/ 目录包含解耦扩展，不修改上游核心代码：

- protocol_adapter: Codex/Claude 协议适配
- token_config: Internal Token 动态刷新与注入
- TokenTemplate: Token 模板（管理员创建，用户选择）

### Token API

| 路由 | 方法 | 权限 | 功能 |
|------|------|------|------|
| /api/user/token-config/ | GET | 用户 | 获取 token 列表 |
| /api/user/token-config/ | POST | 用户 | 创建 token |
| /api/user/token-config/:id | PUT | 用户 | 更新 token |
| /api/user/token-config/:id | DELETE | 用户 | 删除 token |
| /api/user/token-config/:id/refresh | POST | 用户 | 手动刷新 |
| /api/user/token-config/templates | GET | 用户 | 模板列表 |
| /api/user/token-config/templates | POST | 管理员 | 创建模板 |
| /api/user/token-config/templates/:id | PUT | 管理员 | 更新模板 |
| /api/user/token-config/templates/:id | DELETE | 管理员 | 删除模板 |

Headers 中使用 ${token:NAME} 引用 token 值。

## 构建与部署流程

### 前提

- 本地源码: /home/oyhx/github/new-api
- 仓库: origin=gitee, github=github
- 阿里云无 Go 编译器 -> 必须本地 Docker 构建
- 前端 go:embed 编译进二进制

### 一键部署

```bash
#!/bin/bash
set -e
cd ~/github/new-api
docker build -t new-api:local-build .
docker create --name tmp-new-api new-api:local-build
docker cp tmp-new-api:/new-api /tmp/new-api-binary
docker rm tmp-new-api
ssh aliyun 'systemctl stop new-api'
scp /tmp/new-api-binary aliyun:/opt/new-api/new-api-bin
ssh aliyun 'systemctl start new-api'
sleep 2 && ssh aliyun 'systemctl is-active new-api'
```

> 首次构建用 --no-cache，日常用缓存。中国网络下 --no-cache 可能超时。

### 数据库操作

```bash
# 查询
ssh aliyun "docker exec postgres psql -U root -d new-api -c 'SELECT key, value FROM options;'"

# 更新
ssh aliyun "docker exec postgres psql -U root -d new-api -c \"UPDATE options SET value = '10' WHERE key = 'RetryTimes';\""
```

## 常见问题

### 讯飞 Engine Busy 不重试

原因: 讯飞 MaaS 返回 HTTP 200 + error body，shouldRetry() 看到 2xx 不重试。
修复: openAIErrorTypeToStatusCode() 根据 error type 映射 HTTP 状态码，
server_error -> 503, rate_limit_error -> 429, invalid_request_error -> 400 等。

### Channel base_url 路径重复

Custom 类型自动追加 /chat/completions，base_url 不要包含此路径。
例如用 https://example.com/v2 而不是 https://example.com/v2/chat/completions。

### Claude Code 连接失败

ANTHROPIC_BASE_URL 必须是根 URL，Claude Code 自动追加 /v1/messages。
不要用 ANTHROPIC_DEFAULT_SONNET_MODEL（仅 Bedrock/Vertex/Foundry 有效），
用 ANTHROPIC_CUSTOM_MODEL_OPTION + channel model_mapping 代替。

### 多实例 Cookie 冲突

同一服务器运行多个 new-api 实例时，SESSION_SECRET 相同会导致 cookie 冲突。
修复: 动态 cookie 名，基于 PORT 环境变量生成唯一名称。

### TokenRevealEnabled 控制

管理员在系统设置 > 运维 > 系统行为中开启 Token Reveal 开关，
用户才能在 Internal Token 页面看到查看完整 token 的按钮。

# UAPI

Your Unified AI API Gateway.

UAPI 是一个统一的 AI API 网关，支持 OpenAI、Anthropic、Google Gemini 等多家大模型服务商。通过 UAPI，你可以用一套 API 接口管理所有上游渠道，统一鉴权、计费和流量调度。

## 特性

- **多供应商支持** — OpenAI (Chat Completions / Responses API)、Anthropic Messages、Google Gemini，统一转为内部格式并按需互转
- **OpenAI 兼容接口** — 下游客户端只需对接 `/v1/chat/completions`，即可路由到任意供应商
- **多账号池 & 加权轮询** — 同一渠道可挂载多个上游账号，按权重自动调度
- **API Key 管理** — 用户自助创建密钥，支持 IP 白名单、过期时间、模型限制和端点权限
- **用量计费** — 预扣费 / 结算 / 退款，按 token 精确计量
- **管理后台** — 渠道管理、账号管理、用户管理、套餐管理、操作审计
- **用户控制台** — 注册登录、密钥管理、用量查询、套餐订阅
- **OAuth 接入** — 支持 OpenAI / Gemini OAuth 授权流程，自动刷新 Token
- **流式转发** — SSE 流式响应透明转发，支持流式转非流式

## 快速开始

### 前置条件

- Go 1.26+
- PostgreSQL 17+
- Node.js 20+ (前端)

### 启动后端

```bash
# 配置数据库
cp config.example.yaml config.yaml
# 编辑 config.yaml，填入数据库连接和密钥

# 启动开发数据库
make dev

# 编译运行
make build
./bin/uapi -config config.yaml
```

### 启动前端

```bash
npm --prefix web install
npm --prefix web run dev
```

生产构建：

```bash
npm --prefix web run build
npm --prefix web run serve:static
```

## 项目结构

```
cmd/uapi/          程序入口
internal/
  server/          HTTP 服务器 & 路由
  relay/           核心中继引擎 (调度、计费、流式)
    provider/      上游适配器 (OpenAI / Anthropic / Gemini)
  admin/           管理后台 API
  user/            用户系统 API
  auth/            JWT 认证
  db/              数据模型 (GORM)
  crypto/          AES-256-GCM 加密
  config/          配置加载
web/               Next.js 前端
docs/              项目文档
```

## API 概览

| 路径前缀 | 说明 |
|----------|------|
| `/v1/*` | 中继接口 (OpenAI 兼容) |
| `/api/user/*` | 用户 API |
| `/api/admin/*` | 管理后台 API |

## 技术栈

- **后端**: Go / fasthttp / GORM / PostgreSQL / JWT (HS256) / AES-256-GCM
- **前端**: Next.js 15 / React / TypeScript / 纯 CSS

## 部署

参见 [docs/deployment/nginx.md](docs/deployment/nginx.md) 了解 Nginx 反向代理和 Systemd 部署配置。

Docker Compose 一键部署：

```bash
docker compose up -d
```

## 文档

- [文档索引](docs/README.md)
- [项目交接文档](docs/current/handoff.md)
- [前端文档](docs/current/frontend.md)
- [平台设计](docs/current/platform-design.md)

## License

Private. All rights reserved.

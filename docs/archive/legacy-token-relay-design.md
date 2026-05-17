> ARCHIVED / SUPERSEDED. Do not use this document for current implementation
> decisions. Start with `docs/current/handoff.md` and the active docs under
> `docs/current/`.

# cli-relay 设计文档

## 项目目标

常驻后台的 Go 服务，逆向模拟多个 AI CLI 工具的认证流程，自动获取和维护 token，对下游提供稳定的 HTTP 接口。

## 架构概览

```
┌─────────────────────────────────────────┐
│            cli-relay Server             │
│                                         │
│  ┌──────────┐ ┌──────────┐ ┌─────────┐ │
│  │  Codex   │ │  Gemini  │ │  Kilo   │ │  ← Provider 插件化
│  │ Provider │ │ Provider │ │Provider │ │
│  └────┬─────┘ └────┬─────┘ └───┬─────┘ │
│       └──────┬──────┘───────────┘       │
│         Token Manager                   │
│     (自动刷新 / 过期检测 / 401 recovery) │
│              │                          │
│         Token Store                     │
│     (JSON 文件，兼容原 CLI 格式)         │
│              │                          │
│         HTTP API                        │
│  GET  /api/v1/providers/{name}/token    │
│  POST /api/v1/providers/{name}/login    │
│  ...                                    │
└──────────────┬──────────────────────────┘
               │
         下游客户端
     (固定接口, 无需关心 token 刷新)
```

## 项目结构

```
cli-relay/
├── upstream/                        # 上游 CLI 源码参考
├── docs/                            # 设计文档
├── cmd/
│   └── cli-relay/
│       └── main.go                  # 入口
├── internal/
│   ├── provider/
│   │   ├── provider.go              # Provider 接口 + TokenSet + AuthMethod
│   │   └── codex/
│   │       ├── codex.go             # Codex provider 实现
│   │       ├── oauth.go             # 浏览器 OAuth2+PKCE + token-exchange grant
│   │       ├── device.go            # 设备码流程
│   │       ├── refresh.go           # Token 刷新 + 错误分类
│   │       ├── storage.go           # auth.json 读写（兼容原 Codex CLI 格式）
│   │       ├── revoke.go            # Token 撤销
│   │       └── jwt.go               # JWT 解析（exp, account_id, fedramp claims）
│   ├── store/
│   │   ├── store.go                 # TokenStore 接口
│   │   └── file.go                  # JSON 文件存储实现
│   ├── manager/
│   │   ├── manager.go               # TokenManager：统一调度 + recovery
│   │   └── scheduler.go             # 后台定时刷新
│   ├── server/
│   │   ├── server.go                # HTTP server + 路由
│   │   └── handlers.go              # API handler
│   └── config/
│       └── config.go                # 配置结构 + 加载
├── config.example.yaml
├── go.mod
└── Makefile
```

## Core Interfaces

### Provider 接口

```go
type AuthMethod string  // "browser" | "device_code" | "api_key"

type TokenSet struct {
    AccessToken  string            // Bearer token
    RefreshToken string            // 刷新令牌
    IDToken      string            // JWT (OpenAI 专用)
    APIKey       string            // 静态 API key
    AccountID    string            // 工作区/账号 ID
    ExpiresAt    *time.Time        // JWT exp 或服务端返回
    Scopes       []string
    LastRefresh  *time.Time
    ExtraHeaders map[string]string // 下游需附加的 header
}

type Provider interface {
    Name() string
    SupportedMethods() []AuthMethod
    Login(ctx context.Context, method AuthMethod) (*TokenSet, error)
    Refresh(ctx context.Context, tokens *TokenSet) (*TokenSet, error)
    Validate(ctx context.Context, tokens *TokenSet) (bool, error)
    Revoke(ctx context.Context, tokens *TokenSet) error
}
```

### TokenStore 接口

```go
type TokenStore interface {
    Load(ctx context.Context, providerName string) (*provider.TokenSet, error)
    Save(ctx context.Context, providerName string, tokens *provider.TokenSet) error
    Delete(ctx context.Context, providerName string) error
}
```

## HTTP API

```
GET    /api/v1/providers                    # 列出所有 provider 状态
GET    /api/v1/providers/{name}/token       # 获取有效 token（自动刷新）
POST   /api/v1/providers/{name}/login       # 发起登录 {"method": "device_code"}
DELETE /api/v1/providers/{name}/token        # 登出（撤销+删除）
POST   /api/v1/providers/{name}/refresh     # 强制刷新
GET    /api/v1/health                       # 健康检查
```

### GET /api/v1/providers/{name}/token 响应

```json
{
  "access_token": "eyJ...",
  "token_type": "Bearer",
  "account_id": "acct_xxx",
  "expires_at": "2026-04-19T11:30:00Z",
  "extra_headers": {
    "ChatGPT-Account-ID": "acct_xxx"
  }
}
```

### POST /api/v1/providers/{name}/login (设备码)

```json
// Request:
{"method": "device_code"}

// Response 202:
{"status": "pending", "verification_url": "...", "user_code": "ABCD-1234"}

// 客户端轮询 GET /providers/{name}/token 直到返回 200
```

## 配置格式

```yaml
server:
  listen: "127.0.0.1:9876"

providers:
  codex:
    enabled: true
    auth_method: browser
    issuer: "https://auth.openai.com"
    proactive_refresh_age: 192h    # 8天
    refresh_buffer: 5m

  gemini:
    enabled: false
    auth_method: browser

  kilocode:
    enabled: false
    auth_method: device_code

refresh:
  check_interval: 1m
  max_retries: 3
  retry_backoff: 30s

log:
  level: info
```

## Token 生命周期

```
登录 → 存储 → 定时检查 → 主动刷新 → 持续可用
                ↓
          401 被动触发 → Recovery 状态机
                ↓
          Reload → Refresh → 完成/失败
```

1. **登录**：用户调用 login API，选择浏览器/设备码/API key
2. **定时检查**：Scheduler 每 `check_interval` 检查所有 provider
3. **主动刷新**：token 过期 或 last_refresh > `proactive_refresh_age`
4. **被动刷新**：下游报告 401 时触发 recovery 流程
5. **错误分类**：
   - 永久错误（token expired/reused/revoked）：缓存，不重试
   - 临时错误（网络/5xx）：指数退避重试

## 401 Recovery 状态机

```
401 received
    │
    v
[Step 1: Reload]
Check on-disk tokens
Are they different/newer?
   /            \
  Yes            No
  |               |
  v               v
[Use new     [Step 2: Refresh]
 tokens]     Call provider.Refresh()
               /            \
           Success        Failure
              |              |
              v              v
         [Use new       [Exhausted]
          tokens]       Return error
```

## 实现阶段

### Phase 1: 骨架 + Codex Provider（当前）

1. 初始化 Go module，建立目录结构
2. 实现 config 包（YAML 配置加载）
3. 实现 provider 接口 + TokenSet
4. 实现 store/file.go（JSON 文件存储）
5. 实现 codex/jwt.go（JWT 解析）
6. 实现 codex/oauth.go（浏览器 OAuth2+PKCE）
7. 实现 codex/device.go（设备码流程）
8. 实现 codex/refresh.go（刷新 + 错误分类）
9. 实现 codex/storage.go（auth.json 读写）
10. 实现 codex/revoke.go（Token 撤销）
11. 实现 codex/codex.go（组装 Provider 接口）
12. 实现 manager 包（TokenManager + Scheduler）
13. 实现 server 包（HTTP API）
14. 实现 cmd/cli-relay/main.go
15. 编写 config.example.yaml + Makefile

### Phase 2: Gemini CLI Provider

### Phase 3: Kilocode Provider

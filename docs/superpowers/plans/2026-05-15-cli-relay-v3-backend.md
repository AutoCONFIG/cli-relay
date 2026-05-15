# CLI Relay v3 — 后端核心重构实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.
>
> **执行方式：** 使用 Subagent-Driven Development。每个 Task 派一个独立 subagent 执行，主 agent 在 Task 间做 review。新会话启动时，读取本文件和 spec 即可接续工作，无需额外上下文。

**Goal:** 将现有 cli-relay 后端从单体结构重构为分层架构，新增用户系统、路由器、中间件链、中间格式转换，同时保留核心中转引擎的性能。

**Architecture:** 保留 fasthttp 内核，将手动 switch 路由替换为轻量路由器，实现中间件管道，admin handler 按资源拆分，新增 user 包处理用户注册/登录/管理。引入统一中间格式 `InternalRequest`/`InternalResponse` 用于 4 种 API 格式互转。中转路径保持最短中间件链确保极致性能。

**Tech Stack:** Go 1.26, fasthttp, GORM/PostgreSQL, JWT (HS256), AES-256-GCM

**Spec:** `docs/superpowers/specs/2026-05-15-cli-relay-v3-platform-design.md`

**当前进度：** 未开始。所有 Task 均为 pending。

---

## 文件结构

### 新建文件

| 文件 | 职责 |
|------|------|
| `internal/server/router.go` | 路由注册，替代 switch |
| `internal/server/middleware.go` | CORS、请求日志、限流中间件 |
| `internal/auth/jwt.go` | JWT 生成/验证（双系统：admin + user） |
| `internal/auth/middleware.go` | JWT 认证中间件（admin 和 user 两套） |
| `internal/user/handler.go` | 用户 API handler |
| `internal/user/service.go` | 用户业务逻辑 |
| `internal/user/dto.go` | 用户请求/响应 DTO |
| `internal/admin/handler.go` | Admin 路由分发 |
| `internal/admin/channel_handler.go` | 渠道 CRUD |
| `internal/admin/account_handler.go` | 账号 CRUD |
| `internal/admin/token_handler.go` | 令牌 CRUD |
| `internal/admin/plan_handler.go` | 套餐 CRUD |
| `internal/admin/user_handler.go` | 用户管理 |
| `internal/admin/log_handler.go` | 日志查询 |
| `internal/admin/dto.go` | Admin DTO |
| `internal/db/user.go` | User 模型 |
| `internal/db/redeem_code.go` | 充值码模型 |
| `internal/relay/provider/types.go` | Adaptor 接口 + 中间格式定义 |
| `internal/relay/provider/credentials.go` | 凭证提取（从 types/credentials.go 迁移） |
| `internal/relay/provider/convert.go` | 中间格式转换器注册和调度 |
| `internal/relay/provider/openai/adaptor.go` | OpenAI adaptor（迁移） |
| `internal/relay/provider/openai/responses.go` | Responses API 转换（迁移） |
| `internal/relay/provider/openai/to_internal.go` | OpenAI → Internal |
| `internal/relay/provider/openai/from_internal.go` | Internal → OpenAI |
| `internal/relay/provider/openai/auth.go` | OpenAI OAuth |
| `internal/relay/provider/anthropic/adaptor.go` | Anthropic adaptor（迁移） |
| `internal/relay/provider/anthropic/streaming.go` | Anthropic SSE（迁移） |
| `internal/relay/provider/anthropic/to_internal.go` | Anthropic → Internal |
| `internal/relay/provider/anthropic/from_internal.go` | Internal → Anthropic |
| `internal/relay/provider/anthropic/auth.go` | Anthropic OAuth（预留） |
| `internal/relay/provider/gemini/adaptor.go` | Gemini adaptor（迁移） |
| `internal/relay/provider/gemini/streaming.go` | Gemini SSE（迁移） |
| `internal/relay/provider/gemini/to_internal.go` | Gemini → Internal |
| `internal/relay/provider/gemini/from_internal.go` | Internal → Gemini |
| `internal/relay/provider/gemini/auth.go` | Google OAuth |
| `internal/relay/account_refresh.go` | OAuth token 自动刷新逻辑 |

### 修改文件

| 文件 | 变更 |
|------|------|
| `internal/server/server.go` | 使用 router.go 注册路由，删除 switch |
| `internal/relay/handler.go` | 支持 4 种入口格式，集成中间格式转换 |
| `internal/relay/pool.go` | Account 结构适配 OAuth 字段 |
| `internal/relay/billing.go` | 关联 User 维度 |
| `internal/db/db.go` | 新增 User、RedeemCode 模型，Account 扩展字段 |
| `internal/db/account.go` | 从 db.go 拆出，扩展 OAuth 字段 |
| `internal/db/channel.go` | 从 db.go 拆出 |
| `internal/db/token.go` | 从 db.go 拆出，新增 UserID 字段 |
| `internal/db/plan.go` | 从 db.go 拆出 |
| `internal/db/log.go` | 从 db.go 拆出 |
| `internal/db/audit_log.go` | 从 db.go 拆出 |
| `internal/config/config.go` | 扩展用户系统配置 |
| `cmd/relay/main.go` | 适配新架构 |

### 删除文件

| 文件 | 原因 |
|------|------|
| `internal/relay/types/adaptor.go` | 迁移到 `provider/types.go` |
| `internal/relay/types/credentials.go` | 迁移到 `provider/credentials.go` |
| `internal/relay/openai/` | 迁移到 `provider/openai/` |
| `internal/relay/anthropic/` | 迁移到 `provider/anthropic/` |
| `internal/relay/gemini/` | 迁移到 `provider/gemini/` |
| `internal/web/embed.go` | 前端分离后不再嵌入 |
| `internal/web/index.html` | 前端独立项目 |

---

## Task 1: 数据模型重构

**Files:**
- Modify: `internal/db/db.go`
- Create: `internal/db/user.go`
- Create: `internal/db/account.go`
- Create: `internal/db/channel.go`
- Create: `internal/db/token.go`
- Create: `internal/db/plan.go`
- Create: `internal/db/log.go`
- Create: `internal/db/audit_log.go`
- Create: `internal/db/redeem_code.go`

- [ ] **Step 1: 将 db.go 中的模型拆分为独立文件**

将 `Channel`, `Account`, `Token`, `Plan`, `TokenPlan`, `Log`, `AuditLog` 从 `db.go` 拆分到各自文件，`db.go` 只保留 `Base`、`InitDB()`、`AutoMigrate`。

- `internal/db/channel.go`: `Channel` 模型
- `internal/db/account.go`: `Account` 模型，扩展 OAuth 字段：

```go
type Account struct {
    Base
    ChannelID     string     `gorm:"index;not null"`
    Name          string     `gorm:"not null"`
    Credentials   string     `gorm:"type:text;not null"`  // AES 加密
    CredType      string     `gorm:"default:api_key"`      // api_key | oauth_token
    Weight        int        `gorm:"default:1"`
    Enabled       bool       `gorm:"default:true"`
    CooldownUntil *time.Time
    RefreshToken  string     `gorm:"type:text"`            // AES 加密（oauth_token 时使用）
    TokenExpiry   *time.Time                                // access_token 过期时间
    ClientID      string     `gorm:"type:text"`            // OAuth client ID
    TokenURL      string     `gorm:"type:text"`            // OAuth token endpoint
}
```

- `internal/db/token.go`: `Token` 模型，新增 `UserID` 字段：

```go
type Token struct {
    Base
    UserID       string     `gorm:"index"`                 // 关联 User
    Name         string     `gorm:"not null"`
    Key          string     `gorm:"uniqueIndex;not null"`
    PlanID       string
    IPWhitelist  string     `gorm:"type:text"`
    Unlimited    bool       `gorm:"default:false"`
    Enabled      bool       `gorm:"default:true"`
}
```

- `internal/db/plan.go`: `Plan`, `TokenPlan`
- `internal/db/log.go`: `Log`
- `internal/db/audit_log.go`: `AuditLog`

- [ ] **Step 2: 创建 User 模型**

`internal/db/user.go`:

```go
package db

type User struct {
    Base
    Email        string `gorm:"uniqueIndex;not null"`
    Username     string `gorm:"uniqueIndex;not null"`
    PasswordHash string `gorm:"not null"`
    Status       string `gorm:"default:active"`  // active, disabled
    Balance      int64  `gorm:"default:0"`       // 余额（token 单位）
}
```

- [ ] **Step 3: 创建 RedeemCode 模型**

`internal/db/redeem_code.go`:

```go
package db

import "time"

type RedeemCode struct {
    Base
    Code      string     `gorm:"uniqueIndex;not null"`
    Value     int64      `gorm:"not null"`
    UsedBy    *string    `gorm:"index"`
    UsedAt    *time.Time
    Status    string     `gorm:"default:active"`  // active, used, expired
    ExpiresAt time.Time  `gorm:"not null"`
}
```

- [ ] **Step 4: 更新 db.go 的 AutoMigrate**

在 `InitDB()` 中添加 `&User{}`, `&RedeemCode{}` 到 AutoMigrate 列表。

- [ ] **Step 5: 编译验证**

Run: `go build ./...`
Expected: 编译通过

- [ ] **Step 6: Commit**

```bash
git add internal/db/
git commit -m "refactor: split db models into separate files, add User and RedeemCode"
```

---

## Task 2: 路由器和中间件

**Files:**
- Create: `internal/server/router.go`
- Create: `internal/server/middleware.go`
- Modify: `internal/server/server.go`

- [ ] **Step 1: 实现轻量路由器**

`internal/server/router.go` — 基于 fasthttp 的前缀匹配路由器，支持参数提取（`:id`）：

```go
package server

import "github.com/valyala/fasthttp"

type route struct {
    method  string
    path    string
    handler fasthttp.RequestHandler
}

type Router struct {
    routes []route
}

func NewRouter() *Router {
    return &Router{}
}

func (r *Router) GET(path string, handler fasthttp.RequestHandler) {
    r.routes = append(r.routes, route{"GET", path, handler})
}

func (r *Router) POST(path string, handler fasthttp.RequestHandler) {
    r.routes = append(r.routes, route{"POST", path, handler})
}

func (r *Router) PUT(path string, handler fasthttp.RequestHandler) {
    r.routes = append(r.routes, route{"PUT", path, handler})
}

func (r *Router) DELETE(path string, handler fasthttp.RequestHandler) {
    r.routes = append(r.routes, route{"DELETE", path, handler})
}

// Lookup 返回匹配的 handler 和提取的路径参数
func (r *Router) Lookup(method, path string) (fasthttp.RequestHandler, map[string]string) {
    for _, rt := range r.routes {
        if rt.method != method {
            continue
        }
        if params, ok := matchPath(rt.path, path); ok {
            return rt.handler, params
        }
    }
    return nil, nil
}

// matchPath 支持 :param 占位符
func matchPath(pattern, path string) (map[string]string, bool) {
    // 精确匹配
    if pattern == path {
        return nil, true
    }
    // 前缀匹配（用于 /v1/* 通配）
    if len(pattern) > 0 && pattern[len(pattern)-1] == '*' {
        prefix := pattern[:len(pattern)-1]
        if len(path) >= len(prefix) && path[:len(prefix)] == prefix {
            return nil, true
        }
    }
    // 参数匹配（如 /api/admin/channels/:id）
    params := make(map[string]string)
    pi, pa := 0, 0
    for pi < len(pattern) && pa < len(path) {
        if pattern[pi] == ':' {
            // 提取参数名
            j := pi + 1
            for j < len(pattern) && pattern[j] != '/' {
                j++
            }
            paramName := pattern[pi+1 : j]
            // 提取参数值
            k := pa
            for k < len(path) && path[k] != '/' {
                k++
            }
            params[paramName] = path[pa:k]
            pi = j
            pa = k
        } else if pattern[pi] != path[pa] {
            return nil, false
        } else {
            pi++
            pa++
        }
    }
    if pi == len(pattern) && pa == len(path) {
        return params, true
    }
    return nil, false
}
```

- [ ] **Step 2: 实现中间件管道**

`internal/server/middleware.go`:

```go
package server

import "github.com/valyala/fasthttp"

// Middleware 中间件函数类型
type Middleware func(next fasthttp.RequestHandler) fasthttp.RequestHandler

// Chain 将多个中间件串联为一个 handler
func Chain(middlewares ...Middleware) Middleware {
    return func(final fasthttp.RequestHandler) fasthttp.RequestHandler {
        for i := len(middlewares) - 1; i >= 0; i-- {
            final = middlewares[i](final)
        }
        return final
    }
}

// CORSMiddleware 跨域中间件
func CORSMiddleware() Middleware {
    return func(next fasthttp.RequestHandler) fasthttp.RequestHandler {
        return func(ctx *fasthttp.RequestCtx) {
            ctx.Response.Header.Set("Access-Control-Allow-Origin", "*")
            ctx.Response.Header.Set("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
            ctx.Response.Header.Set("Access-Control-Allow-Headers", "Content-Type,Authorization")
            if string(ctx.Method()) == "OPTIONS" {
                ctx.SetStatusCode(204)
                return
            }
            next(ctx)
        }
    }
}

// RequestLoggerMiddleware 请求日志中间件
func RequestLoggerMiddleware() Middleware {
    return func(next fasthttp.RequestHandler) fasthttp.RequestHandler {
        return func(ctx *fasthttp.RequestCtx) {
            // 仅记录 API 请求，不记录中转请求（中转路径不走此中间件）
            next(ctx)
        }
    }
}
```

- [ ] **Step 3: 重构 server.go 使用路由器**

修改 `internal/server/server.go`，将 switch 替换为路由器查找：

```go
func (s *Server) handler(ctx *fasthttp.RequestCtx) {
    path := string(ctx.Path())
    method := string(ctx.Method())

    // 中转路径 — 最短路径，不走路由器和中间件
    if strings.HasPrefix(path, "/v1/") {
        s.relayer.HandleRelay(ctx)
        return
    }

    // API 路径 — 走路由器和中间件
    handler, params := s.router.Lookup(method, path)
    if handler == nil {
        ctx.SetStatusCode(404)
        ctx.SetBodyString(`{"code":404,"message":"not found"}`)
        return
    }
    // 将路径参数注入 context
    for k, v := range params {
        ctx.SetUserValue(k, v)
    }
    handler(ctx)
}
```

注册路由的方法 `setupRoutes()`:

```go
func (s *Server) setupRoutes() {
    r := NewRouter()

    // Admin 认证（无需 JWT）
    r.POST("/api/admin/login", s.adminHandler.Login)
    r.GET("/api/admin/init-status", s.adminHandler.InitStatus)
    r.POST("/api/admin/setup", s.adminHandler.Setup)

    // Admin CRUD（需要 admin JWT）
    // 这些路由会在 admin handler 内部做 JWT 校验
    r.GET("/api/admin/dashboard", s.adminHandler.Dashboard)
    r.GET("/api/admin/channels", s.adminHandler.ListChannels)
    r.POST("/api/admin/channels", s.adminHandler.CreateChannel)
    r.PUT("/api/admin/channels", s.adminHandler.UpdateChannel)    // ?id=xxx
    r.DELETE("/api/admin/channels", s.adminHandler.DeleteChannel) // ?id=xxx
    r.GET("/api/admin/accounts", s.adminHandler.ListAccounts)
    r.POST("/api/admin/accounts", s.adminHandler.CreateAccount)
    r.PUT("/api/admin/accounts", s.adminHandler.UpdateAccount)
    r.DELETE("/api/admin/accounts", s.adminHandler.DeleteAccount)
    r.GET("/api/admin/tokens", s.adminHandler.ListTokens)
    r.POST("/api/admin/tokens", s.adminHandler.CreateToken)
    r.PUT("/api/admin/tokens", s.adminHandler.UpdateToken)
    r.DELETE("/api/admin/tokens", s.adminHandler.DeleteToken)
    r.GET("/api/admin/plans", s.adminHandler.ListPlans)
    r.POST("/api/admin/plans", s.adminHandler.CreatePlan)
    r.PUT("/api/admin/plans", s.adminHandler.UpdatePlan)
    r.DELETE("/api/admin/plans", s.adminHandler.DeletePlan)
    r.GET("/api/admin/users", s.adminHandler.ListUsers)
    r.PUT("/api/admin/users", s.adminHandler.UpdateUser)
    r.DELETE("/api/admin/users", s.adminHandler.DeleteUser)
    r.GET("/api/admin/logs", s.adminHandler.ListLogs)
    r.GET("/api/admin/audit-logs", s.adminHandler.ListAuditLogs)

    // 用户认证（无需 JWT）
    r.POST("/api/v1/auth/register", s.userHandler.Register)
    r.POST("/api/v1/auth/login", s.userHandler.Login)
    r.POST("/api/v1/auth/refresh", s.userHandler.RefreshToken)

    // 用户 API（需要 user JWT）
    r.GET("/api/v1/user/profile", s.userHandler.GetProfile)
    r.PUT("/api/v1/user/password", s.userHandler.UpdatePassword)
    r.PUT("/api/v1/user/email", s.userHandler.UpdateEmail)
    r.GET("/api/v1/user/keys", s.userHandler.ListKeys)
    r.POST("/api/v1/user/keys", s.userHandler.CreateKey)
    r.DELETE("/api/v1/user/keys", s.userHandler.DeleteKey) // ?id=xxx
    r.GET("/api/v1/user/usage", s.userHandler.GetUsage)
    r.GET("/api/v1/user/usage/logs", s.userHandler.GetUsageLogs)
    r.GET("/api/v1/plans", s.userHandler.ListPlans)
    r.GET("/api/v1/user/subscription", s.userHandler.GetSubscription)
    r.POST("/api/v1/user/subscription", s.userHandler.Subscribe)
    r.POST("/api/v1/user/redeem", s.userHandler.RedeemCode)

    s.router = r
}
```

- [ ] **Step 4: 编译验证**

Run: `go build ./...`
Expected: 编译失败（因为 admin/user handler 还没实现），确认路由器和中间件代码本身无误

- [ ] **Step 5: Commit**

```bash
git add internal/server/
git commit -m "feat: add lightweight router and middleware pipeline"
```

---

## Task 3: JWT 双系统认证

**Files:**
- Create: `internal/auth/jwt.go`
- Create: `internal/auth/middleware.go`
- Delete: `internal/auth/auth.go`（替换）

- [ ] **Step 1: 实现双系统 JWT**

`internal/auth/jwt.go`:

```go
package auth

import (
    "errors"
    "time"

    "github.com/golang-jwt/jwt/v5"
)

var (
    ErrInvalidToken = errors.New("invalid token")
    ErrExpiredToken = errors.New("token expired")
)

type TokenType string

const (
    TokenTypeAdmin TokenType = "admin"
    TokenTypeUser  TokenType = "user"
)

type Claims struct {
    jwt.RegisteredClaims
    UserID   string    `json:"uid,omitempty"`
    Username string    `json:"username"`
    Type     TokenType `json:"type"`
}

func GenerateToken(secret string, userID, username string, tokenType TokenType, expiry time.Duration) (string, error) {
    now := time.Now()
    claims := Claims{
        RegisteredClaims: jwt.RegisteredClaims{
            ExpiresAt: jwt.NewNumericDate(now.Add(expiry)),
            IssuedAt:  jwt.NewNumericDate(now),
        },
        UserID:   userID,
        Username: username,
        Type:     tokenType,
    }
    token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
    return token.SignedString([]byte(secret))
}

func ParseToken(secret string, tokenStr string) (*Claims, error) {
    token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (interface{}, error) {
        return []byte(secret), nil
    })
    if err != nil {
        if errors.Is(err, jwt.ErrTokenExpired) {
            return nil, ErrExpiredToken
        }
        return nil, ErrInvalidToken
    }
    claims, ok := token.Claims.(*Claims)
    if !ok || !token.Valid {
        return nil, ErrInvalidToken
    }
    return claims, nil
}
```

- [ ] **Step 2: 实现认证中间件**

`internal/auth/middleware.go`:

```go
package auth

import (
    "encoding/json"
    "strings"

    "github.com/valyala/fasthttp"
)

// RequireAdmin 创建 admin JWT 认证中间件
func RequireAdmin(secret string) func(fasthttp.RequestHandler) fasthttp.RequestHandler {
    return requireToken(secret, TokenTypeAdmin)
}

// RequireUser 创建 user JWT 认证中间件
func RequireUser(secret string) func(fasthttp.RequestHandler) fasthttp.RequestHandler {
    return requireToken(secret, TokenTypeUser)
}

func requireToken(secret string, expectedType TokenType) func(fasthttp.RequestHandler) fasthttp.RequestHandler {
    return func(next fasthttp.RequestHandler) fasthttp.RequestHandler {
        return func(ctx *fasthttp.RequestCtx) {
            authHeader := string(ctx.Request.Header.Peek("Authorization"))
            if !strings.HasPrefix(authHeader, "Bearer ") {
                writeUnauthorized(ctx, "missing authorization header")
                return
            }
            tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
            claims, err := ParseToken(secret, tokenStr)
            if err != nil {
                writeUnauthorized(ctx, err.Error())
                return
            }
            if claims.Type != expectedType {
                writeUnauthorized(ctx, "invalid token type")
                return
            }
            // 将 claims 注入 context
            ctx.SetUserValue("claims", claims)
            next(ctx)
        }
    }
}

func writeUnauthorized(ctx *fasthttp.RequestCtx, msg string) {
    ctx.SetStatusCode(401)
    ctx.SetContentType("application/json")
    json.NewEncoder(ctx).Encode(map[string]interface{}{
        "code":    401,
        "message": msg,
    })
}
```

- [ ] **Step 3: 删除旧 auth.go，更新引用**

删除 `internal/auth/auth.go`，将所有引用旧 JWT 函数的地方改为使用新 `auth.GenerateToken` / `auth.ParseToken`。

- [ ] **Step 4: 编译验证**

Run: `go build ./...`
Expected: 可能因其他未重构文件报错，但 auth 包本身应编译通过

- [ ] **Step 5: Commit**

```bash
git add internal/auth/
git commit -m "feat: dual JWT auth system (admin + user tokens)"
```

---

## Task 4: 用户系统

**Files:**
- Create: `internal/user/handler.go`
- Create: `internal/user/service.go`
- Create: `internal/user/dto.go`

- [ ] **Step 1: 定义用户 DTO**

`internal/user/dto.go`:

```go
package user

type RegisterRequest struct {
    Email    string `json:"email"`
    Username string `json:"username"`
    Password string `json:"password"`
}

type LoginRequest struct {
    Email    string `json:"email"`
    Password string `json:"password"`
}

type LoginResponse struct {
    Token     string `json:"token"`
    ExpiresAt int64  `json:"expires_at"`
}

type RefreshRequest struct {
    Token string `json:"token"`
}

type UpdatePasswordRequest struct {
    OldPassword string `json:"old_password"`
    NewPassword string `json:"new_password"`
}

type UpdateEmailRequest struct {
    Password string `json:"password"`
    Email    string `json:"email"`
}

type CreateKeyRequest struct {
    Name string `json:"name"`
}

type ProfileResponse struct {
    ID        string `json:"id"`
    Email     string `json:"email"`
    Username  string `json:"username"`
    Status    string `json:"status"`
    Balance   int64  `json:"balance"`
    CreatedAt string `json:"created_at"`
}

type KeyResponse struct {
    ID        string `json:"id"`
    Name      string `json:"name"`
    Key       string `json:"key"`
    Enabled   bool   `json:"enabled"`
    CreatedAt string `json:"created_at"`
}

type SubscriptionResponse struct {
    PlanID    string `json:"plan_id"`
    PlanName  string `json:"plan_name"`
    PlanType  string `json:"plan_type"`
    Status    string `json:"status"`
}

type RedeemRequest struct {
    Code string `json:"code"`
}
```

- [ ] **Step 2: 实现用户 Service**

`internal/user/service.go` — 包含注册、登录、密码修改、API Key 管理、用量查询、套餐管理、充值码兑换的业务逻辑。关键函数签名：

```go
package user

import (
    "github.com/AutoCONFIG/cli-relay/internal/auth"
    "github.com/AutoCONFIG/cli-relay/internal/crypto"
    "github.com/AutoCONFIG/cli-relay/internal/db"
    "gorm.io/gorm"
)

type Service struct {
    db        *gorm.DB
    jwtSecret string
}

func NewService(database *gorm.DB, jwtSecret string) *Service {
    return &Service{db: database, jwtSecret: jwtSecret}
}

func (s *Service) Register(req *RegisterRequest) (*LoginResponse, error) { ... }
func (s *Service) Login(req *LoginRequest) (*LoginResponse, error) { ... }
func (s *Service) RefreshToken(tokenStr string) (*LoginResponse, error) { ... }
func (s *Service) GetProfile(userID string) (*ProfileResponse, error) { ... }
func (s *Service) UpdatePassword(userID string, req *UpdatePasswordRequest) error { ... }
func (s *Service) UpdateEmail(userID string, req *UpdateEmailRequest) error { ... }
func (s *Service) ListKeys(userID string) ([]KeyResponse, error) { ... }
func (s *Service) CreateKey(userID string, req *CreateKeyRequest) (*KeyResponse, error) { ... }
func (s *Service) DeleteKey(userID, keyID string) error { ... }
func (s *Service) GetUsage(userID string) (map[string]interface{}, error) { ... }
func (s *Service) GetUsageLogs(userID string, page, limit int) (map[string]interface{}, error) { ... }
func (s *Service) ListPlans() ([]map[string]interface{}, error) { ... }
func (s *Service) GetSubscription(userID string) (*SubscriptionResponse, error) { ... }
func (s *Service) Subscribe(userID, planID string) error { ... }
func (s *Service) RedeemCode(userID, code string) (int64, error) { ... }
```

注册逻辑：校验邮箱/用户名唯一 → bcrypt 哈希密码 → 创建 User → 生成 user JWT 返回。
Login 逻辑：查 User → 校验 bcrypt → 生成 user JWT 返回。
API Key 创建：生成 `sk-relay-` + UUID，创建 Token 记录关联 UserID。

- [ ] **Step 3: 实现用户 Handler**

`internal/user/handler.go` — HTTP handler，解析请求、调用 service、返回 JSON。每个方法对应一个路由。

```go
package user

import (
    "encoding/json"
    "github.com/valyala/fasthttp"
    "github.com/AutoCONFIG/cli-relay/internal/auth"
)

type Handler struct {
    service *Service
}

func NewHandler(service *Service) *Handler {
    return &Handler{service: service}
}

func (h *Handler) Register(ctx *fasthttp.RequestCtx) { ... }
func (h *Handler) Login(ctx *fasthttp.RequestCtx) { ... }
func (h *Handler) RefreshToken(ctx *fasthttp.RequestCtx) { ... }
func (h *Handler) GetProfile(ctx *fasthttp.RequestCtx) { ... }
func (h *Handler) UpdatePassword(ctx *fasthttp.RequestCtx) { ... }
func (h *Handler) UpdateEmail(ctx *fasthttp.RequestCtx) { ... }
func (h *Handler) ListKeys(ctx *fasthttp.RequestCtx) { ... }
func (h *Handler) CreateKey(ctx *fasthttp.RequestCtx) { ... }
func (h *Handler) DeleteKey(ctx *fasthttp.RequestCtx) { ... }
func (h *Handler) GetUsage(ctx *fasthttp.RequestCtx) { ... }
func (h *Handler) GetUsageLogs(ctx *fasthttp.RequestCtx) { ... }
func (h *Handler) ListPlans(ctx *fasthttp.RequestCtx) { ... }
func (h *Handler) GetSubscription(ctx *fasthttp.RequestCtx) { ... }
func (h *Handler) Subscribe(ctx *fasthttp.RequestCtx) { ... }
func (h *Handler) RedeemCode(ctx *fasthttp.RequestCtx) { ... }

// getUserID 从 context 中提取 user ID（由 JWT 中间件注入）
func getUserID(ctx *fasthttp.RequestCtx) string {
    claims, _ := ctx.UserValue("claims").(*auth.Claims)
    if claims != nil {
        return claims.UserID
    }
    return ""
}
```

- [ ] **Step 4: 编译验证**

Run: `go build ./internal/user/`
Expected: 编译通过

- [ ] **Step 5: Commit**

```bash
git add internal/user/
git commit -m "feat: user system — registration, login, API keys, usage, subscriptions"
```

---

## Task 5: Admin handler 拆分重构

**Files:**
- Create: `internal/admin/handler.go`
- Create: `internal/admin/channel_handler.go`
- Create: `internal/admin/account_handler.go`
- Create: `internal/admin/token_handler.go`
- Create: `internal/admin/plan_handler.go`
- Create: `internal/admin/user_handler.go`
- Create: `internal/admin/log_handler.go`
- Create: `internal/admin/dto.go`
- Keep: `internal/admin/audit.go`
- Keep: `internal/admin/scheduler.go`
- Delete: `internal/admin/admin.go`（替换）

- [ ] **Step 1: 定义 admin DTO**

`internal/admin/dto.go` — 所有 admin API 的请求/响应结构体，带 JSON tag。包含 ChannelDTO、AccountDTO、TokenDTO、PlanDTO、UserDTO 等。

- [ ] **Step 2: 实现 admin Handler**

`internal/admin/handler.go` — Handler 结构体持有 DB 引用，包含 Login、InitStatus、Setup、Dashboard 方法。在需要认证的方法中使用 `auth.RequireAdmin` 中间件。

- [ ] **Step 3: 按资源拆分 CRUD handler**

每个 `_handler.go` 文件实现对应资源的 List/Create/Update/Delete 方法。从旧 `admin.go` 的逻辑迁移，改用 DTO 而非 `map[string]interface{}`。

- [ ] **Step 4: 新增用户管理 handler**

`internal/admin/user_handler.go` — ListUsers（分页）、UpdateUser（启用/禁用、调整余额）、DeleteUser。

- [ ] **Step 5: 删除旧 admin.go**

确认所有功能迁移后，删除 `internal/admin/admin.go`。

- [ ] **Step 6: 编译验证**

Run: `go build ./internal/admin/`
Expected: 编译通过

- [ ] **Step 7: Commit**

```bash
git add internal/admin/
git commit -m "refactor: split admin handler into resource-specific files with DTOs"
```

---

## Task 6: 中间格式和 Provider 重构

**Files:**
- Create: `internal/relay/provider/types.go`
- Create: `internal/relay/provider/credentials.go`
- Create: `internal/relay/provider/convert.go`
- Create: `internal/relay/provider/openai/adaptor.go`
- Create: `internal/relay/provider/openai/responses.go`
- Create: `internal/relay/provider/openai/to_internal.go`
- Create: `internal/relay/provider/openai/from_internal.go`
- Create: `internal/relay/provider/openai/auth.go`
- Create: `internal/relay/provider/anthropic/adaptor.go`
- Create: `internal/relay/provider/anthropic/streaming.go`
- Create: `internal/relay/provider/anthropic/to_internal.go`
- Create: `internal/relay/provider/anthropic/from_internal.go`
- Create: `internal/relay/provider/anthropic/auth.go`
- Create: `internal/relay/provider/gemini/adaptor.go`
- Create: `internal/relay/provider/gemini/streaming.go`
- Create: `internal/relay/provider/gemini/to_internal.go`
- Create: `internal/relay/provider/gemini/from_internal.go`
- Create: `internal/relay/provider/gemini/auth.go`
- Delete: `internal/relay/types/adaptor.go`
- Delete: `internal/relay/types/credentials.go`
- Delete: `internal/relay/openai/`
- Delete: `internal/relay/anthropic/`
- Delete: `internal/relay/gemini/`

这是最大的重构任务。建议拆为子步骤执行。

- [ ] **Step 1: 定义中间格式**

`internal/relay/provider/types.go`:

```go
package provider

import "github.com/AutoCONFIG/cli-relay/internal/db"

// Format 表示 API 格式类型
type Format string

const (
    FormatOpenAIChat  Format = "openai_chat"
    FormatOpenAIResp  Format = "openai_responses"
    FormatAnthropic   Format = "anthropic"
    FormatGemini      Format = "gemini"
)

// InternalRequest 统一中间格式请求
type InternalRequest struct {
    Model       string
    Messages    []InternalMessage
    Tools       []InternalTool
    ToolChoice  *InternalToolChoice
    Stream      bool
    MaxTokens   *int
    Temperature *float64
    TopP        *float64
    StopWords   []string
    Metadata    map[string]interface{} // 格式特有字段透传
}

type InternalMessage struct {
    Role       string
    Content    []InternalContentPart
    ToolCalls  []InternalToolCall
    ToolResult *InternalToolResult
}

type InternalContentPart struct {
    Type     string // text, image_url
    Text     string
    ImageURL *string
}

type InternalToolCall struct {
    ID       string
    Name     string
    Arguments string
}

type InternalToolResult struct {
    ToolCallID string
    Content    string
    IsError    bool
}

type InternalTool struct {
    Type     string // function
    Name     string
    Description string
    Parameters  interface{}
}

type InternalToolChoice struct {
    Type     string // auto, none, required, function
    Function string // function name when type=function
}

// InternalResponse 统一中间格式响应
type InternalResponse struct {
    ID      string
    Model   string
    Choices []InternalChoice
    Usage   InternalUsage
}

type InternalChoice struct {
    Index        int
    Message      InternalMessage
    FinishReason string
}

type InternalUsage struct {
    PromptTokens     int
    CompletionTokens int
}

// Adaptor 上游适配器接口（扩展版）
type Adaptor interface {
    Init(channel *db.Channel, account *db.Account)
    GetRequestURL(path string) (string, error)
    SetupRequestHeader(req *fasthttp.Request, credentials string) error

    // 中间格式转换
    ToInternal(body []byte) (*InternalRequest, error)
    FromInternal(req *InternalRequest) ([]byte, error)

    // 流式转换
    ConvertStreamLine(line []byte) []byte

    // 用量解析
    ParseUsage(respBody []byte) (promptTokens, completionTokens int, err error)
    ParseStreamUsage(lastChunk []byte) (promptTokens, completionTokens int, err error)

    GetChannelType() string
}
```

- [ ] **Step 2: 迁移凭证提取**

将 `internal/relay/types/credentials.go` 的 `ExtractCredentialKey` 函数迁移到 `internal/relay/provider/credentials.go`，包名改为 `provider`。

- [ ] **Step 3: 实现转换调度器**

`internal/relay/provider/convert.go`:

```go
package provider

import "fmt"

// ToInternalConverters 注册表：格式 → ToInternal 转换器
var toInternalConverters = map[Format]func([]byte) (*InternalRequest, error){}

// FromInternalConverters 注册表：格式 → FromInternal 转换器
var fromInternalConverters = map[Format]func(*InternalRequest) ([]byte, error){}

func RegisterToInternal(format Format, converter func([]byte) (*InternalRequest, error)) {
    toInternalConverters[format] = converter
}

func RegisterFromInternal(format Format, converter func(*InternalRequest) ([]byte, error)) {
    fromInternalConverters[format] = converter
}

// ConvertRequest 将客户端格式请求转为上游格式请求
func ConvertRequest(clientFormat, upstreamFormat Format, body []byte) ([]byte, error) {
    if clientFormat == upstreamFormat {
        return body, nil // 透传
    }
    toInternal, ok := toInternalConverters[clientFormat]
    if !ok {
        return nil, fmt.Errorf("no ToInternal converter for format: %s", clientFormat)
    }
    internal, err := toInternal(body)
    if err != nil {
        return nil, fmt.Errorf("ToInternal conversion failed: %w", err)
    }
    fromInternal, ok := fromInternalConverters[upstreamFormat]
    if !ok {
        return nil, fmt.Errorf("no FromInternal converter for format: %s", upstreamFormat)
    }
    return fromInternal(internal)
}
```

- [ ] **Step 4: 迁移 OpenAI adaptor + 实现 ToInternal/FromInternal**

从 `internal/relay/openai/` 迁移到 `internal/relay/provider/openai/`，修复 fasthttp API 误用（`req.Header` 替代 `req.Request.Header`）。

实现 `to_internal.go`（OpenAI Chat → Internal）和 `from_internal.go`（Internal → OpenAI Chat）。在 `init()` 中注册转换器。

- [ ] **Step 5: 迁移 Anthropic adaptor + 实现 ToInternal/FromInternal**

从 `internal/relay/anthropic/` 迁移，修复 thinking_delta 处理（转发为 `reasoning_content` 字段）。

实现 `to_internal.go`（Anthropic → Internal）和 `from_internal.go`（Internal → Anthropic）。注册转换器。

- [ ] **Step 6: 迁移 Gemini adaptor + 实现 ToInternal/FromInternal**

从 `internal/relay/gemini/` 迁移，修复 tool call ID 稳定性。

实现 `to_internal.go`（Gemini → Internal）和 `from_internal.go`（Internal → Gemini）。注册转换器。

- [ ] **Step 7: 删除旧目录**

删除 `internal/relay/types/`、`internal/relay/openai/`、`internal/relay/anthropic/`、`internal/relay/gemini/`。

- [ ] **Step 8: 编译验证**

Run: `go build ./...`
Expected: 编译通过（handler.go 可能需要适配，见 Task 7）

- [ ] **Step 9: Commit**

```bash
git add internal/relay/provider/
git rm -r internal/relay/types/ internal/relay/openai/ internal/relay/anthropic/ internal/relay/gemini/
git commit -m "refactor: provider adaptor with intermediate format conversion"
```

---

## Task 7: Relay handler 适配

**Files:**
- Modify: `internal/relay/handler.go`
- Modify: `internal/relay/pool.go`
- Modify: `internal/relay/billing.go`
- Create: `internal/relay/account_refresh.go`

- [ ] **Step 1: handler.go 支持多格式入口**

修改 `HandleRelay`，根据请求路径判断客户端格式：

```go
func (r *Relayer) HandleRelay(ctx *fasthttp.RequestCtx) {
    path := string(ctx.Path())

    // 判断客户端格式
    var clientFormat provider.Format
    switch {
    case strings.HasPrefix(path, "/v1/chat/completions"):
        clientFormat = provider.FormatOpenAIChat
    case strings.HasPrefix(path, "/v1/responses"):
        clientFormat = provider.FormatOpenAIResp
    case strings.HasPrefix(path, "/v1/messages"):
        clientFormat = provider.FormatAnthropic
    case strings.HasPrefix(path, "/v1beta/"):
        clientFormat = provider.FormatGemini
    default:
        // 默认 OpenAI Chat（向后兼容 /v1/* 其他路径）
        clientFormat = provider.FormatOpenAIChat
    }

    // ... 后续逻辑中使用 clientFormat + channel type 确定转换路径
}
```

在请求转换阶段使用 `provider.ConvertRequest(clientFormat, upstreamFormat, body)` 替代当前的 `adaptor.ConvertRequest`。

- [ ] **Step 2: 实现 OAuth token 自动刷新**

`internal/relay/account_refresh.go`:

```go
package relay

import (
    "encoding/json"
    "fmt"
    "net/http"
    "net/url"
    "time"

    "github.com/AutoCONFIG/cli-relay/internal/crypto"
    "github.com/AutoCONFIG/cli-relay/internal/db"
    "gorm.io/gorm"
)

// EnsureValidCredentials 检查账号凭证是否有效，必要时刷新
func EnsureValidCredentials(account *db.Account, database *gorm.DB, encKey string) (string, error) {
    if account.CredType == "api_key" {
        return crypto.Decrypt(account.Credentials, encKey)
    }

    // OAuth token — 检查过期
    if account.TokenExpiry != nil && time.Now().After(*account.TokenExpiry) {
        return refreshOAuthToken(account, database, encKey)
    }

    return crypto.Decrypt(account.Credentials, encKey)
}

func refreshOAuthToken(account *db.Account, database *gorm.DB, encKey string) (string, error) {
    refreshToken, err := crypto.Decrypt(account.RefreshToken, encKey)
    if err != nil {
        return "", fmt.Errorf("decrypt refresh token: %w", err)
    }

    data := url.Values{
        "grant_type":    {"refresh_token"},
        "refresh_token": {refreshToken},
        "client_id":     {account.ClientID},
    }

    resp, err := http.PostForm(account.TokenURL, data)
    if err != nil {
        return "", fmt.Errorf("refresh request failed: %w", err)
    }
    defer resp.Body.Close()

    var result struct {
        AccessToken  string `json:"access_token"`
        RefreshToken string `json:"refresh_token"`
        ExpiresIn    int    `json:"expires_in"`
        Error        string `json:"error"`
    }
    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return "", fmt.Errorf("decode refresh response: %w", err)
    }
    if result.Error != "" {
        return "", fmt.Errorf("refresh failed: %s", result.Error)
    }

    // 异步更新数据库
    go func() {
        newCreds, _ := crypto.Encrypt(result.AccessToken, encKey)
        newRefresh, _ := crypto.Encrypt(result.RefreshToken, encKey)
        expiry := time.Now().Add(time.Duration(result.ExpiresIn) * time.Second)
        database.Model(&db.Account{}).Where("id = ?", account.ID).Updates(map[string]interface{}{
            "credentials":  newCreds,
            "refresh_token": newRefresh,
            "token_expiry":  expiry,
        })
    }()

    return result.AccessToken, nil
}
```

- [ ] **Step 3: pool.go 适配 OAuth 字段**

修改 `PoolManager` 的账户加载逻辑，确保 OAuth 字段（`CredType`, `RefreshToken`, `TokenExpiry`, `ClientID`, `TokenURL`）被正确加载。

- [ ] **Step 4: billing.go 关联用户**

在计费逻辑中，通过 Token 的 UserID 关联到 User，支持从用户余额扣费。

- [ ] **Step 5: 编译验证**

Run: `go build ./...`
Expected: 编译通过

- [ ] **Step 6: Commit**

```bash
git add internal/relay/
git commit -m "feat: multi-format relay entry points, OAuth auto-refresh, user-linked billing"
```

---

## Task 8: 入口和配置适配

**Files:**
- Modify: `cmd/relay/main.go`
- Modify: `internal/config/config.go`
- Delete: `internal/web/embed.go`
- Delete: `internal/web/index.html`

- [ ] **Step 1: 扩展配置**

`internal/config/config.go` 添加用户系统相关配置：

```go
type UserConfig struct {
    JWTExpiry       string `yaml:"jwt_expiry"`        // 默认 "24h"
    MaxKeysPerUser  int    `yaml:"max_keys_per_user"`  // 默认 5
}
```

在 `Config` 结构体中添加 `User UserConfig`。

- [ ] **Step 2: 重构 main.go**

适配新架构：初始化 UserService、UserHandler、AdminHandler，注册到 Server。

- [ ] **Step 3: 删除前端嵌入**

删除 `internal/web/embed.go` 和 `internal/web/index.html`，前端将作为独立项目。

- [ ] **Step 4: 编译验证**

Run: `go build ./...`
Expected: 编译通过

- [ ] **Step 5: 运行验证**

Run: `go run ./cmd/relay/`
Expected: 服务器启动，日志输出 "cli-relay ready"，不再提供前端页面

- [ ] **Step 6: Commit**

```bash
git add -A
git commit -m "refactor: wire up new architecture, remove embedded frontend"
```

---

## Task 9: OAuth provider 实现

**Files:**
- Create: `internal/relay/provider/openai/auth.go`
- Create: `internal/relay/provider/gemini/auth.go`

- [ ] **Step 1: 实现 OpenAI (Codex) OAuth**

`internal/relay/provider/openai/auth.go`:

```go
package openai

const (
    DefaultAuthURL    = "https://auth.openai.com/oauth/authorize"
    DefaultTokenURL   = "https://auth.openai.com/oauth/token"
    DefaultClientID   = "app_EMoamEEZ73f0CkXaXp7hrann"
    DefaultScope      = "openid profile email offline_access api.connectors.read api.connectors.invoke"
)

// BuildAuthURL 构建授权 URL（PKCE）
func BuildAuthURL(clientID, redirectURI, codeChallenge, state string) string { ... }

// ExchangeCode 用授权码换取 token
func ExchangeCode(tokenURL, code, redirectURI, codeVerifier, clientID string) (*TokenResponse, error) { ... }

// ExchangeForAPIKey 用 id_token 换取 API Key（Codex 特有流程）
func ExchangeForAPIKey(tokenURL, idToken, clientID string) (string, error) { ... }

// RefreshToken 刷新 access_token
func RefreshToken(tokenURL, refreshToken, clientID string) (*TokenResponse, error) { ... }

type TokenResponse struct {
    AccessToken  string `json:"access_token"`
    RefreshToken string `json:"refresh_token"`
    IDToken      string `json:"id_token"`
    TokenType    string `json:"token_type"`
    ExpiresIn    int    `json:"expires_in"`
}
```

- [ ] **Step 2: 实现 Gemini (Google) OAuth**

`internal/relay/provider/gemini/auth.go`:

```go
package gemini

const (
    DefaultAuthURL  = "https://accounts.google.com/o/oauth2/v2/auth"
    DefaultTokenURL = "https://oauth2.googleapis.com/token"
    DefaultClientID = "681255809395-oo8ft2oprdrnp9e3aqf6av3hmdib135j.apps.googleusercontent.com"
    DefaultScope    = "https://www.googleapis.com/auth/cloud-platform https://www.googleapis.com/auth/userinfo.email https://www.googleapis.com/auth/userinfo.profile"
)

// BuildAuthURL 构建授权 URL（PKCE）
func BuildAuthURL(clientID, redirectURI, codeChallenge, state string) string { ... }

// ExchangeCode 用授权码换取 token
func ExchangeCode(tokenURL, code, redirectURI, codeVerifier, clientID string) (*TokenResponse, error) { ... }

// RefreshToken 刷新 access_token
func RefreshToken(tokenURL, refreshToken, clientID string) (*TokenResponse, error) { ... }

type TokenResponse struct {
    AccessToken  string `json:"access_token"`
    RefreshToken string `json:"refresh_token"`
    TokenType    string `json:"token_type"`
    ExpiresIn    int    `json:"expires_in"`
    Scope        string `json:"scope"`
}
```

- [ ] **Step 3: 创建 Anthropic auth 预留文件**

`internal/relay/provider/anthropic/auth.go` — 当前 Anthropic 使用 API Key 认证，预留空文件供后期 OAuth 实现。

- [ ] **Step 4: 编译验证**

Run: `go build ./...`
Expected: 编译通过

- [ ] **Step 5: Commit**

```bash
git add internal/relay/provider/openai/auth.go internal/relay/provider/gemini/auth.go internal/relay/provider/anthropic/auth.go
git commit -m "feat: OpenAI (Codex) and Gemini OAuth PKCE implementation"
```

---

## Task 10: 端到端集成验证

**Files:**
- Modify: `Makefile`
- Modify: `Dockerfile`

- [ ] **Step 1: 更新 Makefile**

添加前端构建命令占位（后续前端项目就绪后填充）：

```makefile
.PHONY: build clean web

build:
	go build -o bin/relay ./cmd/relay/

clean:
	rm -rf bin/

web:
	@echo "Frontend build not yet configured. See web/ directory."
```

- [ ] **Step 2: 更新 Dockerfile**

移除前端嵌入相关步骤，后端单独构建。

- [ ] **Step 3: 启动服务并验证 API**

Run: `go run ./cmd/relay/`
验证以下端点：

```bash
# 管理员初始化
curl -X POST http://localhost:8080/api/admin/setup \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123"}'

# 管理员登录
curl -X POST http://localhost:8080/api/admin/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123"}'

# 用户注册
curl -X POST http://localhost:8080/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{"email":"test@example.com","username":"test","password":"test123"}'

# 用户登录
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"test@example.com","password":"test123"}'
```

Expected: 所有端点返回正确的 JSON 响应

- [ ] **Step 4: Commit**

```bash
git add Makefile Dockerfile
git commit -m "chore: update build config for v3 architecture"
```

---

## 依赖关系

```
Task 1 (数据模型) ← Task 2 (路由器) ← Task 3 (JWT)
                                            ↓
Task 6 (Provider/中间格式) ← Task 7 (Handler适配) ← Task 8 (入口适配)
                                            ↓
Task 4 (用户系统) ← Task 5 (Admin拆分)
                                            ↓
                               Task 9 (OAuth) ← Task 10 (集成验证)
```

Task 1-3 是基础层，Task 4-5 是业务层，Task 6-7 是核心引擎层，Task 8-10 是集成层。可以适当并行：Task 4 和 Task 5 可以并行，Task 6 的子步骤可以按 provider 并行。

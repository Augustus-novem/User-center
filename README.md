# user-center

一个基于 **Go + Gin + GORM + MySQL + Redis** 的用户中心项目，包含邮箱注册登录、JWT 登录态、短信验证码登录、微信 OAuth 登录、Redis 限流、统一配置管理和单元测试。

这个项目最初是一个“参数大量写死”的后端练习项目，后续逐步改造成了一个更像正式后端服务的版本：

- 配置统一收口到 `AppConfig`
- 使用 `viper + yaml + env override`
- 配置消费限制在 `main + ioc`
- 支持本地配置 watch
- `log.level` 支持热更新
- `feature` 开关支持**请求级动态控制**
- `go test ./...` 已可通过

---

## 1. 功能概览

当前实现的核心能力：

- 邮箱注册
- 邮箱密码登录
- JWT 登录态
- 登出
- 刷新 token
- 用户信息查询
- 短信验证码发送与验证码登录
- 微信 OAuth 登录
- Redis 验证码缓存与 Lua 校验
- Redis IP 限流中间件
- CORS 配置化
- Logger 配置化
- 配置文件热更新（部分配置）

---

## 2. 技术栈

### Web 层
- Gin
- 自定义统一 JSON 返回结构
- JWT 中间件
- Feature Guard 中间件

### 业务层
- Service / Repository / DAO 分层
- 用户服务
- 短信验证码服务
- 微信 OAuth 登录服务

### 存储层
- MySQL 8
- GORM
- Redis
- Lua 脚本实现验证码发送/校验限流

### 工程化
- Viper 配置管理
- Zap 日志
- Wire 依赖注入
- Docker Compose 本地依赖环境
- 单元测试 + mock

---

## 3. 项目结构

```text
user-center/
├── config/                     # 多环境配置文件
│   ├── dev.yaml
│   └── test.yaml
├── internal/
│   ├── config/                 # 配置结构、管理器、动态配置 holder
│   ├── domain/                 # 领域模型
│   ├── repository/             # repository / dao / cache
│   ├── service/                # 业务服务
│   └── web/                    # handler / jwt / middleware
├── ioc/                        # provider，负责消费配置并初始化依赖
├── script/mysql/               # MySQL 初始化脚本
├── docker-compose.yaml         # 本地 MySQL / Redis 环境
├── wire.go                     # Wire 注入入口
├── wire_gen.go                 # Wire 生成代码
└── main.go                     # 程序入口
```

---

## 4. 配置系统设计

这个项目的一个重点改造就是“统一配置体系”。

### 配置来源
配置按以下顺序合并：

1. 默认值（`setDefaults`）
2. 配置文件（`config/dev.yaml` / `config/test.yaml`）
3. 环境变量覆盖（敏感项）
4. 启动参数 `--config`

### 核心设计

- `internal/config/type.go`：定义统一的 `AppConfig`
- `internal/config/manager.go`：负责读配置、校验配置、watch 配置文件
- `main.go`：先初始化配置，再初始化其他依赖
- `ioc/*.go`：按模块消费配置初始化 DB / Redis / JWT / Logger / Gin / Wechat
- `service / repository / web`：**不直接依赖 viper**

### 当前支持热更新的配置
当前仅支持**部分动态配置热更新**：

- `log.level`
- `feature.*`

### 当前不会热更新、修改后需重启的配置
以下配置变更后，当前进程不会自动重建依赖：

- `server.*`
- `db.*`
- `redis.*`
- `jwt.*`
- `wechat.*`
- `cors.*`
- `ratelimit.*`

程序在 watch 到这些静态配置变化时，会输出 warning 提示“需重启后生效”。

### 关于 `feature` 开关
`feature` 目前实现的是**请求级动态控制**：

- 路由会正常注册
- 每个请求进入时，根据当前动态配置决定是否放行

这意味着：

- `feature.enable_wechat_login=false` 时，请求会被中间件拦截
- `feature.enable_wechat_login=true` 时，请求可以放行

---

## 5. 环境要求

建议本地准备：

- Go 1.25+
- Docker / Docker Compose
- MySQL 8
- Redis 7

如果你不想自己手动安装 MySQL/Redis，直接用项目自带的 `docker-compose.yaml` 即可。

---

## 6. 快速开始

### 6.1 启动 MySQL 和 Redis

在项目根目录执行：

```bash
docker compose up -d
```

默认会启动：

- MySQL：`localhost:13316`
- Redis：`localhost:6379`

### 6.2 数据库初始化

项目启动时会自动执行 GORM `AutoMigrate`，初始化表结构。

`docker-compose` 中已挂载 `script/mysql/user.sql`，会自动创建：

```sql
CREATE DATABASE user_center;
```

如果你要跑测试配置 `config/test.yaml`，建议再手动建一个测试库：

```sql
CREATE DATABASE user_center_test;
```

### 6.3 安装依赖

```bash
go mod tidy
```

### 6.4 启动服务

默认开发配置：

```bash
go run . --config=config/dev.yaml
```

启动后服务默认监听：

```text
http://localhost:8081
```

---

## 7. 配置文件说明

开发环境配置文件：

- `config/dev.yaml`

测试环境配置文件：

- `config/test.yaml`

### 关键配置项

#### 服务

```yaml
server:
  name: user-center
  port: 8081
  mode: debug
```

#### MySQL / Redis

```yaml
db:
  dsn: root:root@tcp(localhost:13316)/user_center?charset=utf8mb4&parseTime=True&loc=Local

redis:
  addr: localhost:6379
  password: ""
  db: 1
```

#### JWT

```yaml
jwt:
  access_token_key: xxx
  refresh_token_key: xxx
  access_token_ttl: 15m
  refresh_token_ttl: 168h
  idle_timeout: 168h
  absolute_timeout: 720h
```

#### Wechat OAuth

```yaml
wechat:
  app_id: xxx
  app_key: xxx
  redirect_url: http://localhost:8081/oauth2/wechat/callback
  state_cookie_name: jwt-state
  state_token_key: xxx
  state_token_ttl: 10m
  state_cookie_path: /oauth2/wechat/callback
```

#### CORS

```yaml
cors:
  allow_credentials: true
  allow_origins:
    - http://localhost:3000
```

#### 限流

```yaml
ratelimit:
  enabled: true
  prefix: ip-limiter
  interval: 1m
  limit: 100
```

#### 日志

```yaml
log:
  level: info
  encoding: console
```

#### 功能开关

```yaml
feature:
  enable_wechat_login: false
  enable_sms_login: true
  enable_debug_log: false
```

---

## 8. 环境变量覆盖

支持对敏感配置做环境变量覆盖：

```bash
DB_DSN
REDIS_ADDR
REDIS_PASSWORD
REDIS_DB
JWT_ACCESS_TOKEN_KEY
JWT_REFRESH_TOKEN_KEY
WECHAT_APP_ID
WECHAT_APP_KEY
WECHAT_STATE_TOKEN_KEY
```

例如：

```bash
export JWT_ACCESS_TOKEN_KEY=your-access-key
export JWT_REFRESH_TOKEN_KEY=your-refresh-key
go run . --config=config/dev.yaml
```

PowerShell 示例：

```powershell
$env:JWT_ACCESS_TOKEN_KEY="your-access-key"
$env:JWT_REFRESH_TOKEN_KEY="your-refresh-key"
go run . --config=config/dev.yaml
```

---

## 9. API 路由

### 用户相关

前缀：`/user`

- `POST /user/signup`：邮箱注册
- `POST /user/login`：邮箱密码登录
- `POST /user/logout`：退出登录
- `POST /user/refresh_token`：刷新 token
- `GET /user/profile`：获取当前用户信息
- `POST /user/login_sms`：短信验证码登录
- `POST /user/login_sms/code/send`：发送短信验证码

### 微信登录

前缀：`/oauth2/wechat`

- `GET /oauth2/wechat/authurl`：获取微信扫码登录 URL
- `ANY /oauth2/wechat/callback`：微信回调

---

## 10. 鉴权说明

项目使用 JWT 做登录态管理。

登录成功后，服务会在响应头中返回：

- `x-jwt-token`
- `x-refresh-token`

后续请求可通过 `Authorization: Bearer <token>` 携带访问令牌。

---

## 11. 短信验证码说明

当前默认短信服务是：

```go
ioc.InitSmsService() -> localsms.NewService()
```

也就是说，默认不会真的调用第三方短信平台，而是把验证码打印到控制台，便于本地开发调试。

如果后续要接腾讯云、Twilio 等实现，可以在 `ioc/sms.go` 中切换具体 provider。

---

## 12. 测试

项目当前支持直接运行单元测试：

```bash
go test ./...
```

已经通过的测试主要覆盖：

- 配置管理器
- ioc 层
- repository / dao / cache
- service
- web / jwt / wechat

### integration / e2e 测试
部分测试已通过 build tag 与普通单测隔离：

- `integration`
- `e2e`

单独运行示例：

```bash
go test -tags=integration ./internal/integration/...
go test -tags=e2e ./internal/repository/cache/...
```

---

## 13. 已知限制

### 1）Wechat 配置当前为启动必填
为了支持“请求级动态开启/关闭微信登录”，项目当前会始终初始化 Wechat Service，因此 `wechat.*` 相关配置需要在启动时就合法。

### 2）当前只对部分配置做热更新
目前真正会立刻生效的是：

- `log.level`
- `feature.*`

其余配置修改后需要重启。

### 3）短信服务默认是本地 mock
当前适合本地开发与功能演示，不适合直接用于真实生产发短信。

---

## 14. 后续可继续完善的方向

- 接入真实短信平台（腾讯云 / Twilio）
- 增加编辑资料、重置密码等用户能力
- 增加更多 integration / e2e 测试
- 配置中心扩展为远程配置（etcd / nacos 等）
- 进一步完善 feature 动态开关的业务闭环
- 增加 Dockerfile / 部署文档

---

## 15. 适合怎么讲这个项目

这个项目不只是“做了几个接口”，更适合作为一个**有工程化意识的后端实习项目**去讲。

可以重点讲这几条：

- 分层设计：`web -> service -> repository -> dao`
- JWT 登录态与 Redis session 管理
- Redis + Lua 实现验证码发送/校验限流
- 微信 OAuth 登录接入
- 统一配置体系改造：`viper + yaml + env override + watch`
- Wire + IoC 做依赖注入
- 单元测试与 mock

---

## 16. 启动命令速查

### 启动依赖

```bash
docker compose up -d
```

### 启动服务

```bash
go run . --config=config/dev.yaml
```

### 运行单元测试

```bash
go test ./...
```


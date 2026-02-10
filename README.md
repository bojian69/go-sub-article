# WeChat Subscription Service

微信公众号订阅微服务 - 用于获取公众号已发布的图文消息。

## 功能特性

- **双模式支持**
  - **简单模式** - 直接使用公众号 AppID/AppSecret 获取 access_token（推荐）
  - **第三方平台模式** - 适用于代运营多个公众号的 SaaS 平台
- **Token 自动管理** - 自动获取、缓存和刷新 access_token
- **多公众号支持** - 通过配置文件管理多个公众号
- **双协议 API** - 同时提供 HTTP REST API 和 gRPC 接口
- **高可用设计** - 使用 singleflight 防止并发刷新，支持重试机制
- **结构化日志** - 基于 slog 的 JSON 日志，支持 TraceID/RequestID，兼容 ELK/Loki
- **日志轮转** - 按天自动轮转，支持压缩和自动清理
- **Web 测试界面** - 内置前端页面，方便测试 API
- **Docker 部署** - 支持 Docker 和 docker-compose 一键部署

## 技术栈

- Go 1.25+
- Gin (HTTP 框架)
- gRPC
- Redis (Token 缓存)
- Uber FX (依赖注入)
- slog + lumberjack (结构化日志 + 按天轮转)
- gopter (属性测试)
- GitHub Actions (CI/CD)
- Docker Buildx (多架构镜像构建)

## 项目结构

```
.
├── api/proto/              # gRPC Proto 定义
├── cmd/server/             # 主程序入口
├── configs/                # 配置文件
├── docs/                   # API 文档
├── logs/                   # 日志文件（按天轮转）
├── internal/
│   ├── config/             # 配置加载
│   ├── fx/                 # FX 模块
│   ├── handler/
│   │   ├── grpc/           # gRPC Handler
│   │   └── http/           # HTTP Handler
│   ├── logger/             # 日志模块（slog + 文件轮转）
│   ├── repository/cache/   # Redis 缓存
│   ├── service/            # 业务服务
│   ├── version/            # 版本信息（ldflags 注入）
│   └── wechat/             # 微信 API 客户端
├── web/                    # 前端测试页面
├── .github/workflows/      # CI/CD 工作流
│   ├── ci.yaml             # CI: lint + security + test + build
│   └── cd.yaml             # CD: 多架构镜像构建与推送
├── Dockerfile
├── docker-compose.yml
└── Makefile
```

## 快速开始

### 1. 配置

编辑 `configs/config.local.yaml`：

#### 简单模式（推荐）

适用于获取自己公众号的图文内容：

```yaml
# 日志配置
log:
  level: "info"                    # debug, info, warn, error
  output: "both"                   # console, file, both
  service: "wechat-subscription-svc"
  file:
    path: "./logs"
    filename: "app.log"
    max_age: 30                    # 保留天数
    compress: true                 # 压缩旧日志

server:
  http_port: 8080
  grpc_port: 9090

redis:
  host: localhost
  port: 6379

wechat:
  simple_mode:
    enabled: true
    accounts:
      - app_id: "wx1234567890abcdef"    # 你的公众号 AppID
        app_secret: "your_appsecret"    # 你的公众号 AppSecret
```

**获取 AppID 和 AppSecret：**
1. 登录 [微信公众平台](https://mp.weixin.qq.com)
2. 设置 → 公众号设置 → 基本配置
3. 找到 AppID 和 AppSecret

> 注意：需要认证的服务号才能调用图文接口

#### 第三方平台模式（高级）

适用于代运营多个公众号的 SaaS 平台：

```yaml
wechat:
  simple_mode:
    enabled: false

  component:
    app_id: "your_component_appid"
    app_secret: "your_component_appsecret"
    verify_ticket: "your_component_verify_ticket"

  authorizers:
    - app_id: "authorizer_appid_1"
      refresh_token: "authorizer_refresh_token_1"
```

### 2. 本地运行

```bash
# 安装依赖
go mod download

# 启动 Redis
redis-server

# 运行服务
make run
```

### 3. Docker 部署

```bash
# 一键启动（包含 Redis）
make docker-up

# 查看日志
make docker-logs

# 停止服务
make docker-down
```

构建时可注入版本信息：

```bash
# 本地构建（注入版本信息）
docker build \
  --build-arg VERSION=v26.2.9 \
  --build-arg BUILD_TIME=$(date -u +'%Y-%m-%dT%H:%M:%SZ') \
  --build-arg GIT_COMMIT=$(git rev-parse HEAD) \
  -t wechat-subscription-svc:v26.2.9 .

# 多架构构建（需要 buildx）
docker buildx build \
  --platform linux/amd64,linux/arm64 \
  --build-arg VERSION=v26.2.9 \
  --build-arg BUILD_TIME=$(date -u +'%Y-%m-%dT%H:%M:%SZ') \
  --build-arg GIT_COMMIT=$(git rev-parse HEAD) \
  -t wechat-subscription-svc:v26.2.9 .
```

docker-compose 也支持版本注入：

```bash
# 使用环境变量传入版本信息
VERSION=v26.2.9 BUILD_TIME=$(date -u +'%Y-%m-%dT%H:%M:%SZ') GIT_COMMIT=$(git rev-parse HEAD) \
  docker-compose up -d
```

### 4. 访问服务

- **Web 测试界面**: http://localhost:8080
- **HTTP API**: http://localhost:8080/v1/
- **gRPC**: localhost:9090

## API 接口

### HTTP REST API

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/v1/accounts/{appid}/articles` | 获取图文列表 |
| GET | `/v1/accounts/{appid}/articles/{id}` | 获取图文详情 |

**示例请求：**

```bash
# 获取图文列表
curl "http://localhost:8080/v1/accounts/wx123456/articles?offset=0&count=10"

# 获取图文详情
curl "http://localhost:8080/v1/accounts/wx123456/articles/ARTICLE_ID"
```

**响应格式：**

```json
{
  "code": 0,
  "message": "success",
  "request_id": "uuid",
  "data": { ... }
}
```

### gRPC API

```protobuf
service SubscriptionService {
  rpc BatchGetPublishedArticles(BatchGetArticlesRequest) returns (BatchGetArticlesResponse);
  rpc GetPublishedArticle(GetArticleRequest) returns (GetArticleResponse);
}
```

详细文档请查看 [docs/api.md](docs/api.md)

## 开发命令

```bash
make build          # 编译
make run            # 运行
make test           # 测试
make lint           # 代码检查
make proto          # 生成 Proto 代码
make docker         # 构建 Docker 镜像
make docker-up      # 启动 Docker Compose
make docker-down    # 停止 Docker Compose
```

## 自动发布

项目提供自动发布脚本，支持检测代码更新、构建镜像、运行容器。

### auto-release.sh

自动检测远程更新，构建 Docker 镜像并运行：

```bash
# 添加执行权限
chmod +x auto-release.sh

# 仅本地构建和运行
./auto-release.sh

# 构建并推送到 Docker Hub
./auto-release.sh myusername

# 强制执行（忽略 commit 对比）
./auto-release.sh -f

# 强制构建并推送
./auto-release.sh -f myusername
```

**版本标签规则：** `v年.月.日`，如 `v26.2.6`，同一天多次发布自动递增：`v26.2.6.1`

### setup-cron.sh

设置定时任务，每 5 分钟自动检查更新：

```bash
# 添加执行权限
chmod +x setup-cron.sh

# 设置定时任务（仅本地构建）
./setup-cron.sh

# 设置定时任务（构建并推送）
./setup-cron.sh myusername

# 查看定时任务
crontab -l

# 查看执行日志
tail -f auto-release.log
```

## CI/CD

项目使用 GitHub Actions 实现自动化 CI/CD。

### CI 流水线

Push 到 main 或 PR 到 main 时自动触发：

```
lint (gofmt + go vet + golangci-lint)
  ├── security (govulncheck 漏洞扫描)
  └── test (单元测试 + 覆盖率检查)
        └── build (Go 编译 + Docker 构建验证)
```

### CD 流水线

推送 `v*` 格式的 tag 时自动触发：

```bash
# 推送 tag 触发自动构建和推送
git tag v26.2.9
git push origin v26.2.9
```

CD 会自动：
1. 构建 `linux/amd64` + `linux/arm64` 多架构 Docker 镜像
2. 注入版本信息（VERSION、BUILD_TIME、GIT_COMMIT）
3. 推送到 Docker Registry，打上版本标签和 `latest` 标签

### GitHub Secrets 配置

在 repo Settings → Secrets and variables → Actions 中配置：

| Secret | 说明 | 示例 |
|--------|------|------|
| `DOCKER_REGISTRY` | Registry 地址 | `docker.io` |
| `DOCKER_NAMESPACE` | 命名空间/用户名 | `myusername` |
| `DOCKER_USERNAME` | 登录用户名 | `myusername` |
| `DOCKER_PASSWORD` | 登录密码/Token | `dckr_pat_xxx` |

## 架构设计

```
┌─────────────┐     ┌─────────────┐
│ HTTP Client │     │ gRPC Client │
└──────┬──────┘     └──────┬──────┘
       │                   │
       ▼                   ▼
┌──────────────────────────────────┐
│           Handler Layer          │
│   (HTTP Handler / gRPC Handler)  │
└──────────────┬───────────────────┘
               │
               ▼
┌──────────────────────────────────┐
│          Service Layer           │
│  (TokenService / ArticleService) │
└──────────────┬───────────────────┘
               │
       ┌───────┴───────┐
       ▼               ▼
┌─────────────┐  ┌─────────────┐
│    Redis    │  │ WeChat API  │
│   (Cache)   │  │  (Client)   │
└─────────────┘  └─────────────┘
```

## Token 管理流程

1. 请求到达时，先检查 Redis 缓存
2. 缓存命中且 TTL > 10min，直接返回
3. 缓存未命中或即将过期，调用微信 API 刷新
4. 使用 singleflight 防止并发刷新
5. 新 Token 缓存到 Redis，TTL = expires_in - 5min

## 错误码

| 错误码 | 说明 |
|--------|------|
| 0 | 成功 |
| 400001 | 参数错误 |
| 404001 | 资源不存在 |
| 500001 | 微信 API 错误 |
| 500002 | Redis 错误 |
| 500003 | 内部错误 |

## License

MIT

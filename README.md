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
- **Web 测试界面** - 内置前端页面，方便测试 API
- **Docker 部署** - 支持 Docker 和 docker-compose 一键部署

## 技术栈

- Go 1.22+
- Gin (HTTP 框架)
- gRPC
- Redis (Token 缓存)
- Uber FX (依赖注入)
- gopter (属性测试)

## 项目结构

```
.
├── api/proto/              # gRPC Proto 定义
├── cmd/server/             # 主程序入口
├── configs/                # 配置文件
├── docs/                   # API 文档
├── internal/
│   ├── config/             # 配置加载
│   ├── fx/                 # FX 模块
│   ├── handler/
│   │   ├── grpc/           # gRPC Handler
│   │   └── http/           # HTTP Handler
│   ├── repository/cache/   # Redis 缓存
│   ├── service/            # 业务服务
│   └── wechat/             # 微信 API 客户端
├── web/                    # 前端测试页面
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

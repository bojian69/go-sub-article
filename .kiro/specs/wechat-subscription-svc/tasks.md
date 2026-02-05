# Implementation Plan: WeChat Subscription Service

## Overview

本实现计划将微信公众号订阅微服务的设计分解为可执行的编码任务。采用自底向上的方式，先实现基础设施层，再实现业务层，最后实现 API 层。

## Tasks

- [x] 1. 项目初始化和基础设施
  - [x] 1.1 初始化 Go 模块和项目结构
    - 创建 go.mod (git.uhomes.net/uhs-go/wechat-subscription-svc)
    - 创建目录结构: cmd/, configs/, internal/, api/, pkg/
    - 创建 Makefile 和 .golangci.yml
    - _Requirements: 6.1_

  - [x] 1.2 创建配置结构和加载器
    - 实现 internal/config/config.go 定义配置结构体
    - 使用 Viper 加载 YAML 配置
    - 实现配置验证逻辑
    - 创建 configs/config.local.yaml 示例配置
    - _Requirements: 6.1, 6.2, 6.3, 6.4, 6.5, 6.6_

  - [x] 1.3 编写配置加载单元测试
    - 测试有效配置解析
    - 测试缺失必填字段检测
    - 测试多公众号配置加载
    - _Requirements: 6.6_

- [x] 2. 缓存层实现
  - [x] 2.1 实现 Redis 缓存仓库
    - 创建 internal/repository/cache/redis.go
    - 实现 CacheRepository 接口
    - 实现 GetComponentToken/SetComponentToken
    - 实现 GetAuthorizerToken/SetAuthorizerToken
    - 使用 go-redis v9+ 单机模式
    - _Requirements: 7.1, 7.3, 7.4_

  - [x] 2.2 编写缓存 Key 格式属性测试
    - **Property 2: Cache Key Format Includes Identifier**
    - 生成随机 appid 验证 key 包含该 appid
    - **Validates: Requirements 1.6, 7.3, 7.4**

  - [x] 2.3 编写 TTL 计算属性测试
    - **Property 15: TTL Calculation**
    - 生成随机 expires_in 验证 TTL = expires_in - 5min
    - **Validates: Requirements 7.2**

- [x] 3. 微信 API 客户端实现
  - [x] 3.1 定义微信 API 数据模型
    - 创建 internal/wechat/models.go
    - 定义 ComponentTokenRequest/Response
    - 定义 RefreshAuthorizerTokenRequest/Response
    - 定义 WeChatBatchGetRequest/Response
    - 定义 WeChatGetArticleResponse
    - 定义 WeChatErrorResponse
    - _Requirements: 2.8, 3.4_

  - [x] 3.2 实现微信 API 客户端
    - 创建 internal/wechat/client/client.go
    - 实现 WeChatClient 接口
    - 实现 GetComponentAccessToken 方法
    - 实现 RefreshAuthorizerToken 方法
    - 实现 BatchGetPublishedArticles 方法
    - 实现 GetPublishedArticle 方法
    - 实现重试逻辑（3 次，指数退避）
    - _Requirements: 2.7, 2.9, 3.3, 3.5_

  - [x] 3.3 编写重试行为属性测试
    - **Property 10: Retry Behavior**
    - 模拟失败验证重试次数
    - **Validates: Requirements 2.9, 3.5**

  - [x] 3.4 编写响应透传属性测试
    - **Property 9: Response Transparency**
    - 生成微信响应验证字段保留
    - **Validates: Requirements 2.8, 3.4**

- [x] 4. Checkpoint - 基础设施层验证
  - Ensure all tests pass, ask the user if questions arise.

- [x] 5. Token 服务实现
  - [x] 5.1 实现 Token 服务
    - 创建 internal/service/token_service.go
    - 实现 TokenService 接口
    - 实现 GetComponentToken（缓存优先）
    - 实现 GetAuthorizerToken（缓存优先）
    - 使用 singleflight 防止并发刷新
    - 实现主动刷新逻辑（TTL < 10min）
    - _Requirements: 1.2, 1.3, 1.4, 1.5, 1.7, 1.8, 1.9_

  - [x] 5.2 编写 Token 缓存优先属性测试
    - **Property 1: Token Cache-First Pattern**
    - 验证缓存命中时不调用 API
    - **Validates: Requirements 1.2, 1.3, 1.4, 1.5**

  - [x] 5.3 编写 Singleflight 并发控制属性测试
    - **Property 4: Singleflight Concurrency Control**
    - 并发请求验证只有一次 API 调用
    - **Validates: Requirements 1.8**

- [x] 6. 图文服务实现
  - [x] 6.1 实现图文服务
    - 创建 internal/service/article_service.go
    - 实现 ArticleService 接口
    - 实现 BatchGetPublishedArticles 方法
    - 实现 GetPublishedArticle 方法
    - 集成 TokenService 获取 Token
    - _Requirements: 2.3, 2.6, 2.8, 3.4_

  - [x] 6.2 编写 no_content 参数属性测试
    - **Property 7: No Content Parameter Behavior**
    - 验证 no_content=1 时不返回 content 字段
    - **Validates: Requirements 2.6**

- [x] 7. Checkpoint - 服务层验证
  - Ensure all tests pass, ask the user if questions arise.

- [x] 8. gRPC API 实现
  - [x] 8.1 定义 Proto 文件
    - 创建 api/proto/subscription.proto
    - 定义 SubscriptionService
    - 定义 BatchGetPublishedArticles RPC
    - 定义 GetPublishedArticle RPC
    - 定义请求/响应消息类型
    - _Requirements: 5.1, 5.2, 5.3_

  - [x] 8.2 生成 gRPC 代码
    - 运行 protoc 生成 Go 代码
    - 更新 Makefile 添加 proto 生成命令
    - _Requirements: 5.1_

  - [x] 8.3 实现 gRPC Handler
    - 创建 internal/handler/grpc/handler.go
    - 实现 BatchGetPublishedArticles
    - 实现 GetPublishedArticle
    - 实现请求验证
    - 实现 gRPC 状态码映射
    - 在响应 metadata 中添加 request_id
    - _Requirements: 5.2, 5.3, 5.4, 5.5_

  - [x] 8.4 编写 gRPC 状态码映射属性测试
    - **Property 13: gRPC Status Code Mapping**
    - 验证错误条件返回正确的 gRPC 状态码
    - **Validates: Requirements 5.4**

- [x] 9. HTTP API 实现
  - [x] 9.1 实现 HTTP Handler
    - 创建 internal/handler/http/handler.go
    - 实现 GET /v1/accounts/:authorizer_appid/articles
    - 实现 GET /v1/accounts/:authorizer_appid/articles/:article_id
    - 实现请求验证
    - 实现标准响应结构
    - 生成唯一 request_id
    - _Requirements: 2.1, 4.1, 4.2, 4.3, 4.4, 4.5_

  - [x] 9.2 编写请求验证属性测试
    - **Property 6: Request Parameter Validation**
    - 生成无效请求验证拒绝
    - **Validates: Requirements 2.1, 2.2, 2.4, 3.1, 3.2**

  - [x] 9.3 编写 HTTP 响应结构属性测试
    - **Property 11: HTTP Response Structure**
    - 验证响应包含 code, message, request_id
    - **Validates: Requirements 4.3, 4.4**

  - [x] 9.4 编写 Request ID 唯一性属性测试
    - **Property 12: Request ID Uniqueness**
    - 生成多个请求验证 ID 唯一
    - **Validates: Requirements 4.5, 5.5**

- [x] 10. Checkpoint - API 层验证
  - Ensure all tests pass, ask the user if questions arise.

- [x] 11. 应用组装和启动
  - [x] 11.1 实现 Uber FX 模块
    - 创建 internal/fx/modules.go
    - 定义 ConfigModule
    - 定义 CacheModule
    - 定义 WeChatModule
    - 定义 ServiceModule
    - 定义 HandlerModule
    - _Requirements: 1.1, 1.10_

  - [x] 11.2 实现主程序入口
    - 创建 cmd/server/main.go
    - 使用 Uber FX 组装应用
    - 启动 HTTP 和 gRPC 服务器
    - 实现优雅关闭
    - _Requirements: 1.1_

  - [x] 11.3 编写配置验证失败属性测试
    - **Property 14: Configuration Validation**
    - 验证无效配置导致启动失败
    - **Validates: Requirements 6.6**

- [x] 12. Final Checkpoint - 完整功能验证
  - Ensure all tests pass, ask the user if questions arise.

## Notes

- All tasks are required for comprehensive test coverage
- Each task references specific requirements for traceability
- Checkpoints ensure incremental validation
- Property tests validate universal correctness properties using gopter library
- Unit tests validate specific examples and edge cases
- 使用 `go test -v ./...` 运行所有测试
- 使用 `make lint` 运行代码检查

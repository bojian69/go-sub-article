# Requirements Document

## Introduction

本文档定义了微信公众号订阅微服务的功能需求。该服务负责管理微信第三方平台的 Token、获取公众号已发布的图文消息列表和详情，并通过 gRPC 和 HTTP REST API 对外提供服务。

## Glossary

- **Subscription_Service**: 微信公众号订阅微服务，负责 Token 管理和图文消息获取
- **Token_Manager**: Token 管理组件，负责获取、缓存和刷新各类 access_token
- **WeChat_Client**: 微信 API 客户端，负责调用微信开放平台接口
- **Cache_Repository**: 缓存仓库，使用 Redis 存储 Token 等数据
- **Config_Provider**: 配置提供者，从配置文件读取第三方平台和公众号授权信息
- **HTTP_Handler**: HTTP 处理器，提供 RESTful API 接口
- **GRPC_Handler**: gRPC 处理器，提供 gRPC 接口
- **component_access_token**: 第三方平台接口调用凭证，通过 component_appid、component_appsecret 和 component_verify_ticket 获取
- **authorizer_access_token**: 授权方（公众号）接口调用凭证，有效期约 2 小时
- **authorizer_refresh_token**: 用于刷新 authorizer_access_token 的令牌，长期有效

## Requirements

### Requirement 1: Token 管理

**User Story:** As a 服务调用方, I want 系统自动管理微信 Token, so that 我可以直接调用业务接口而无需关心 Token 的获取和刷新。

#### Acceptance Criteria

1. WHEN the Subscription_Service starts, THE Token_Manager SHALL load component_appid, component_appsecret, and component_verify_ticket from configuration
2. WHEN the Token_Manager needs component_access_token, THE Token_Manager SHALL first check Redis cache for a valid cached token
3. IF the cached component_access_token is not found or expired, THEN THE Token_Manager SHALL call WeChat API to obtain a new component_access_token and cache it in Redis
4. WHEN the Token_Manager needs authorizer_access_token for a specific authorizer_appid, THE Token_Manager SHALL first check Redis cache using a key that includes the authorizer_appid
5. IF the cached authorizer_access_token is not found or expired, THEN THE Token_Manager SHALL use authorizer_refresh_token to obtain a new authorizer_access_token
6. WHEN caching authorizer_access_token, THE Token_Manager SHALL use a Redis key format that includes authorizer_appid to distinguish between different official accounts
7. WHEN the authorizer_access_token is about to expire within 10 minutes, THE Token_Manager SHALL proactively refresh the token
8. WHEN multiple concurrent requests need to refresh the same token, THE Token_Manager SHALL use singleflight pattern to prevent duplicate refresh operations
9. IF token refresh fails, THEN THE Token_Manager SHALL log an error with alert level and return an appropriate error to the caller
10. WHEN the Subscription_Service starts, THE Config_Provider SHALL load multiple official account configurations including authorizer_appid and authorizer_refresh_token from YAML configuration file

### Requirement 2: 获取已发布消息列表

**User Story:** As a 服务调用方, I want 获取公众号已发布的图文消息列表, so that 我可以展示公众号的历史文章。

#### Acceptance Criteria

1. WHEN a client requests the published article list via HTTP GET /v1/accounts/{authorizer_appid}/articles, THE HTTP_Handler SHALL validate the request parameters
2. WHEN a client requests the published article list via gRPC BatchGetPublishedArticles, THE GRPC_Handler SHALL validate the request parameters
3. WHEN the request contains offset parameter, THE Subscription_Service SHALL pass it to WeChat API freepublish_batchget
4. WHEN the request contains count parameter, THE Subscription_Service SHALL validate that count is between 1 and 20 inclusive
5. IF the count parameter is outside the valid range, THEN THE Subscription_Service SHALL return a validation error with code 400
6. WHEN the request contains no_content parameter set to 1, THE Subscription_Service SHALL exclude content field from the response
7. WHEN calling WeChat API freepublish_batchget, THE WeChat_Client SHALL include the authorizer_access_token as query parameter
8. WHEN WeChat API returns successfully, THE Subscription_Service SHALL return the response structure transparently including total_count, item_count, and item array
9. IF WeChat API call fails, THEN THE WeChat_Client SHALL retry up to 3 times with exponential backoff
10. IF all retries fail, THEN THE Subscription_Service SHALL return an error response with appropriate error code and message

### Requirement 3: 获取已发布图文详情

**User Story:** As a 服务调用方, I want 获取指定图文的详细信息, so that 我可以展示文章的完整内容。

#### Acceptance Criteria

1. WHEN a client requests article details via HTTP GET /v1/accounts/{authorizer_appid}/articles/{article_id}, THE HTTP_Handler SHALL validate the request parameters
2. WHEN a client requests article details via gRPC GetPublishedArticle, THE GRPC_Handler SHALL validate the request parameters
3. WHEN calling WeChat API freepublishGetarticle, THE WeChat_Client SHALL include the authorizer_access_token as query parameter and article_id in request body
4. WHEN WeChat API returns successfully, THE Subscription_Service SHALL return the news_item array transparently including all fields (title, author, digest, content, content_source_url, thumb_media_id, thumb_url, need_open_comment, only_fans_can_comment, url, is_deleted)
5. IF WeChat API call fails, THEN THE WeChat_Client SHALL retry up to 3 times with exponential backoff
6. IF all retries fail, THEN THE Subscription_Service SHALL return an error response with appropriate error code and message

### Requirement 4: HTTP REST API 规范

**User Story:** As a 外部服务调用方, I want 通过标准的 RESTful API 访问服务, so that 我可以方便地集成到现有系统。

#### Acceptance Criteria

1. THE HTTP_Handler SHALL expose all endpoints under /v1 path prefix for version management
2. THE HTTP_Handler SHALL use lowercase plural nouns for resource naming (accounts, articles)
3. WHEN returning a successful response, THE HTTP_Handler SHALL include code, message, request_id, data, and metadata fields in the response body
4. WHEN returning an error response, THE HTTP_Handler SHALL include code, message, and request_id fields following uhomes standard error code system
5. THE HTTP_Handler SHALL generate a unique request_id for each request and include it in the response
6. WHEN receiving a request, THE HTTP_Handler SHALL log the request with request_id for traceability

### Requirement 5: gRPC API 规范

**User Story:** As a 内部服务调用方, I want 通过 gRPC 接口访问服务, so that 我可以获得更高效的服务间通信。

#### Acceptance Criteria

1. THE GRPC_Handler SHALL define service and message types in proto files under api/proto directory
2. THE GRPC_Handler SHALL implement BatchGetPublishedArticles RPC for fetching article list
3. THE GRPC_Handler SHALL implement GetPublishedArticle RPC for fetching article details
4. WHEN returning a response, THE GRPC_Handler SHALL use standard gRPC status codes for error handling
5. THE GRPC_Handler SHALL include request_id in response metadata for traceability

### Requirement 6: 配置管理

**User Story:** As a 运维人员, I want 通过配置文件管理服务配置, so that 我可以灵活地部署和管理服务。

#### Acceptance Criteria

1. THE Config_Provider SHALL load configuration from YAML file at configs/config.local.yaml for local development
2. THE Config_Provider SHALL support configuration for component_appid, component_appsecret, and component_verify_ticket
3. THE Config_Provider SHALL support configuration for multiple official accounts with authorizer_appid and authorizer_refresh_token
4. THE Config_Provider SHALL support Redis connection configuration including host, port, password, and database
5. THE Config_Provider SHALL support HTTP server port and gRPC server port configuration
6. WHEN configuration validation fails, THE Subscription_Service SHALL fail to start with a clear error message

### Requirement 7: 缓存管理

**User Story:** As a 系统管理员, I want Token 被正确缓存和管理, so that 系统可以高效运行并避免频繁调用微信 API。

#### Acceptance Criteria

1. THE Cache_Repository SHALL connect to Redis in standalone mode using go-redis v9+
2. WHEN caching component_access_token, THE Cache_Repository SHALL set TTL based on expires_in returned by WeChat API minus a safety margin of 5 minutes
3. WHEN caching authorizer_access_token, THE Cache_Repository SHALL use key format "wechat:token:authorizer:{authorizer_appid}" to distinguish different official accounts
4. WHEN caching component_access_token, THE Cache_Repository SHALL use key format "wechat:token:component:{component_appid}"
5. IF Redis connection fails, THEN THE Cache_Repository SHALL return an error and the service SHALL fall back to fetching token from WeChat API

### Requirement 8: 错误处理与日志

**User Story:** As a 开发人员, I want 系统有完善的错误处理和日志记录, so that 我可以快速定位和解决问题。

#### Acceptance Criteria

1. THE Subscription_Service SHALL use slog with zap backend for structured logging
2. WHEN an error occurs, THE Subscription_Service SHALL log the error with context including request_id, authorizer_appid, and error details
3. IF token refresh fails after all retries, THEN THE Token_Manager SHALL log an alert-level message for monitoring system to pick up
4. WHEN calling WeChat API, THE WeChat_Client SHALL log request and response details at debug level
5. WHEN WeChat API returns an error code, THE WeChat_Client SHALL log the error code and message at error level

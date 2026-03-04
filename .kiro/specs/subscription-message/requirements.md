# 需求文档：微信小程序订阅消息功能

## 简介

本功能为微信公众号小程序添加订阅消息能力，允许用户在小程序中主动订阅消息，公众号可以通过订阅消息向用户推送详细信息。订阅消息是微信小程序提供的一种用户主动订阅的消息推送机制，相比模板消息更加规范和用户友好。

## 术语表

- **Subscription_Message_Service**: 订阅消息服务，负责管理订阅消息的发送和模板管理
- **WeChat_Callback_Handler**: 微信回调处理器，负责处理微信服务器的验证请求和消息推送
- **Server_Verification**: 服务器验证，微信用于验证服务器URL有效性的机制
- **Callback_Token**: 回调令牌，用于验证微信服务器请求的安全性
- **Signature**: 签名，使用Token、timestamp、nonce通过SHA1算法生成的验证字符串
- **Mini_Program**: 微信小程序客户端
- **Template**: 订阅消息模板，定义消息的格式和内容结构
- **Template_ID**: 订阅消息模板的唯一标识符
- **OpenID**: 微信用户的唯一标识符
- **Access_Token**: 微信API访问令牌
- **WeChat_API_Client**: 微信API客户端，封装与微信服务器的通信
- **Subscription_Request**: 用户订阅请求，包含用户授权信息
- **Message_Payload**: 消息内容载荷，包含要发送的具体数据
- **Template_Data**: 模板数据，填充到消息模板中的键值对
- **Send_Result**: 消息发送结果，包含成功或失败状态
- **Error_Code**: 微信API返回的错误代码
- **Rate_Limit**: 微信API的频率限制

## 需求

### 需求 0: 微信服务器验证（前置需求）

**用户故事:** 作为系统管理员，我希望能够完成微信公众号的服务器配置验证，以便微信服务器能够将消息和事件推送到我们的服务器。

#### 验收标准

1. THE System SHALL 提供一个 HTTP GET 接口用于接收微信服务器的验证请求
2. WHEN 接收到验证请求，THE System SHALL 验证请求参数包含 signature、timestamp、nonce 和 echostr
3. THE System SHALL 使用配置的 Token 对 timestamp、nonce 进行 SHA1 签名验证
4. IF 签名验证成功，THEN THE System SHALL 原样返回 echostr 参数
5. IF 签名验证失败，THEN THE System SHALL 返回错误响应
6. THE System SHALL 记录每次验证请求的详细日志，包括验证结果
7. THE System SHALL 支持配置多个公众号的 Token（通过 authorizer_app_id 区分）
8. THE System SHALL 在 1 秒内完成验证响应

### 需求 1: 发送订阅消息

**用户故事:** 作为公众号运营者，我希望能够向已订阅的用户发送订阅消息，以便及时推送重要信息。

#### 验收标准

1. WHEN 接收到有效的发送请求，THE Subscription_Message_Service SHALL 调用微信API发送订阅消息
2. THE Subscription_Message_Service SHALL 验证 Template_ID 的有效性
3. THE Subscription_Message_Service SHALL 验证 OpenID 的有效性
4. THE Subscription_Message_Service SHALL 验证 Template_Data 符合模板定义的字段要求
5. WHEN 发送成功，THE Subscription_Message_Service SHALL 返回包含消息ID的 Send_Result
6. IF 发送失败，THEN THE Subscription_Message_Service SHALL 返回包含 Error_Code 和错误描述的 Send_Result
7. WHEN Access_Token 过期（Error_Code 为 40001 或 42001），THE Subscription_Message_Service SHALL 刷新令牌并重试一次
8. THE Subscription_Message_Service SHALL 在 5 秒内完成单次发送操作（不包括令牌刷新）
9. THE Subscription_Message_Service SHALL 记录每次发送操作的详细日志，包括请求参数、响应结果和耗时

### 需求 2: 订阅消息模板管理

**用户故事:** 作为开发者，我希望能够查询和管理订阅消息模板，以便了解可用的消息模板。

#### 验收标准

1. THE Subscription_Message_Service SHALL 提供获取模板列表的接口
2. WHEN 请求模板列表，THE Subscription_Message_Service SHALL 调用微信API获取已添加的模板
3. THE Subscription_Message_Service SHALL 返回包含 Template_ID、模板标题和内容示例的模板列表
4. WHEN 模板列表为空，THE Subscription_Message_Service SHALL 返回空列表而非错误
5. THE Subscription_Message_Service SHALL 缓存模板列表 30 分钟以减少API调用
6. WHEN 缓存过期，THE Subscription_Message_Service SHALL 自动刷新模板列表

### 需求 3: 错误处理和重试机制

**用户故事:** 作为系统管理员，我希望系统能够妥善处理微信API的各种错误情况，以确保服务的稳定性。

#### 验收标准

1. IF Access_Token 无效或过期，THEN THE Subscription_Message_Service SHALL 自动刷新令牌并重试
2. IF 微信API返回频率限制错误（Error_Code 45009），THEN THE Subscription_Message_Service SHALL 返回明确的频率限制错误
3. IF 用户拒绝订阅或订阅已过期（Error_Code 43101），THEN THE Subscription_Message_Service SHALL 返回订阅状态错误
4. IF 模板ID不存在（Error_Code 40037），THEN THE Subscription_Message_Service SHALL 返回模板不存在错误
5. IF 网络超时，THEN THE Subscription_Message_Service SHALL 在 30 秒后返回超时错误
6. THE Subscription_Message_Service SHALL 仅对令牌过期错误进行自动重试，其他错误直接返回
7. THE Subscription_Message_Service SHALL 记录所有错误的详细信息，包括 Error_Code、错误消息和请求上下文

### 需求 4: API接口设计

**用户故事:** 作为API调用方，我希望有清晰的接口定义，以便正确调用订阅消息功能。

#### 验收标准

1. THE Subscription_Message_Service SHALL 提供 SendSubscriptionMessage 接口
2. THE SendSubscriptionMessage 接口 SHALL 接受 authorizer_app_id、openid、template_id、data 和可选的 page 参数
3. THE SendSubscriptionMessage 接口 SHALL 验证所有必需参数不为空
4. THE SendSubscriptionMessage 接口 SHALL 验证 data 字段包含的键值对数量不超过 20 个
5. THE SendSubscriptionMessage 接口 SHALL 验证每个 data 值的长度不超过 20 个字符
6. THE Subscription_Message_Service SHALL 提供 GetTemplateList 接口
7. THE GetTemplateList 接口 SHALL 接受 authorizer_app_id 参数
8. THE GetTemplateList 接口 SHALL 返回模板列表或错误信息

### 需求 5: 日志和监控

**用户故事:** 作为运维人员，我希望有完整的日志记录，以便追踪问题和分析性能。

#### 验收标准

1. THE Subscription_Message_Service SHALL 记录每次API调用的 request_id
2. THE Subscription_Message_Service SHALL 记录每次操作的开始时间和结束时间
3. THE Subscription_Message_Service SHALL 记录令牌获取的耗时
4. THE Subscription_Message_Service SHALL 记录微信API调用的耗时
5. THE Subscription_Message_Service SHALL 记录总操作耗时
6. WHEN 操作失败，THE Subscription_Message_Service SHALL 记录错误级别日志，包含完整的错误堆栈
7. WHEN 操作成功，THE Subscription_Message_Service SHALL 记录信息级别日志，包含关键操作参数
8. THE Subscription_Message_Service SHALL 使用结构化日志格式（slog）

### 需求 6: 与现有架构集成

**用户故事:** 作为架构师，我希望新功能能够无缝集成到现有系统中，遵循项目的设计模式。

#### 验收标准

1. THE Subscription_Message_Service SHALL 使用 Token_Service 获取 Access_Token
2. THE Subscription_Message_Service SHALL 使用 WeChat_API_Client 调用微信API
3. THE Subscription_Message_Service SHALL 遵循现有的依赖注入模式（使用 fx 框架）
4. THE Subscription_Message_Service SHALL 实现与 Article_Service 相同的错误处理模式
5. THE Subscription_Message_Service SHALL 使用 context.Context 传递请求上下文
6. THE Subscription_Message_Service SHALL 支持 request_id 追踪
7. THE Subscription_Message_Service SHALL 使用项目统一的 slog.Logger 进行日志记录

### 需求 7: 消息内容验证

**用户故事:** 作为开发者，我希望系统能够验证消息内容的合法性，以避免发送失败。

#### 验收标准

1. THE Subscription_Message_Service SHALL 验证 Template_Data 中的所有键都是字符串类型
2. THE Subscription_Message_Service SHALL 验证 Template_Data 中的所有值都是字符串类型
3. THE Subscription_Message_Service SHALL 验证 page 参数（如果提供）是有效的小程序页面路径格式
4. THE Subscription_Message_Service SHALL 验证 OpenID 长度为 28 个字符
5. IF 验证失败，THEN THE Subscription_Message_Service SHALL 返回参数验证错误，而不调用微信API

### 需求 8: 并发和性能

**用户故事:** 作为系统管理员，我希望服务能够处理并发请求，保持良好的性能。

#### 验收标准

1. THE Subscription_Message_Service SHALL 支持并发处理多个发送请求
2. THE Subscription_Message_Service SHALL 使用 singleflight 模式防止令牌重复刷新
3. THE Subscription_Message_Service SHALL 在高并发场景下保持稳定性
4. THE Subscription_Message_Service SHALL 复用 HTTP 连接以提高性能
5. WHEN 模板列表缓存命中，THE Subscription_Message_Service SHALL 在 100 毫秒内返回结果

## 正确性属性

### 属性 0: 签名验证正确性（Invariant）
- 对于任何验证请求，只有当使用Token、timestamp、nonce计算的SHA1签名与请求中的signature匹配时，才应该返回echostr
- 签名验证失败的请求必须被拒绝

### 属性 1: 令牌刷新幂等性（Idempotence）
- 对于同一个 authorizer_app_id，多次调用令牌刷新应该得到有效的令牌
- 并发的令牌刷新请求应该只触发一次实际的API调用（通过 singleflight 保证）

### 属性 2: 错误处理一致性（Invariant）
- 所有微信API错误都应该被正确映射为服务层错误
- Error_Code 40001 和 42001 必须触发令牌刷新和重试
- 其他错误必须直接返回，不进行重试

### 属性 3: 日志完整性（Invariant）
- 每个请求必须有唯一的 request_id
- 每个操作必须记录开始和结束日志
- 失败的操作必须记录错误详情

### 属性 4: 参数验证先行（Invariant）
- 所有参数验证必须在调用微信API之前完成
- 验证失败必须立即返回错误，不消耗API配额

### 属性 5: 缓存一致性（Metamorphic）
- 缓存的模板列表应该与微信API返回的列表一致
- 缓存过期后重新获取的数据应该是最新的

### 属性 6: 超时边界（Invariant）
- 单次API调用（不含重试）的总耗时不应超过 30 秒
- 包含令牌刷新的重试操作总耗时不应超过 60 秒

### 属性 7: 并发安全性（Confluence）
- 并发发送不同消息的操作应该互不影响
- 并发刷新同一令牌的操作应该只触发一次实际刷新

## 非功能性需求

### 性能要求
- 单次消息发送（缓存命中）：< 2 秒
- 模板列表查询（缓存命中）：< 100 毫秒
- 支持并发：至少 100 QPS

### 可靠性要求
- 令牌过期自动恢复
- 详细的错误日志和追踪
- 优雅的错误处理，不崩溃

### 可维护性要求
- 遵循项目现有代码风格
- 完整的单元测试覆盖
- 清晰的接口文档

## 参考资料

- 微信小程序订阅消息API文档: https://developers.weixin.qq.com/miniprogram/dev/api-backend/open-api/subscribe-message/subscribeMessage.send.html
- 微信小程序前端订阅API: https://developers.weixin.qq.com/miniprogram/dev/api/open-api/subscribe-message/wx.requestSubscribeMessage.html
- 项目现有服务: internal/service/article_service.go, internal/service/token_service.go

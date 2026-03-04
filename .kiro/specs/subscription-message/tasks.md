# 实施计划：微信小程序订阅消息功能

## 概述

本实施计划将微信小程序订阅消息功能的设计转化为可执行的编码任务。实施将按照以下顺序进行：

1. **微信服务器验证**（前置任务）：实现微信公众号服务器URL验证功能
2. **数据模型定义**：定义订阅消息相关的数据结构和常量
3. **客户端扩展**：扩展WeChat API Client以支持订阅消息API
4. **服务实现**：实现SubscriptionMessageService核心业务逻辑
5. **Handler实现**：实现HTTP接口层
6. **依赖注入配置**：配置fx模块和路由
7. **测试和验证**：运行测试并验证功能

## 任务列表

- [x] 0. 实现微信服务器验证（前置任务）
  - [x] 0.1 在配置文件中添加 callback_token 配置项
    - 在 `internal/config/config.go` 中添加 CallbackToken 字段
    - 在 `configs/config.example.yaml` 中添加配置示例
    - _需求: 0.7_
  
  - [x] 0.2 创建 `internal/handler/wechat_callback_handler.go` 文件
    - 定义 `WeChatCallbackHandler` 结构体
    - 实现 `NewWeChatCallbackHandler` 构造函数
    - 定义服务器验证请求和响应结构体
    - _需求: 0.1, 0.2_
  
  - [x] 0.3 实现 `VerifyServer` 方法
    - 获取配置的 callback_token
    - 对 token、timestamp、nonce 进行字典序排序
    - 计算 SHA1 签名
    - 验证签名是否匹配
    - 返回 echostr 或错误
    - 记录详细日志
    - _需求: 0.2, 0.3, 0.4, 0.5, 0.6, 0.8_
  
  - [x] 0.4 在 HTTP Handler 中添加验证路由
    - 在 `internal/handler/http/handler.go` 中添加 GET /wechat/callback 路由
    - 解析查询参数（signature, timestamp, nonce, echostr）
    - 调用 VerifyServer 方法
    - 返回 echostr 或错误响应
    - _需求: 0.1, 0.4_
  
  - [ ]* 0.5 为服务器验证编写单元测试
    - 测试签名验证成功场景
    - 测试签名验证失败场景
    - 测试缺少参数场景
    - 测试Token未配置场景
    - _需求: 0.3, 0.4, 0.5_
  
  - [ ]* 0.6 为服务器验证编写属性测试
    - **属性 0: 签名验证正确性**
    - **验证需求: 0.3, 0.4**
  
  - [x] 0.7 在 fx 模块中注册 WeChatCallbackHandler
    - 在 `internal/fx/modules.go` 中添加依赖注入配置
    - _需求: 0.1_

- [x] 1. 定义数据模型和常量
  - [x] 1.1 在 `internal/wechat/models.go` 中添加订阅消息相关的数据结构
    - 添加 `SendSubscriptionMessageRequest` 结构体
    - 添加 `SendSubscriptionMessageResponse` 结构体
    - 添加 `GetTemplateListResponse` 结构体
    - 添加 `SubscriptionTemplate` 结构体
    - _需求: 1.1, 1.5, 1.6, 2.1, 2.3, 4.1, 4.2_
  
  - [x] 1.2 在 `internal/wechat/constants.go` 中添加错误码常量
    - 添加订阅消息相关错误码常量（43101, 40037, 45009, 40003, 47003）
    - _需求: 3.2, 3.3, 3.4_
  
  - [x] 1.3 在 `internal/repository/cache/keys.go` 中添加缓存键格式化函数
    - 添加 `FormatTemplateListKey` 函数
    - _需求: 2.5_

- [x] 2. 扩展 WeChat API Client
  - [x] 2.1 在 `internal/wechat/client/client.go` 中添加 `SendSubscriptionMessage` 方法
    - 实现HTTP POST请求到微信API
    - 处理响应和错误
    - 设置30秒超时
    - _需求: 1.1, 1.8, 3.5_
  
  - [x] 2.2 在 `internal/wechat/client/client.go` 中添加 `GetSubscriptionTemplateList` 方法
    - 实现HTTP GET请求到微信API
    - 处理响应和错误
    - _需求: 2.2_
  
  - [ ]* 2.3 为 WeChat API Client 方法编写单元测试
    - 测试成功场景
    - 测试各种错误码处理
    - 测试超时场景
    - _需求: 1.1, 2.2, 3.5_

- [x] 3. 实现 SubscriptionMessageService
  - [x] 3.1 创建 `internal/service/subscription_message_service.go` 文件
    - 定义 `SubscriptionMessageService` 接口
    - 定义服务层的请求和响应结构体
    - 实现 `SubscriptionMessageServiceImpl` 结构体
    - 实现 `NewSubscriptionMessageService` 构造函数
    - _需求: 4.1, 4.6, 6.1, 6.2, 6.3, 6.5, 6.8_
  
  - [x] 3.2 实现参数验证函数 `validateRequest`
    - 验证 OpenID 长度为28字符
    - 验证 data 字段数量不超过20个
    - 验证 data 字段值长度不超过20字符
    - 验证 data 字段格式（必须包含 "value" 键）
    - 验证 page 路径格式（如果提供）
    - _需求: 1.2, 1.3, 1.4, 4.3, 4.4, 4.5, 7.1, 7.2, 7.3, 7.4, 7.5_
  
  - [ ]* 3.3 为参数验证函数编写属性测试
    - **属性 1: 参数验证先行**
    - **验证需求: 1.2, 1.3, 1.4, 4.3, 4.4, 4.5, 7.1, 7.2, 7.3, 7.4, 7.5**
  
  - [x] 3.4 实现 `SendSubscriptionMessage` 方法
    - 确保 request_id 存在
    - 调用参数验证
    - 获取访问令牌
    - 构造微信API请求
    - 调用微信API
    - 处理令牌过期错误（自动刷新并重试）
    - 记录详细日志（开始、令牌获取、API调用、完成）
    - _需求: 1.1, 1.5, 1.6, 1.7, 1.8, 1.9, 3.1, 3.6, 4.1, 4.2, 5.1, 5.2, 5.3, 5.4, 5.5, 5.6, 5.7, 6.1, 6.2, 6.5, 6.6_
  
  - [ ]* 3.5 为 SendSubscriptionMessage 编写单元测试
    - 测试成功发送场景
    - 测试参数验证失败场景
    - 测试令牌过期自动恢复场景
    - 测试各种错误码处理（43101, 40037, 45009等）
    - 测试日志记录完整性
    - _需求: 1.1, 1.5, 1.6, 1.7, 3.1, 3.2, 3.3, 3.4_
  
  - [ ]* 3.6 为 SendSubscriptionMessage 编写属性测试
    - **属性 2: 令牌过期自动恢复**
    - **验证需求: 1.7, 3.1, 3.6**
    - **属性 3: 成功响应包含消息ID**
    - **验证需求: 1.5**
    - **属性 4: 错误响应包含错误信息**
    - **验证需求: 1.6**
  
  - [x] 3.7 实现 `GetTemplateList` 方法
    - 确保 request_id 存在
    - 检查缓存
    - 如果缓存命中，返回缓存结果
    - 如果缓存未命中，获取访问令牌
    - 调用微信API获取模板列表
    - 处理令牌过期错误（自动刷新并重试）
    - 缓存结果（30分钟）
    - 记录详细日志
    - _需求: 2.1, 2.2, 2.3, 2.4, 2.5, 2.6, 4.6, 4.7, 4.8, 5.1, 5.2, 5.3, 5.4, 5.5_
  
  - [ ]* 3.8 为 GetTemplateList 编写单元测试
    - 测试缓存命中场景
    - 测试缓存未命中场景
    - 测试空模板列表处理
    - 测试令牌过期自动恢复
    - _需求: 2.2, 2.4, 2.5, 2.6_
  
  - [ ]* 3.9 为 GetTemplateList 编写属性测试
    - **属性 5: 模板列表缓存一致性**
    - **验证需求: 2.5, 2.6**
    - **属性 6: 空模板列表处理**
    - **验证需求: 2.4**
  
  - [x] 3.10 实现错误处理辅助函数
    - 实现 `isTokenExpiredError` 函数
    - 实现 `isSubscriptionExpiredError` 函数
    - 实现 `isRateLimitError` 函数
    - 实现 `handleError` 函数（错误分类和映射）
    - _需求: 3.1, 3.2, 3.3, 3.4, 3.7_
  
  - [ ]* 3.11 为错误处理函数编写单元测试
    - 测试各种错误码的正确识别
    - 测试错误消息的正确映射
    - _需求: 3.1, 3.2, 3.3, 3.4_
  
  - [ ]* 3.12 编写属性测试验证错误处理一致性
    - **属性 7: 特定错误码映射正确性**
    - **验证需求: 3.2, 3.3, 3.4**

- [x] 4. 实现 HTTP Handler
  - [x] 4.1 创建 `internal/handler/subscription_message_handler.go` 文件
    - 定义 `SubscriptionMessageHandler` 结构体
    - 实现 `NewSubscriptionMessageHandler` 构造函数
    - 实现 `RegisterRoutes` 方法
    - _需求: 6.3_
  
  - [x] 4.2 实现 `SendSubscriptionMessage` HTTP handler
    - 解析请求参数
    - 调用 service 层
    - 返回JSON响应
    - 处理错误
    - _需求: 4.1, 4.2_
  
  - [x] 4.3 实现 `GetTemplateList` HTTP handler
    - 解析请求参数
    - 调用 service 层
    - 返回JSON响应
    - 处理错误
    - _需求: 4.6, 4.7, 4.8_
  
  - [ ]* 4.4 为 HTTP handlers 编写单元测试
    - 测试成功场景
    - 测试参数解析错误
    - 测试服务层错误处理
    - _需求: 4.1, 4.2, 4.6, 4.7, 4.8_

- [x] 5. 配置依赖注入和路由
  - [x] 5.1 在 `internal/fx/modules.go` 中添加 SubscriptionMessageModule
    - 注册 SubscriptionMessageService
    - 注册 SubscriptionMessageHandler
    - _需求: 6.3_
  
  - [x] 5.2 在 `cmd/server/main.go` 中注册模块
    - 添加 SubscriptionMessageModule 到 fx.New
    - _需求: 6.3_
  
  - [x] 5.3 在路由注册中添加订阅消息路由
    - 注册 POST /api/v1/subscription-message/send
    - 注册 GET /api/v1/subscription-message/templates
    - _需求: 4.1, 4.6_

- [x] 6. 检查点 - 确保所有测试通过
  - 运行所有单元测试
  - 运行所有属性测试
  - 检查代码覆盖率（目标 > 80%）
  - 如有问题请询问用户

- [ ] 7. 添加并发安全性测试（可选）
  - [ ]* 7.1 编写并发请求独立性属性测试
    - **属性 11: 并发请求独立性**
    - **验证需求: 8.1**
  
  - [ ]* 7.2 编写并发令牌刷新幂等性属性测试
    - **属性 10: 并发令牌刷新幂等性**
    - **验证需求: 8.2**
  
  - [ ]* 7.3 编写并发场景的压力测试
    - 测试100并发请求
    - 验证性能目标（< 2秒响应时间）
    - _需求: 8.1, 8.3_

- [ ] 8. 添加日志和监控（可选）
  - [ ]* 8.1 添加 Prometheus 指标
    - 添加 `subscription_message_sent_total` 计数器
    - 添加 `subscription_message_duration_seconds` 直方图
    - 添加 `template_list_cache_hits_total` 计数器
    - _需求: 5.1, 5.2, 5.3, 5.4, 5.5_
  
  - [ ]* 8.2 编写属性测试验证日志完整性
    - **属性 8: 日志完整性**
    - **验证需求: 1.9, 3.7, 5.1, 5.2, 5.3, 5.4, 5.5, 5.6, 5.7, 6.6**

- [x] 9. 最终检查点
  - 确保所有测试通过
  - 验证代码符合项目规范
  - 检查日志输出格式正确
  - **验证微信服务器URL配置成功**
  - 如有问题请询问用户

## 微信公众号后台配置步骤

完成代码实施后，需要在微信公众号后台进行以下配置：

1. **登录微信公众号后台**
   - 访问 https://mp.weixin.qq.com/

2. **进入服务器配置**
   - 点击左侧菜单：设置与开发 -> 基本配置
   - 找到"服务器配置"，点击"修改配置"

3. **填写服务器信息**
   - **URL**: 填入你的服务器接收消息的接口
     - 格式：`https://your-domain.com/wechat/callback`
     - 或：`https://your-domain.com/wechat/callback?authorizer_appid=YOUR_APPID`（如果需要区分多个公众号）
   - **Token**: 填入你在配置文件中设置的 `callback_token`
   - **EncodingAESKey**: 点击"随机生成"
   - **消息加解密方式**: 初期调试建议选择"明文模式"

4. **验证服务器**
   - **重要**: 在点击"提交"之前，确保你的服务器代码已经部署并运行
   - 点击"提交"按钮
   - 微信服务器会向你的URL发送GET请求进行验证
   - 如果验证成功，会显示"提交成功"
   - 点击右侧的"启用"按钮

5. **验证成功标志**
   - 服务器配置状态显示为"已启用"
   - 可以在服务器日志中看到验证请求的记录

6. **常见问题排查**
   - 如果验证失败，检查：
     - 服务器是否可以从公网访问
     - URL是否正确（必须是HTTPS，除非是测试号）
     - Token配置是否与代码中一致
     - 服务器是否正常运行并监听正确的端口
     - 查看服务器日志中的错误信息

## 注意事项

- 标记 `*` 的任务为可选任务，可以跳过以加快MVP开发
- 每个任务都引用了具体的需求编号，便于追溯
- 检查点任务确保增量验证
- 属性测试使用 gopter 库验证通用正确性属性
- 单元测试验证具体示例和边界情况
- 所有实现应遵循现有项目的代码风格和架构模式

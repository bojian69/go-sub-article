# WeChat Subscription Service API 文档

## 概述

微信公众号订阅微服务提供 HTTP REST API 和 gRPC 两种接口，用于获取公众号已发布的图文消息。

- **HTTP Base URL**: `http://localhost:8080`
- **gRPC Address**: `localhost:9090`

## HTTP REST API

### 1. 获取图文列表

获取指定公众号已发布的图文消息列表。

**请求**

```
GET /v1/accounts/{authorizer_appid}/articles
```

**路径参数**

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| authorizer_appid | string | 是 | 授权公众号的 AppID |

**查询参数**

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|--------|------|
| offset | int | 否 | 0 | 起始位置 |
| count | int | 否 | 10 | 返回数量，范围 1-20 |
| no_content | int | 否 | 0 | 是否不返回 content 字段，1=不返回 |

**响应示例**

```json
{
  "code": 0,
  "message": "success",
  "request_id": "550e8400-e29b-41d4-a716-446655440000",
  "data": {
    "total_count": 100,
    "item_count": 2,
    "item": [
      {
        "article_id": "ARTICLE_ID_1",
        "content": {
          "news_item": [
            {
              "title": "文章标题",
              "author": "作者",
              "digest": "摘要",
              "content": "<p>HTML内容</p>",
              "content_source_url": "https://example.com",
              "thumb_media_id": "THUMB_MEDIA_ID",
              "thumb_url": "https://mmbiz.qpic.cn/xxx",
              "need_open_comment": 0,
              "only_fans_can_comment": 0,
              "url": "https://mp.weixin.qq.com/s/xxx",
              "is_deleted": false
            }
          ]
        },
        "update_time": 1609459200
      }
    ]
  }
}
```

**错误响应**

```json
{
  "code": 400001,
  "message": "count must be between 1 and 20",
  "request_id": "550e8400-e29b-41d4-a716-446655440000"
}
```

### 2. 获取图文详情

获取指定图文的详细信息。

**请求**

```
GET /v1/accounts/{authorizer_appid}/articles/{article_id}
```

**路径参数**

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| authorizer_appid | string | 是 | 授权公众号的 AppID |
| article_id | string | 是 | 图文消息 ID |

**响应示例**

```json
{
  "code": 0,
  "message": "success",
  "request_id": "550e8400-e29b-41d4-a716-446655440000",
  "data": {
    "news_item": [
      {
        "title": "文章标题",
        "author": "作者",
        "digest": "摘要",
        "content": "<p>完整HTML内容</p>",
        "content_source_url": "https://example.com",
        "thumb_media_id": "THUMB_MEDIA_ID",
        "thumb_url": "https://mmbiz.qpic.cn/xxx",
        "need_open_comment": 0,
        "only_fans_can_comment": 0,
        "url": "https://mp.weixin.qq.com/s/xxx",
        "is_deleted": false
      }
    ]
  }
}
```

## gRPC API

### Proto 定义

```protobuf
service SubscriptionService {
  rpc BatchGetPublishedArticles(BatchGetArticlesRequest) returns (BatchGetArticlesResponse);
  rpc GetPublishedArticle(GetArticleRequest) returns (GetArticleResponse);
}
```

### 1. BatchGetPublishedArticles

获取图文列表。

**请求**

```protobuf
message BatchGetArticlesRequest {
  string authorizer_appid = 1;  // 公众号 AppID
  int32 offset = 2;             // 起始位置
  int32 count = 3;              // 返回数量 (1-20)
  int32 no_content = 4;         // 是否不返回 content (0 或 1)
}
```

**响应**

```protobuf
message BatchGetArticlesResponse {
  int32 total_count = 1;
  int32 item_count = 2;
  repeated PublishedArticle item = 3;
}
```

### 2. GetPublishedArticle

获取图文详情。

**请求**

```protobuf
message GetArticleRequest {
  string authorizer_appid = 1;  // 公众号 AppID
  string article_id = 2;        // 图文 ID
}
```

**响应**

```protobuf
message GetArticleResponse {
  repeated NewsItem news_item = 1;
}
```

## 错误码

| 错误码 | 说明 |
|--------|------|
| 0 | 成功 |
| 400001 | 参数错误 |
| 401001 | 未授权 |
| 404001 | 资源不存在 |
| 500001 | 微信 API 错误 |
| 500002 | Redis 错误 |
| 500003 | 内部错误 |

## gRPC 状态码映射

| 场景 | gRPC Status |
|------|-------------|
| 参数验证失败 | InvalidArgument |
| 公众号未找到 | NotFound |
| 服务内部错误 | Internal |

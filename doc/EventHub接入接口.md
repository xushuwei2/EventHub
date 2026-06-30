# EventHub 接入接口

## 1. 适用范围

本文档面向所有需要接入 `EventHub` 的产品前后端开发。  
平台按项目隔离，接入方只需实现：

- 前端事件采集与批量上报
- 后端签发可信 `reportToken`

## 2. 接入流程

### 2.1 项目注册

接入前需要在 `EventHub` 创建项目配置，最少提供：

- `projectKey`
- `projectName`
- 是否允许匿名上报
- 后端签发 `reportToken` 用的共享密钥

### 2.2 前端配置

前端至少需要配置：

- `projectKey`
- `release`
- `env`
- `reportingBaseUrl`

### 2.3 后端配置

业务后端需要持有该项目的 `trustedTokenSecret`，用于本地签发 `reportToken`。  
`EventHub` 不会参与登录链路上的 token 生成。

## 3. 上报模式

### 3.1 匿名上报

适用场景：

- 启动期错误
- 资源加载失败
- 登录前错误

请求头：

- `Content-Type: application/json`
- `X-Project-Key: <projectKey>`

特点：

- 不依赖业务登录
- 限流更严格
- 服务端不信任 `userId`、`roomId` 等身份字段

### 3.2 可信上报

适用场景：

- 登录后错误
- 业务会话内错误
- 关键业务错误

请求头：

- `Content-Type: application/json`
- `Authorization: Bearer <reportToken>`

特点：

- 服务端信任 token 内的项目与会话身份
- 可关联用户、业务会话、版本

## 4. `reportToken` 规范

### 4.1 签名算法

首版推荐：

- `HS256`

### 4.2 推荐 claims

- `project_key`：项目标识
- `user_id`：用户 ID，可空
- `session_id`：业务会话 ID，可空
- `room_id`：房间 ID，可空；没有房间概念的业务可省略
- `release`：当前版本号
- `exp`：过期时间，建议 10-30 分钟

### 4.3 约束

- `reportToken` 只用于错误上报
- 不可复用业务登录 token
- 不可在前端内置签名密钥

## 5. 上报接口

### 5.1 批量事件上报

`POST /reporting/v1/events/batch`

说明：

- 推荐所有产品统一走批量接口
- 即使只有 1 条事件，也放入 `events` 数组

请求体：

```json
{
  "clientSentAt": "2026-06-30T12:00:00Z",
  "events": [
    {
      "eventId": "6f4c8dbe-f6df-4b38-9a1c-7cdb22ef0001",
      "occurredAt": "2026-06-30T11:59:58Z",
      "release": "0.1.3",
      "env": "prod",
      "category": "api_failure",
      "severity": "error",
      "message": "提交订单失败：HTTP 500",
      "route": "checkout",
      "scene": "checkout",
      "module": "platform/api/order",
      "stack": "Error: ...",
      "language": "zh-CN",
      "runtime": "web",
      "devicePlatform": "android",
      "userId": "user_123",
      "roomId": null,
      "sessionId": "sess_001",
      "apiMethod": "POST",
      "apiPath": "/api/orders",
      "httpStatus": 500,
      "extra": {
        "uiAction": "tap_submit_order"
      }
    }
  ]
}
```

成功响应：

```json
{
  "requestId": "req_20260630_0001",
  "accepted": 1,
  "rejected": 0
}
```

失败响应示例：

```json
{
  "error": "invalid_token"
}
```

## 6. 事件字段说明

### 6.1 通用字段

| 字段 | 必填 | 说明 |
|------|------|------|
| `eventId` | 是 | 事件唯一 ID，建议 UUID |
| `occurredAt` | 是 | 事件发生时间，ISO8601 |
| `release` | 是 | 产品版本号 |
| `env` | 是 | `prod` / `staging` / `dev` |
| `category` | 是 | 事件分类 |
| `severity` | 是 | `fatal` / `error` / `warn` |
| `message` | 是 | 错误摘要 |
| `route` | 否 | 当前路由/页面 |
| `scene` | 否 | 当前场景 |
| `module` | 否 | 业务模块 |
| `stack` | 否 | 错误堆栈 |
| `file` | 否 | 源文件 |
| `line` | 否 | 行号 |
| `column` | 否 | 列号 |
| `language` | 否 | `ja` / `zh-CN` 等 |
| `runtime` | 否 | `web` / `webview` / `miniapp` |
| `devicePlatform` | 否 | `ios` / `android` / `unknown` |
| `deviceModel` | 否 | 设备型号 |
| `osVersion` | 否 | 系统版本 |
| `sdkVersion` | 否 | 宿主 SDK 版本 |
| `networkType` | 否 | 网络类型 |
| `userId` | 否 | 用户 ID |
| `roomId` | 否 | 房间 ID |
| `sessionId` | 否 | 业务会话 ID |
| `extra` | 否 | 扩展上下文对象 |

### 6.2 `category` 枚举

- `uncaught_js`
- `unhandled_promise`
- `api_failure`
- `ws_failure`
- `asset_failure`
- `biz_error`

### 6.3 分类补充字段

#### `api_failure`

| 字段 | 必填 | 说明 |
|------|------|------|
| `apiMethod` | 是 | HTTP 方法 |
| `apiPath` | 是 | 请求路径，使用原始路径 |
| `httpStatus` | 否 | HTTP 状态码 |

#### `ws_failure`

| 字段 | 必填 | 说明 |
|------|------|------|
| `wsPhase` | 是 | `connect` / `handshake` / `message` / `close` |
| `wsCode` | 否 | 关闭码 |
| `wsReason` | 否 | 关闭原因 |

#### `asset_failure`

| 字段 | 必填 | 说明 |
|------|------|------|
| `assetType` | 是 | `image` / `audio` / `svg` / `manifest` |
| `assetPath` | 否 | 资源相对路径 |
| `assetUrl` | 否 | 资源 URL |

#### `biz_error`

| 字段 | 必填 | 说明 |
|------|------|------|
| `bizCode` | 是 | 稳定业务码，例如 `room_result_retry_exhausted` |

## 7. 限制与建议

### 7.1 平台限制

- 单次请求建议不超过 64 KB
- 单批事件建议不超过 20 条
- `message` 建议不超过 512 字符
- `stack` 建议不超过 8 KB
- `extra` 建议只放排查必须字段

### 7.2 缺字段策略

若宿主环境拿不到字段，例如小游戏壳、App WebView 或受限浏览器环境中常见的：

- `deviceModel`
- `osVersion`
- `networkType`

则可以直接省略，不要伪造默认值。

### 7.3 重试策略

- `fatal`：立即发 1 次，失败后最多重试 1 次
- `error/warn`：进入批量队列
- 队列失败后指数退避
- 不要因为上报失败阻塞业务流程

## 8. 服务端错误码

| 错误码 | 含义 |
|------|------|
| `invalid_project` | `projectKey` 不存在或已停用 |
| `invalid_token` | `reportToken` 无效 |
| `token_expired` | `reportToken` 已过期 |
| `payload_too_large` | 请求体过大 |
| `too_many_events` | 单批事件过多 |
| `rate_limited` | 触发限流 |
| `invalid_event` | 事件结构非法 |

## 9. 推荐接入步骤

1. 注册项目并拿到 `projectKey`
2. 业务后端接入 `reportToken` 本地签发
3. 前端实现统一 `errorReporter`
4. 先接入 `window.onerror`、`unhandledrejection`
5. 再接入 API / WS / 资源失败
6. 最后给关键业务异常补稳定 `bizCode`
7. 联调后台 issue 聚合与详情展示

## 10. 接入验收清单

- 能上报 1 条未捕获错误
- 能上报 1 条 API 500
- 能上报 1 条 WS 建连失败
- 能上报 1 条资源加载失败
- 同类错误重复出现时后台只增长计数，不新增 issue
- 缺失设备字段时服务端正常接收

## 11. 首批接入建议

任意业务首批接入时，建议优先覆盖：

- 启动阶段错误
- 登录或会话建立相关错误
- 核心 API 失败
- WebSocket 或实时连接失败
- 远程资源清单与图片/音频加载失败
- 会导致用户主流程中断的关键业务异常

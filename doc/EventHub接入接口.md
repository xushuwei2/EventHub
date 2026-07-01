# EventHub 接入接口

## 1. 适用范围

本文档面向所有需要接入 `EventHub` 的产品前后端开发。  
平台按项目隔离，接入方只需实现：

- 前端事件采集与批量上报
- 后端签发可信 `reportToken`

## 2. 接入流程

### 2.1 项目注册

在 EventHub 后台 **项目配置** 页面创建项目：

- 地址：`/reporting/admin/projects`（需先登录后台）
- 路径示例：`https://<域名>/reporting/admin/projects`

每个项目需配置：

| 字段 | 说明 |
|------|------|
| `projectKey` | 项目唯一标识。小写字母开头，仅含 `a-z`、`0-9`、`_`、`-`；创建后不可修改 |
| `trustedTokenSecret` | 业务后端签发 `reportToken` 用的共享密钥。可手动填写、随机生成，或留空由服务端自动生成 |
| `status` | `active` / `disabled`，停用后该项目上报会被拒绝 |

创建或修改密钥后，请将 `trustedTokenSecret` 同步到业务后端配置。

### 2.2 前端配置

前端至少需要配置：

- `projectKey`
- `release`
- `env`
- `reportingBaseUrl`

### 2.3 后端配置

业务后端需要持有该项目的 `trustedTokenSecret`，用于本地签发 `reportToken`。  
`EventHub` 不会参与登录链路上的 token 生成。

## 3. 上报认证

所有上报均需携带业务后端签发的 `reportToken`。

请求头：

- `Content-Type: application/json`
- `Authorization: Bearer <reportToken>`

特点：

- 服务端信任 token 内的项目与会话身份
- 可关联用户、业务会话、版本
- 登录前错误也需由业务后端签发 token 后上报

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

### 4.3 约束

- `reportToken` 不设过期时间，服务端仅校验签名与 `project_key`
- `reportToken` 只用于错误上报
- 不可复用业务登录 token
- 不可在前端内置签名密钥

## 5. 上报接口

### 5.1 批量事件上报

`POST /reporting/v1/events/batch`

请求头：

- `Content-Type: application/json`
- `Authorization: Bearer <reportToken>`

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
| `payload_too_large` | 请求体过大 |
| `too_many_events` | 单批事件过多 |
| `invalid_event` | 事件结构非法 |

## 9. 推荐接入步骤

1. 在后台「项目配置」创建项目，记录 `projectKey` 与 `trustedTokenSecret`
2. 将 `trustedTokenSecret` 配置到业务后端
3. 业务后端接入 `reportToken` 本地签发
4. 前端实现统一 `errorReporter`，上报时携带 `reportToken`
5. 先接入 `window.onerror`、`unhandledrejection`
6. 再接入 API / WS / 资源失败
7. 最后给关键业务异常补稳定 `bizCode`
8. 联调后台 issue 聚合与详情展示

## 10. 接入验收清单

- 后台能创建项目并获取 `trustedTokenSecret`
- 业务后端能签发有效 `reportToken`
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

## 12. 行为打点（活跃 / 留存 / 漏斗）

除错误上报外，EventHub 支持独立的行为打点管道，用于统计 DAU/MAU、留存 cohort 与漏斗转化率。

### 12.1 批量打点上报

`POST /reporting/v1/track/batch`

鉴权与错误上报相同：`Authorization: Bearer <reportToken>`

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
      "eventName": "page_view",
      "route": "home",
      "funnelKey": "register_flow",
      "stepKey": "home_view",
      "extra": {}
    }
  ]
}
```

成功响应格式与错误 batch 相同：

```json
{
  "requestId": "req_20260630_0001",
  "accepted": 1,
  "rejected": 0
}
```

### 12.2 字段说明

| 字段 | 必填 | 说明 |
|------|------|------|
| `eventId` | 是 | 事件唯一 ID，建议 UUID |
| `occurredAt` | 是 | 事件发生时间，ISO8601 |
| `release` | 是 | 产品版本号 |
| `env` | 是 | `prod` / `staging` / `dev` |
| `eventName` | 是 | 打点名称，如 `page_view`、`tap_buy` |
| `route` / `scene` / `module` | 否 | 页面或业务上下文 |
| `funnelKey` / `stepKey` | 否 | 漏斗标识，须成对出现且与后台配置一致 |
| `extra` | 否 | 扩展字段 |

设备、语言等上下文字段与错误上报相同，可省略。

### 12.3 活跃与留存口径

- **活跃（DAU/MAU）**：当日/近 30 日有任意行为打点的去重 `user_id`
- **留存**：以用户首次活跃日为 cohort，计算 D1/D7/D14/D30 回访率
- **重要**：`reportToken` 中须携带 `user_id`，否则事件仅计入总量，不参与 DAU/留存/漏斗用户统计

### 12.4 漏斗配置

1. 在后台「漏斗转化」创建漏斗，配置 `funnelKey` 与有序步骤（`stepKey`）
2. 前端打点时携带相同的 `funnelKey` + `stepKey`
3. 后台按转化窗口（默认 24 小时）计算逐步转化率

### 12.5 推荐首批打点

- `app_launch` — 应用启动
- `page_view` — 页面浏览
- 核心按钮点击（如 `tap_submit_order`）
- 关键业务完成（如 `register_success`、`payment_success`）

### 12.6 后台查看

- 活跃概览：`/reporting/admin/analytics`
- 留存分析：`/reporting/admin/analytics/retention`
- 漏斗转化：`/reporting/admin/funnels`

## 13. 用户主动反馈

App 内「意见反馈」入口：用户手写问题描述后提交。与自动错误上报不同，每条反馈独立存储，不做聚合。

### 13.1 提交反馈

`POST /reporting/v1/feedback`

鉴权与错误上报相同：`Authorization: Bearer <reportToken>`

请求体：

```json
{
  "feedbackId": "6f4c8dbe-f6df-4b38-9a1c-7cdb22ef0001",
  "submittedAt": "2026-07-01T12:00:00Z",
  "release": "1.0.0",
  "env": "prod",
  "category": "bug",
  "content": "点击支付按钮后没有任何反应，试了好几次都一样",
  "contact": "user@example.com",
  "route": "checkout",
  "scene": "payment",
  "language": "zh-CN",
  "runtime": "webview",
  "devicePlatform": "ios",
  "extra": {
    "screenshotUrl": "https://cdn.example.com/shot.png"
  }
}
```

成功响应：

```json
{
  "requestId": "req_20260701_0001",
  "feedbackId": "6f4c8dbe-f6df-4b38-9a1c-7cdb22ef0001"
}
```

### 13.2 字段说明

| 字段 | 必填 | 说明 |
|------|------|------|
| `feedbackId` | 是 | 反馈唯一 ID，建议 UUID |
| `submittedAt` | 否 | 提交时间，ISO8601；缺省为服务端接收时间 |
| `release` | 是 | 产品版本号 |
| `env` | 是 | `prod` / `staging` / `dev` |
| `category` | 否 | `bug` / `suggestion` / `question` / `other`，默认 `other` |
| `content` | 是 | 用户手写描述，建议不超过 2000 字 |
| `contact` | 否 | 用户留下的联系方式（邮箱/手机号等），最多 256 字符 |
| `route` / `scene` | 否 | 当前页面或业务场景 |
| `extra` | 否 | 扩展字段，如截图 URL |

设备、语言等上下文字段与错误上报相同，可省略。`userId` 优先取自 `reportToken`。

### 13.3 错误码

| 错误码 | 含义 |
|------|------|
| `invalid_feedback` | 反馈结构非法或内容超长 |
| `duplicate_feedback` | `feedbackId` 重复提交 |

### 13.4 后台查看

- 反馈列表：`/reporting/admin/feedback`
- 反馈详情：`/reporting/admin/feedback/{id}`

支持按项目、分类、状态、用户 ID 筛选；详情页可更新处理状态（`open` / `processing` / `resolved` / `closed`）并填写内部备注。

# EventHub 设计

## 1. 背景

前端产品发布到生产环境后，错误往往无法稳定回流到研发侧，线上问题只能依赖用户口述、临时远程调试或业务后端侧残留日志，定位成本高。当前这类系统通常存在以下客观约束：

- 某些宿主环境下，设备型号、系统版本、网络类型等字段可能无法获取，例如小游戏壳、App WebView 或受限浏览器环境。
- 启动期、资源加载期、登录前阶段也可能发生关键错误，这类错误不能依赖业务后端登录成功后再上报。
- 后续通常会有多个产品接入，因此方案不能写死为某个单独项目的专用逻辑。

本设计的目标是新增一个名为 `EventHub` 的独立前端事件采集服务，用于统一接收、清洗、聚合、查询多产品前端错误，并在服务内提供带用户名密码登录的简易后台页面。

## 2. 目标与非目标

### 2.1 目标

- 支持采集以下事件：
  - 未捕获错误
  - Promise 未处理拒绝
  - API 请求失败
  - WebSocket 连接/握手/异常关闭失败
  - 资源加载失败
  - 关键业务异常
- 支持按错误指纹聚合，并查看最近版本、语言、平台、接口、资源路径等维度。
- 支持多产品接入，按 `projectKey` 隔离数据。
- 提供最小可用后台页面，支持登录、列表筛选、详情查看、状态流转。
- 提供清晰的接入接口文档，便于其他产品复用。

### 2.2 非目标

- 首版不做用户主动反馈入口。
- 首版不做复杂告警编排，仅保留后续扩展位。
- 首版不做完整多管理员体系，仅支持静态配置的后台账号。
- 首版不做第三方平台依赖，例如 Sentry、GlitchTip。

## 3. 总体架构

### 3.1 组件划分

- 前端产品侧
  - 每个产品接入统一的 `errorReporter` 封装。
  - `errorReporter` 负责采集标准化、去重、批量发送、生命周期补发。
- 产品业务后端
  - 登录/建会话成功后，按共享密钥本地签发短时 `reportToken`。
  - 该 token 只用于错误上报，不参与业务接口鉴权。
- `EventHub`
  - 接收事件
  - 验证项目身份
  - 清洗归一化
  - 生成指纹并聚合
  - 保存原始事件与日统计
  - 提供后台管理页面
- MySQL
  - 可与现有业务数据库共用实例，但必须使用独立表。
- 反向代理
  - 统一暴露 `/reporting/*` 路径到 `EventHub`。

### 3.2 部署形态

推荐部署为“独立服务 + 同域反向代理路径”：

- 业务接口：`/api/*`、`/ws/*`
- 错误采集：`/reporting/v1/*`
- 后台页面：`/reporting/admin/*`

这样做的原因：

- 业务侧通常只需维护一个主域名或主网关入口。
- 前端只需维护一个主域名。
- 服务仍然保持独立进程/独立容器，便于单独扩容和限流。

## 4. 多产品模型

### 4.1 项目注册

`EventHub` 不是某个业务项目的专用服务，而是多租户的“项目级接入”模型。每个接入产品至少需要配置：

- `projectKey`：前端公开项目标识，唯一。
- `projectName`：后台展示名称。
- `status`：`active` / `disabled`。
- `trustedTokenSecret`：产品后端签发 `reportToken` 用的共享密钥。
- `anonymousIngestEnabled`：是否允许匿名上报。
- `sampleRateConfig`：可选，后续用于降采样。

其中：

- `projectKey` 可以出现在前端包中。
- `trustedTokenSecret` 只能由产品后端和 `EventHub` 持有，不能下发前端。

### 4.2 数据隔离

所有 issue、event、统计数据都按项目隔离，后台页面首层筛选需支持按项目过滤。  
聚合指纹同样带 `projectKey` 维度，避免不同产品同文案错误被错误合并。

## 5. 前端采集设计

### 5.1 采集入口

首版要求覆盖以下入口：

- `window.onerror`
- `window.onunhandledrejection`
- 统一 API 请求封装
- 统一 WebSocket 封装
- 统一资源预加载封装
- 明确标注的关键业务异常埋点

关键业务异常不直接抓全量 `logger.warn`，而是要求业务代码显式调用：

- `reportBizError(code, message, context)`

原因很精确：`warn` 里会混入调试信息、可恢复降级和提示类消息，全量上报会淹没真正问题。

### 5.2 发送策略

- `fatal` 事件立即发送。
- `error/warn` 事件进入本地队列，按“5 秒或 10 条”批量发送。
- 页面、WebView 或小游戏宿主切后台时主动 flush。
- 下次启动时尝试补发上一轮未成功发送的少量事件。
- 同一短时间内的重复事件做本地去重，避免雪崩上报。

### 5.3 失败降级

错误上报必须是旁路能力，不能阻塞业务主链路：

- 采集服务失败时，业务不能报错。
- 上报失败不弹玩家提示。
- 登录、页面进入、资源下载、核心业务流程都不能依赖上报结果。

## 6. 身份与鉴权

### 6.1 匿名模式

用于登录前、启动期、资源加载期等无法拿到业务会话的场景。

- 请求头带 `X-Project-Key`
- 不信任客户端上送的 `userId`、`roomId` 等业务身份字段
- 服务端只把这些字段当“参考字段”，不作为可信身份
- 匿名模式限流最严

### 6.2 可信模式

用于登录后、进入业务态后的错误上报。

- 请求头带 `Authorization: Bearer <reportToken>`
- `reportToken` 由产品业务后端本地签发
- 建议采用 HMAC JWT

推荐 claims：

- `project_key`
- `user_id`
- `session_id`
- `room_id`（可空）
- `release`
- `exp`

可信模式的优势：

- 不依赖 `EventHub` 在线签 token
- 避免把 `EventHub` 变成登录链路前置依赖
- 多产品后端只需实现标准 JWT 签发即可接入

## 7. 事件模型

### 7.1 通用字段

- `eventId`
- `occurredAt`
- `projectKey`
- `release`
- `env`
- `category`
- `severity`
- `message`
- `route`
- `scene`
- `module`
- `stack`
- `file`
- `line`
- `column`
- `language`
- `runtime`
- `devicePlatform`
- `deviceModel`
- `osVersion`
- `sdkVersion`
- `networkType`
- `userId`
- `roomId`
- `sessionId`
- `extra`

### 7.2 分类字段

- `api_failure`
  - `apiMethod`
  - `apiPath`
  - `httpStatus`
- `ws_failure`
  - `wsPhase`
  - `wsCode`
  - `wsReason`
- `asset_failure`
  - `assetType`
  - `assetPath`
  - `assetUrl`
- `biz_error`
  - `bizCode`

### 7.3 缺失字段原则

某些受限宿主环境可能拿不到如下字段：

- `deviceModel`
- `osVersion`
- `sdkVersion`
- `networkType`

这些字段全部按 `best effort` 设计：

- 前端可不上送
- 服务端不能拒收
- 后台统一显示为 `unknown`

## 8. 指纹聚合规则

### 8.1 两层指纹

- `groupFingerprint`
  - 用于跨版本聚合同类问题
  - 不包含 `release`
- `releaseFingerprint`
  - 用于观察某一版本的爆发情况
  - 为 `groupFingerprint + release`

### 8.2 构成规则

- `uncaught_js` / `unhandled_promise`
  - `category + normalized_message + normalized_stack_top + module`
- `api_failure`
  - `category + method + normalized_path_template + status + normalized_error_message`
- `ws_failure`
  - `category + wsPhase + code + normalized_reason`
- `asset_failure`
  - `category + assetType + assetPath_or_manifest_code`
- `biz_error`
  - `category + bizCode`

### 8.3 归一化规则

生成指纹前需要去掉高噪声动态值：

- `roomId`
- `userId`
- UUID
- 时间戳
- 纯数字 ID
- token
- query string 中的动态参数

例如：

- `/api/orders/123/submit` 归一化为 `/api/orders/{id}/submit`

## 9. 存储设计

### 9.1 `report_project`

用途：项目注册表

主要字段：

- `id`
- `project_key`
- `project_name`
- `status`
- `anonymous_ingest_enabled`
- `created_at`
- `updated_at`

### 9.2 `error_issue`

用途：聚合后的问题主表

主要字段：

- `id`
- `project_id`
- `group_fingerprint`
- `category`
- `severity`
- `title`
- `normalized_message`
- `normalized_stack_top`
- `status`
- `first_seen_at`
- `last_seen_at`
- `total_count`
- `last_release`
- `last_language`
- `last_platform`
- `sample_event_id`
- `created_at`
- `updated_at`

唯一键：

- `project_id + group_fingerprint`

### 9.3 `error_event`

用途：原始事件明细

主要字段：

- `id`
- `event_id`
- `project_id`
- `issue_id`
- `release_fingerprint`
- `occurred_at`
- `release`
- `env`
- `category`
- `severity`
- `message`
- `stack`
- `route`
- `scene`
- `module`
- `language`
- `runtime`
- `user_id`
- `room_id`
- `session_id`
- `device_platform`
- `device_model`
- `os_version`
- `sdk_version`
- `network_type`
- `api_method`
- `api_path`
- `http_status`
- `ws_phase`
- `ws_code`
- `ws_reason`
- `asset_type`
- `asset_path`
- `asset_url`
- `biz_code`
- `extra_json`
- `created_at`

唯一键：

- `event_id`

### 9.4 `error_issue_release_daily`

用途：按 issue + release + 日期聚合的统计表

主要字段：

- `issue_id`
- `release`
- `stat_date`
- `event_count`
- `first_seen_at`
- `last_seen_at`

唯一键：

- `issue_id + release + stat_date`

### 9.5 保留策略

- `error_issue`：长期保留
- `error_issue_release_daily`：长期保留
- `error_event`：保留 30-90 天

原因：原始事件最占空间，但历史长期查询主要依赖 issue 和日统计。

## 10. 后台页面

### 10.1 登录页

- 用户名
- 密码
- 错误提示
- 登录限流

### 10.2 问题列表页

展示列：

- 项目
- 状态
- 分类
- 标题
- 首见时间
- 最近出现
- 总次数
- 近 24 小时次数
- 最新版本
- 最新语言
- 最新平台

筛选项：

- 项目
- 版本
- 分类
- 状态
- 语言
- 平台
- `bizCode`

### 10.3 问题详情页

- issue 基本信息
- 归一化 message
- 归一化 stack 顶部
- 最近样本事件
- 版本分布
- 语言分布
- 平台分布
- API / WS / 资源 / 业务码分布
- 近 24 小时 / 7 天趋势

### 10.4 状态流转

首版仅支持：

- `open`
- `resolved`
- `ignored`

## 11. 安全与防滥用

### 11.1 后台登录

首版使用静态配置账号：

- `ADMIN_USERNAME`
- `ADMIN_PASSWORD_HASH`
- `ADMIN_SESSION_SECRET`

密码使用 bcrypt hash 存储。  
登录成功后发服务端 session cookie，要求：

- `HttpOnly`
- `Secure`
- `SameSite=Lax`

### 11.2 上报防滥用

- 限制 body 大小
- 限制单批事件数量
- 限制 message / stack / extra 长度
- 匿名模式按 `IP + projectKey` 限流
- 登录页按 IP 限制失败次数
- 事件 `eventId` 去重
- 自动脱敏：
  - `Authorization`
  - `sessionToken`
  - URL 中的 token / code

## 12. 部署与运维

### 12.1 代码形态

首版推荐代码形态：

- Go 服务
- MySQL
- embed migrations
- 环境变量配置
- Docker 独立容器

### 12.2 推荐环境变量

- `DB_HOST`
- `DB_PORT`
- `DB_USER`
- `DB_PASSWORD`
- `DB_NAME`
- `HTTP_ADDR`
- `ADMIN_USERNAME`
- `ADMIN_PASSWORD_HASH`
- `ADMIN_SESSION_SECRET`

项目级共享密钥可放在：

- 数据库注册表扩展字段
- 或部署配置中心

首版推荐集中配置在服务端项目注册配置中，不向前端暴露。

### 12.3 与业务系统的关系

- 业务前端接入 `EventHub`
- 业务后端负责签发 `reportToken`
- `EventHub` 独立部署，不复用业务主进程
- 数据库可复用现有 MySQL 实例，也可单独部署

## 13. 验收标准

- 未捕获错误能在 30 秒内进入后台 issue 列表
- API 500 能正确聚合，不因动态路径拆成多条 issue
- WebSocket 握手失败和异常关闭能区分展示
- 资源清单失败与单资源失败能区分展示
- 同一 `bizCode` 的关键业务异常能稳定聚合
- 受限宿主环境缺少设备字段时服务端仍正常入库
- `EventHub` 故障不影响业务主流程
- 后台未登录不可访问列表与详情

## 14. 配套文档

为满足多产品接入需求，需同步维护：

- [EventHub 接入接口](./EventHub接入接口.md)

该文档面向其他产品前后端开发，定义项目注册、token 签发、事件协议、接口返回和接入步骤。

# EventHub

独立前端错误采集服务，统一接收、清洗、聚合、查询多产品前端错误，并提供简易后台管理页面。

## 功能

- 批量事件上报：`POST /reporting/v1/events/batch`
- 可信上报（`Authorization: Bearer <reportToken>`）
- 按指纹聚合 issue，支持版本/语言/平台等维度统计
- 后台登录、项目配置、问题列表、详情、状态流转（open / resolved / ignored）

详细设计见 [doc/EventHub设计.md](doc/EventHub设计.md)，接入说明见 [doc/EventHub接入接口.md](doc/EventHub接入接口.md)。

## 目录结构

```
src/                 Go 源码（cmd、internal、go.mod）
tests/               单元测试
script/              构建与发布脚本
.run/                运行时工作目录（不纳入版本库）
  config/            配置文件（.env）
  log/               运行日志
.temp/               编译中间文件
.dist/               本地发布产物
docker/              Docker 部署
doc/                 设计文档
aidoc/               AI 生成文档
0run.ps1             一键编译运行
```

## 快速开始

### 1. 初始化开发环境

```powershell
.\script\init_dev.ps1
```

会自动下载依赖，并在 `.run/config/.env` 生成开发配置（默认管理员密码 `admin123`）。

### 2. 编译并运行

```powershell
.\0run.ps1
```

支持参数：

| 参数 | 说明 |
|------|------|
| `--release` | 构建 Release 版本 |
| `--test` | 运行所有单元测试 |
| `--test-filter=NAME` | 运行指定测试 |

- 健康检查：http://localhost:8080/healthz
- 后台登录：http://localhost:8080/reporting/admin/login（默认用户 `admin`）
- 项目配置：http://localhost:8080/reporting/admin/projects
- 上报接口：http://localhost:8080/reporting/v1/events/batch

首启会自动创建 demo 项目（`projectKey=demo`）。

### 3. 生成管理员密码哈希

```powershell
.\.run\hashpassword.exe your-password
```

或：

```powershell
cd src; go run ./cmd/hashpassword your-password
```

### 4. Docker Compose 启动

```bash
export ADMIN_PASSWORD_HASH=$(cd src && go run ./cmd/hashpassword admin123)
cd docker/eventhub/eventhub
docker compose up --build
```

详见 [docker/readme.md](docker/readme.md)。

## 上报示例

上报需先由业务后端签发 `reportToken`，示例：

```bash
curl -X POST http://localhost:8080/reporting/v1/events/batch \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <reportToken>" \
  -d '{
    "clientSentAt": "2026-06-30T12:00:00Z",
    "events": [{
      "eventId": "6f4c8dbe-f6df-4b38-9a1c-7cdb22ef0001",
      "occurredAt": "2026-06-30T11:59:58Z",
      "release": "0.1.0",
      "env": "dev",
      "category": "uncaught_js",
      "severity": "error",
      "message": "Cannot read property x of undefined",
      "stack": "TypeError: Cannot read property x\n    at main (app.js:10:5)"
    }]
  }'
```

## 日志

运行日志输出到 `.run/log/`，格式：

```
[YYYY-MM-DD HH:MM:SS] [LEVEL] [FILE:LINE] MESSAGE
```

级别：DEBUG / INFO / WARN / ERR / FATAL。崩溃日志：`YYYY-MM-DD_crash.log`。

## 反向代理

推荐将 `/reporting/*` 反向代理到本服务，与业务 API 同域部署。

# EventHub 项目规则
服务部署文档：doc/服务部署.md

## 目录架构

遵循标准项目布局，源码在 `src/`，运行在 `.run/`，编译中间文件在 `.temp/`。

## 开发流程

1. `.\script\init_dev.ps1` — 初始化环境
2. `.\0run.ps1` — 编译并运行（Debug）
3. `.\0run.ps1 --test` — 运行单元测试
4. `.\script\publish.ps1` — 发布到 `.dist/`

## 技术栈

- Go 1.23 + chi + MySQL
- 模块路径：`github.com/eventhub/eventhub`

## 编码规范

- 日志使用 `internal/logger`，写入 `.run/log/`
- 配置从 `.run/config/.env` 加载
- 单元测试放在 `tests/`（外部测试）或 `src/` 内 `_test.go`
- 数据库名称：`eventhub`（与程序名一致）

## 禁止

- 不要在 `src/` 下生成 `bin/`、`obj/` 等编译产物
- 不要提交 `.run/`、`.temp/`、`.dist/` 到版本库

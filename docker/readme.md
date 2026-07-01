# EventHub Docker 部署

## 目录结构

```
docker/
  eventhub/
    Dockerfile
    eventhub/
      docker-compose.yml
      config/       配置文件
      data/log/     运行日志
```

## 启动

在项目根目录执行：

```bash
export ADMIN_PASSWORD_HASH=$(cd src && go run ./cmd/hashpassword admin123)
cd docker/eventhub/eventhub
docker compose up --build
```

- 健康检查：http://localhost:8080/healthz
- 后台：http://localhost:8080/reporting/admin/login

## 说明

- 数据库名称为 `eventhub`（与程序名一致）
- 日志挂载到 `data/log/`
- 配置可通过 `config/.env` 覆盖环境变量

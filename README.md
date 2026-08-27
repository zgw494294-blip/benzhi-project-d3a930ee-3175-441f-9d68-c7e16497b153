# 城市树木病害采样保全台

本项目为城市园林采样员提供树木病害样本的建档、采样批次登记、样本质检、补采整改、证据冻结和凭据验真闭环。服务使用 JSON HTTP API 和 SQLite 持久化，默认仅监听回环地址 `127.0.0.1:19081`。

## 构建、运行与测试

```text
go test ./...
go run ./cmd/biocuration-selfcheck
go run ./cmd/biocuration-api -addr=127.0.0.1:19081
```

也可以通过 `PORT` 环境变量设置端口号。生产 API 路由包括 `/v1/trees` 建档与 `PATCH /v1/trees/{treeID}` 基线更新、批次登记与质检、补采任务查询和单个/批量复核、冻结记录回读，以及支持 `history`、`from`、`to`、`limit` 查询参数的凭据验真端点；另提供 `/healthz`、`/selfcheck`。

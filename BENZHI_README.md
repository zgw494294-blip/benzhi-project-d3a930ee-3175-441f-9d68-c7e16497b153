# BENZHI_README

基于 Go 实现的城市树木病害采样保全台 HTTP API 项目，一款后端服务，园林采样员为城市树木建立病害样本保全凭据闭环。

## 项目说明
- 项目：benzhi-project-d3a930ee-3175-441f-9d68-c7e16497b153
- 项目用途：园林采样员为城市树木建立病害样本保全凭据闭环。
- Go 工具链：`golang:1.22`
- 前端工具链：无

## 标准构建、运行和测试命令
进入容器后执行：
cd '/app' && GOTOOLCHAIN=local go build ./...
cd /app && GOTOOLCHAIN=local go run ./cmd/biocuration-api -addr=127.0.0.1:19081
cd '/app' && GOTOOLCHAIN=local go test ./...

## Docker 构建和进入容器
chmod +x build_benzhi_docker.sh
./build_benzhi_docker.sh benzhi-project-d3a930ee-3175-441f-9d68-c7e16497b153-amd64 linux/amd64
./build_benzhi_docker.sh benzhi-project-d3a930ee-3175-441f-9d68-c7e16497b153-arm64 linux/arm64
docker run -it benzhi-project-d3a930ee-3175-441f-9d68-c7e16497b153-amd64:latest

## 题目验证命令
1. 预期退出码 0：`go test ./...`
2. 预期退出码 0：`go run ./cmd/biocuration-selfcheck`

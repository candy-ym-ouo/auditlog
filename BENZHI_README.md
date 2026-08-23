# AuditChain Guard 交付说明

AuditChain Guard 是一个以 SQLite 持久化的防篡改审计日志服务。它通过 SHA-256 哈希链保存审计记录，提供日志写入、查询、完整性验证、归档、追溯与内嵌的 Web 页面。

## 本地验证

要求：Go 1.21 或更高版本。

```bash
go mod download
go vet ./...
go test ./...
go build ./...
go run ./cmd/server -db ./audit.db -addr 127.0.0.1:8080
```

服务启动后访问 `http://127.0.0.1:8080`。默认 SQLite 数据库为 `audit.db`；本地数据库和构建产物均已被 `.gitignore` 排除。

## Docker 打包与验证

镜像保留完整 Go 工具链，并在构建阶段执行 `go mod download` 和 `go build ./...`。默认构建目标是 `linux/amd64`：

```bash
chmod +x build_benzhi_docker.sh
./build_benzhi_docker.sh auditlog linux/amd64
./build_benzhi_docker.sh auditlog linux/arm64
docker run -it auditlog:latest
```

进入容器后可执行：

```bash
go build ./...
go test ./...
go run ./cmd/server -db /tmp/audit.db -addr 127.0.0.1:8080
```

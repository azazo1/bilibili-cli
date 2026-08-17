[private]
default:
    @just --list

# 构建 CLI 二进制.
# just build
build:
    go run ./scripts/release -mode build

# 生成当前平台的发布归档.
dist:
    go run ./scripts/release -mode dist

# 运行全部 Go 测试.
test:
    go test ./...

# 直接运行 CLI.
# just run hot --max 5
run *args:
    go run ./cmd/bili {{args}}

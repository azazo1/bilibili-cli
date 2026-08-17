[private]
default:
    @just --list

# 构建 CLI 二进制.
# just build
build:
    go build -o bin/bili ./cmd/bili

# 运行全部 Go 测试.
test:
    go test ./...

# 直接运行 CLI.
# just run hot --max 5
run *args:
    go run ./cmd/bili {{args}}


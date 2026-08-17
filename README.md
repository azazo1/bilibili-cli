# bilibili-cli-go

一个 Go 实现的 Bilibili 终端客户端. 项目覆盖上游 Python CLI 的账户, 视频, 搜索, 发现, 收藏, 动态, 互动和音频工作流.

## 功能

- 视频详情, 字幕, AI 总结, 评论和相关推荐.
- UP 主资料, 视频列表, 用户和视频搜索.
- 热门视频, 排行榜, 收藏夹, 关注, 历史和稍后再看.
- 动态时间线, 我的动态, 文字动态发布和删除.
- 点赞, 投币, 一键三连和取消关注.
- 保存凭证, QR 登录, JSON/YAML 稳定 envelope 输出.
- 音频下载和 16 kHz 单声道 WAV 分段. 非 PCM WAV 输入通过系统 `ffmpeg` 转码.

`references/bilibili-cli` 保存上游代码作为本地迁移参考, 已被 Git 忽略.

## 构建

```shell
go build -o bin/bili ./cmd/bili
./bin/bili --help
```

也可以直接运行:

```shell
go run ./cmd/bili hot --max 5 --yaml
```

## 登录

```shell
go run ./cmd/bili login
go run ./cmd/bili status --yaml
```

配置和认证默认保存在 `~/.config/bilibili-cli/config.toml` 与 `~/.config/bilibili-cli/auth.json`. 如需从已有 cookie 导入, 可以传入 `BILI_COOKIE` 或 Netscape cookie 文件路径 `BILI_COOKIE_FILE`.

## 配置

使用 `bili config init` 创建默认 `config.toml`. 根目录的 [`config.toml.example`](./config.toml.example) 可作为手动创建配置的模板.

```toml
version = 1

[output]
format = "auto"

[network]
timeout_seconds = 30

[safety]
read_only = false
confirm_dangerous_actions = true
```

`output.format` 支持 `auto`, `rich`, `json`, `yaml`. `OUTPUT` 环境变量会覆盖这个值.

`safety.read_only = true` 会拒绝动态发布和删除, 点赞, 投币, 一键三连, 取关等账户侧写操作. `login` 和 `logout` 仍然可用. `confirm_dangerous_actions` 控制删除动态和取关是否需要额外确认.

## 输出

查询命令支持 `--json` 和 `--yaml`. 非交互 stdout 默认输出 YAML. 成功和失败都使用统一结构:

```yaml
ok: true
schema_version: "1"
data: {}
```

## 开发

```shell
go test ./...
just test
```

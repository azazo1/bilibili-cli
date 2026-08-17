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

凭证默认保存在 `~/.bilibili-cli/credential.json`. 如需从已有 cookie 导入, 可以传入 `BILI_COOKIE` 或 Netscape cookie 文件路径 `BILI_COOKIE_FILE`.

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


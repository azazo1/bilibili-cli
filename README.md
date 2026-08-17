# bilibili-cli-go

一个 Go 实现的 Bilibili 终端客户端. 项目覆盖上游 Python CLI 的账户, 视频, 搜索, 发现, 收藏, 动态, 互动和媒体下载工作流.

## 功能

- 视频详情, AI 总结, 评论和相关推荐.
- 字幕轨道列出与导出, 覆盖普通字幕和平台标记的 AI 字幕.
- UP 主资料, 视频列表, 专栏, 视频, 用户, 番剧, 直播和影视搜索.
- 热门视频, 排行榜, 收藏夹, 关注, 历史和稍后再看.
- 动态时间线, 我的动态, 文字动态发布和删除.
- 点赞, 投币, 一键三连和取消关注.
- 保存凭证, QR 登录, JSON/YAML 稳定 envelope 输出.
- 视频音频流和视频流下载, 支持仅下载音频或视频.

`references/bilibili-cli` 保存上游代码作为本地迁移参考, 已被 Git 忽略.

## 构建

```shell
go build -o bin/bili ./cmd/bili
./bin/bili --help
```

也可以直接运行:

```shell
go run ./cmd/bili video hot --max 5 --yaml
```

## 登录

```shell
go run ./cmd/bili me login
go run ./cmd/bili me --yaml
```

扫码登录会同时保存 Web Cookie, App access token 和 refresh token. 升级到此版本后, 请重新执行一次 `bili me login` 以启用 App 请求路径.

## 字幕

```shell
go run ./cmd/bili video subtitle BV1ABcsztEcY
go run ./cmd/bili video subtitle BV1ABcsztEcY --language zh_CN --type non-ai
go run ./cmd/bili video st BV1ABcsztEcY -o ./subtitles
go run ./cmd/bili video st "https://www.bilibili.com/video/BV1ABcsztEcY?p=2" -o ./downloads
```

`bili video subtitle` 会列出播放器接口返回的所有字幕轨道, 包含 ID, 语言, 字幕类型, AI 标记, 作者和字幕行数. 可通过 `--id`, `--language` 或 `--type all|ai|non-ai` 筛选轨道. `--language` 使用 API 的 `lan`, 并兼容 `zh-CN` 与 `zh_CN` 两种写法. `-o` 始终指定输出目录, 字幕会写为 `<视频基名>.<语言>.srt`, 与下载视频共享基名以便播放器自动识别. 多分P视频需通过 URL 的 `?p=N` 选择对应字幕.

## 视频下载

```shell
bili video download BV1ABcsztEcY
bili video download BV1ABcsztEcY -o ./downloads
bili video download "https://www.bilibili.com/video/BV1ABcsztEcY?p=2"
bili video download BV1ABcsztEcY --audio-only
bili video download BV1ABcsztEcY --video-only
bili video download BV1ABcsztEcY --no-merge
bili video download BV1ABcsztEcY --with-srt
```

`bili video download` 默认保存到当前文件夹. DASH 地址会分别保存 `<标题>_audio.m4a` 和 `<标题>_video.m4a`, 然后尝试使用 `ffmpeg -c copy` 生成 `<标题>.mp4`. `--no-merge` 可保留两个流而不合并. 原生 `durl` 地址会直接保存一个 `.mp4`.

多分P视频必须在 URL 中指定 `?p=N`. 未指定时会列出所有分P及其标题, 不会下载第一P. 文件名会包含 `P` 序号和该分P的标题. `--with-srt` 会尝试选择并保存一条字幕, 优先中文, 同语言优先人工字幕, 其次 AI, 再按英文和其他语言兜底. 交互终端会显示下载进度条. `--audio-only` 和 `--video-only` 不能同时使用.

其他命令按领域组织在 `bili me`, `bili user`, `bili video` 和 `bili dynamic` 下. 例如 `bili user video UID_OR_NAME`, `bili user follow UID`, `bili me fav`, `bili video watch` 和 `bili dynamic post TEXT`.

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

`safety.read_only = true` 会拒绝动态发布和删除, 点赞, 投币, 一键三连, 取关等账户侧写操作. `me login` 和 `me logout` 仍然可用. `confirm_dangerous_actions` 控制删除动态和取关是否需要额外确认.

视频信息和媒体下载属于读取操作, 在只读模式下仍然可用.

## 输出

查询命令支持 `--json` 和 `--yaml`. 非交互 stdout 的表格默认输出 TSV, 其他查询命令默认输出 YAML. 成功和失败都使用统一结构:

```yaml
ok: true
schema_version: "1"
data: {}
```

## 搜索

```shell
bili search "关键词" --type video --order totalrank
bili search "关键词" --type article --order pubdate
bili search "关键词" --type bangumi --order click
```

`--type` 支持 `all`, `article`, `video`, `user`, `bangumi`, `live`, `media`.
省略 `--type` 时使用综合搜索, 也可以显式指定 `all`.
`--order` 支持 `totalrank`, `click`, `pubdate`, `dm`, `stow`.
表格在 PTY 中会按终端宽度对齐并截断可变长列. `--no-truncate` 可关闭截断. 非 PTY 的表格输出为未截断 TSV.

## 开发

```shell
go test ./...
just test
```

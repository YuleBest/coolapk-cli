# coolapk

酷安的只读命令行工具，支持搜索动态、用户和应用，查看动态正文与评论区，查看用户主页资料和动态列表。

## 安装

从 Releases 下载对应平台的压缩包，解压后把可执行文件放进 `PATH`：

```sh
tar -xzf coolapk_0.1.0_linux_amd64.tar.gz
install -m755 coolapk ~/.local/bin/coolapk
```

也可以从源码构建，需要 Go 1.24 或更高版本：

```sh
go build -trimpath -ldflags "-s -w -X main.version=0.1.0" -o ~/.local/bin/coolapk ./cmd/coolapk
```

## 用法

```sh
coolapk search --feed "小米13"          # 搜索动态
coolapk search --user wherewhere        # 搜索用户
coolapk search --apk 微信               # 搜索应用
coolapk feed 74360444                   # 动态正文加评论第一页
coolapk feed 74360444 --no-comments     # 只看正文
coolapk user 536381                     # 用户资料加动态列表
```

常用参数：

- `--page N` 翻页，`--limit N` 限制当前页显示条数，服务端每页约 20 条。
- `feed` 命令用 `--comment-page` 和 `--comment-limit` 控制评论区。
- `--json` 输出接口原始数据，方便接 `jq` 一类的工具。
- `--no-color` 关闭颜色，设置 `NO_COLOR` 环境变量同样有效。

## 配置

配置文件位于 `~/.config/coolapk/config.json`，首次运行会自动生成设备码：

```json
{
  "device": "自动生成的设备码",
  "cookie": ""
}
```

`cookie` 可以填已登录客户端的 Cookie，环境变量 `COOLAPK_DEVICE` 和 `COOLAPK_COOKIE` 优先级更高。

## 已知限制

- 只读工具，不提供登录、点赞、发帖等写操作。
- 查看动态正文的移动端接口偶尔触发酷安风控，需要人机验证，与登录态无关。CLI 会自动改走网页版 `www.coolapk.com/feed/<id>` 取同构 JSON，正常仍能看到正文；只有两条路径都失败时才会报错退出。
- 数据来自酷安官方接口，仅供学习与个人使用。

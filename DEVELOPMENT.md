# 开发

## 环境

Go 1.24 或更高版本，无需 cgo，也不需要额外的系统依赖。

## 常用命令

```sh
go build ./...        # 编译
go vet ./...          # 静态检查
go test ./...         # 单元测试，不访问网络
go build -trimpath -ldflags "-s -w -X main.version=0.1.0" -o ~/.local/bin/coolapk ./cmd/coolapk
```

## 联网冒烟测试

默认测试全部离线。需要验证线上接口时：

```sh
go test -tags=live ./internal/api/ -run TestLiveEndpoints -v
```

该测试真实调用酷安只读接口，返回风控或要求登录会被记为跳过而不是失败。可用 `COOLAPK_DEVICE`、`COOLAPK_COOKIE` 环境变量指定访问身份。

## 代码结构

- `cmd/coolapk`：命令行入口、子命令与错误提示
- `internal/auth`：设备码与 X-App-Token v3
- `internal/api`：请求头、响应信封、端点和数据类型
- `internal/config`：配置文件与环境变量
- `internal/render`：终端文本渲染

## 注意事项

- `/v6/feed/detail` 可能被风控拦截并返回 `err_request_captcha_v2`。2026-10-15 实测该拦截与登录态无关：带有效 Cookie、换设备码、换 api2、加 formApi/fromApi、改用 cost=10 的 token 都照样返回验证码。
- 正文的应对方式参考 coolapk-desktop：先走移动端接口，遇到验证码时改用网页版 `https://www.coolapk.com/feed/<id>`，同时带上 `X-Requested-With: XMLHttpRequest` 与 `X-App-Id: com.coolapk.market`，网页版会直接返回同构 JSON。兜底逻辑在 `api.Client.FeedDetail`，相关断言见 `internal/api` 的 `TestFeedDetailFallsBackToWeb`。
- 上游桌面端还有一条更重的路子：跑网易易盾滑块验证码，拿到 `validate` 后拼成 `NEC:<captchaId 前 8 位>:<validate>` 作为 `_v2_post_token` 重试。本仓库当前不做验证码，需要时再补。
- X-App-Token 依赖内嵌的 `auth_blob` 与 bcrypt cost=4，改动签名逻辑前先跑 `internal/auth` 的固定向量测试。
- 输出格式的改动要同步 `internal/render` 的断言，避免文本回归。
## 发版

推 tag 即可触发 `.github/workflows/release.yml`：

```sh
git tag v0.1.0
git push origin v0.1.0
```

工作流先跑 `go vet` 与 `go test`，再交叉编译 linux、darwin、windows 的 amd64 与 arm64，打包成 tar.gz 或 zip，连同 `checksums.txt` 一起挂到 GitHub Release。版本号来自 tag，通过 `-ldflags "-X main.version=<版本>"` 注入，不需要手工改代码。

本地发版前建议先跑一遍 `go vet ./... && go test ./...`，并按上面的命令本地交叉编译一次确认能过。

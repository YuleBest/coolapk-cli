# 项目信息

## 这是什么

酷安的只读命令行工具，二进制名 `coolapk`。可以搜索动态、用户和应用，查看动态正文与评论区，查看用户主页资料和动态列表。

## 技术栈

- 语言：Go 1.24，纯 Go，不需要 cgo
- 包管理：Go modules
- 命令行框架：spf13/cobra
- 关键依赖：golang.org/x/crypto 的 blowfish 用于 bcrypt 签名，golang.org/x/net/html 用于正文转文本
- 接口来源：酷安移动端 V6 只读接口，字段说明与实测记录来自 coolapk-api-collect 文档项目

## 要做的

- 匿名只读访问，X-App-Token 在本地生成
- 正文接口被风控拦住时自动改走网页版接口
- 默认输出适合终端阅读的文本，`--json` 输出接口原始数据
- 配置只保存设备码与可选 Cookie

## 不要做的

- 不实现登录、验证码、点赞、发帖、关注、私信等写操作
- 不做交互式界面，不做自动全量翻页
- 不内置代理参数，需要时用 `HTTPS_PROXY` 环境变量

## 第三方来源

- `internal/auth/auth_blob.b64` 与 token 生成算法参考 MIT 许可的 coolapk-desktop
- 正文接口被风控时的网页版兜底思路同样来自 coolapk-desktop

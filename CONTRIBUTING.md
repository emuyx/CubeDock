# 参与开发

欢迎报告可复现问题、补充文档和改进实现。大范围改动请先说明目标、支持的上游
版本和验证方式。

## 环境

- Go 1.25+（`go.mod` 要求 1.25，工具链固定 1.26.9）；
- Node.js/npm（仅修改 `webui-src` 需要）；
- `vendor-src/nemc-tan-lobby-solver` 与 `vendor-src/g79client` 必须保留；
- 本机代理可按需设置 `GOPROXY=https://goproxy.cn,direct`。

```bash
go test ./...
go vet ./...

cd webui-src
npm install
npm run build

cd ..
.\scripts\build.ps1   # Windows：launcher + 主程序 + limbo
```

非 Windows 可手动构建 `build/bin/CubeDockMain`、`build/bin/limbo`，再按
README 的 launcher 步骤打包。

不要随意 `go mod tidy`：vendored 房间模块依赖 gophertunnel 1.51.x 与 Pion v3/v4，
升级会破坏 DTLS/transport 类型。

## 代码结构

| 目录 | 职责 |
| --- | --- |
| `cmd/cubedock` | 参数、启动、进程生命周期 |
| `internal/core` | 房间生命周期、账号、事件、转发编排 |
| `internal/server` | HTTP 控制台与管理员会话 |
| `internal/console` | 终端横幅、LAN 地址、Host 白名单 |
| `internal/auth`、`internal/funauth` | 互通版认证与房间凭据 |
| `internal/interoplogin` | a50/x19 邮箱登录链路 |
| `internal/room` | TanLobby 控制协议、WebRTC 信令 |
| `internal/bridge` | 登录重签、加密握手、游戏包加解密、Ping |
| `internal/relay`、`internal/gamebatch` | NetherNet/RakNet 转发与批次边界 |
| `internal/friends` | 好友申请自动同意 |
| `internal/config`、`internal/secret`、`internal/session`、`internal/fileutil` | 配置、凭据、账号互斥、原子写 |
| `internal/webui`、`webui-src` | 内嵌 React/MUI 控制台 |

`internal/` 下的胶水代码是在 CubeDock 独立化时重写的，改动时请保持与
NEMCBridge 来源的边界清晰；新增原创文件使用 MIT。

## 测试

- 协议/解析改动必须有往返或 fixture 测试；
- 加密中继改动必须覆盖登录重签、身份字段保留、游戏包加解密与篡改检测；
- 真实上游测试使用自己的支持网易协议的服务端和测试账号，不要把会话写入仓库；
- 前端改动后重新 `npm run build`，再跑 Go 构建。

## 不提交

令牌、`room-tokens.dat`、`accounts.json`、`admin.json`、`interop-device.dat`、
日志、世界、玩家身份、个人路径、私有服务凭据或抓包原文。测试 fixture 必须脱敏。

## 许可

- 原创代码按 MIT 提供（见 `LICENSE`）；
- `vendor-src/g79client` 与 `internal/funauth` 为 AGPL-3.0；
- `nemc-tan-lobby-solver` 原上游仓库已被作者删除，根目录没有许可证；公开分发前
  需取得明确许可或替换该模块；
- 提交前确认没有把 AGPL 或无许可证代码误标为 MIT。

## 上游项目

不要修改外部上游服务端的源码；需要上游配合时，提交提示词或补丁建议，由上游
维护者自行应用。

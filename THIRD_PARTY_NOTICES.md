# 第三方组件与归属

根目录 [MIT License](LICENSE) 覆盖 CubeDock 原创代码和文档。它不重新授权
第三方组件、Go 模块依赖、第三方商标或游戏资源。

## 认证链路

CubeDock 的网易认证链（`internal/netease`、`internal/auth`、
`internal/interoplogin`）是 CubeDock 自有实现，使用 MIT 许可。

历史上曾使用的 `vendor-src/g79client` 与 `internal/funauth`（AGPL-3.0）已在
2026-11-11 移除；当前源码和发布包不再包含 AGPL 组件，也不再附带 `SOURCE/`。

## nemc-tan-lobby-solver

`vendor-src/nemc-tan-lobby-solver` 以本地 `replace` 使用，模块路径
`github.com/Happy2018new/nemc-tan-lobby-solver`。它提供 NetherNet、WebRTC、
RakNet 和房间协议实现。

原上游仓库已被作者删除，原模块根目录和 `bunker/` 没有许可证。CubeDock 已做
以下清理：

- 删除未使用的根目录 `main.go` 和 `debug/` demo；
- 将原先无许可证的 `bunker/` 重写为 CubeDock 自有的 MIT Phoenix TanLobby
  HTTP 客户端；
- 保留的 `minecraft`、`protocol`、`core/nethernet`、`core/raknet`、
  `core/webrtc`、`core/datachannel`、`core/sctp` 子目录各自带有 MIT LICENSE，
  按对应目录条款使用；
- 本地胶水和新增文件的许可见该目录的 `LICENSE` / `NOTICE`。

升级该目录时仍需逐个核对新增文件的许可证。

## Go 依赖

依赖版本由 `go.mod` / `go.sum` 固定，主要包括：

- `github.com/sandertv/gophertunnel`（MIT；`limbo/` 使用 1.53.0 的本地副本
  `limbo/third_party/gophertunnel`，仅含网易资源包流程补丁，见 `PATCHES.md`）
- `limbo/` 通过本地 replace 复用 `vendor-src/nemc-tan-lobby-solver/core/raknet`
  实现 NetEase RakNet v8 拉包。
- `github.com/sandertv/go-raknet`（MIT）
- Pion 系列（MIT）
- `github.com/coder/websocket`、`github.com/df-mc/jsonc`、`github.com/ugorji/go/codec`、
  `github.com/go-gl/mathgl`、`github.com/andreburgaud/crypt2go`、
  `github.com/database64128/chacha8-go`（MIT）
- `github.com/klauspost/compress`、`github.com/google/uuid`（BSD-3-Clause）
- `golang.org/x/crypto`、`golang.org/x/net`、`golang.org/x/sys`、`golang.org/x/term`
  （BSD-3-Clause）
- `github.com/go-jose/go-jose/v4`（Apache-2.0，limbo 使用）
- 其他间接依赖以各自模块的 LICENSE 为准。

## 前端依赖

`webui-src/` 使用 React、Material UI、Emotion 和 Vite。它们均为 MIT 许可，
构建产物内嵌进 Go 二进制，不依赖外部 CDN。`webui-src/node_modules` 不进仓库。

## 游戏与商标

Minecraft、Mojang、Microsoft、网易及其相关名称和商标归各自权利人所有。
CubeDock 不代表或隶属于任何权利人，也不附带游戏客户端、服务端或官方资源。
使用网易账号与本地联机功能可能受网易服务条款约束，风险由使用者承担。

## 许可义务摘要

1. 自用、不对外分发：保持仓库内许可文件完整即可；
2. 分发 CubeDock 二进制：随包提供 `LICENSE`、`NOTICE`、本文件与 `licenses/`
   目录（含 MIT/BSD/Apache 文本和 vendored 组件的 LICENSE 副本）；
3. 当前发布包不再包含 AGPL 代码或 `SOURCE/`，无需 AGPL 源码提供义务；
4. 公开分发前仍建议对 `nemc-tan-lobby-solver` 的来源与许可做法律复核。

以上仅为工程记录，不构成法律意见；公开分发前请咨询专业人士。

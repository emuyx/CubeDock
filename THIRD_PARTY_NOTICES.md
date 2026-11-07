# 第三方组件与归属

根目录 [MIT License](LICENSE) 覆盖 CubeDock 原创代码和文档。它不重新授权
`vendor-src/` 中的上游代码、Go 模块依赖、第三方商标或游戏资源。

## AGPL-3.0 组件

| 位置 | 来源 | 许可 |
| --- | --- | --- |
| `vendor-src/g79client` | `UCKETX/g79client`（模块路径 `github.com/Yeah114/g79client`），本地修改过互通版认证与后台刷新 | AGPL-3.0 |
| `internal/funauth` | 从 FunAuth 的认证包提取 `TanLobbyCreate` / `TanLobbyLogin`，并做本地化修改 | AGPL-3.0 |

这两部分是网易账号认证链路的核心。若分发包含它们的二进制或源码，或通过网络
向他人提供服务，必须遵守 AGPL-3.0：提供对应源码、保留许可声明，并告知用户其
权利。根目录 MIT 声明不改变这些部分的许可证。

`internal/interoplogin` 是 CubeDock 自己实现的互通版邮箱登录链路，但它依赖
`g79client` 的协议常量与工具；对外分发时仍应把整个认证链路按 AGPL 义务评估。

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

现在构建不再包含原无许可证代码；升级该目录时仍需逐个核对新增文件。自写/
第三方代码的完整归属见 [docs/ownership.md](docs/ownership.md)。

## Go 依赖

依赖版本由 `go.mod` / `go.sum` 固定，主要包括：

- `github.com/sandertv/gophertunnel`（MIT；根模块 1.51.1；`limbo/` 使用 1.53.0
  的本地副本 `limbo/third_party/gophertunnel`，仅含网易资源包流程补丁，见
  `PATCHES.md`）
- `limbo/` 通过本地 replace 复用 `vendor-src/nemc-tan-lobby-solver/core/raknet`
  实现 NetEase RakNet v8 拉包；该模块根目录无许可证的问题同样适用于 limbo。
- `github.com/sandertv/go-raknet`（MIT）
- Pion 系列（MIT）
- `golang.org/x/crypto`、`golang.org/x/net`、`golang.org/x/sys`（BSD-3-Clause）
- `github.com/google/uuid`（BSD-3-Clause）
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
2. 分发 CubeDock 二进制：必须一并提供 AGPL 组件的对应源码（或按 AGPL
   要求提供获取方式），并保留 `NOTICE`、本文件与各上游许可证；
3. 对外提供网络服务：AGPL 同样要求向服务使用者提供对应源码；
4. 公开分发前，先取得 `nemc-tan-lobby-solver` 的明确许可或替换该模块。

以上仅为工程记录，不构成法律意见；公开分发前请咨询专业人士。

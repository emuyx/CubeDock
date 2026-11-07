# CubeDock limbo

转服模式的内置 limbo 辅助进程。转服模式已开放，可直接在转服与资源包页选择，
详见 `docs/transfer-limbo.md`。

独立 Go module，使用 gophertunnel `v1.53.0`（Bedrock protocol 898 / 1.21.130）
的本地副本 `third_party/gophertunnel`，其中只有网易资源包流程补丁；改动见
`third_party/gophertunnel/PATCHES.md`。

```bash
cd limbo
go build -trimpath -o ../build/bin/limbo.exe .
```

启动参数：

- `-listen 127.0.0.1:19133`：标准 RakNet 监听地址；
- `-target host:port`：玩家点击表单按钮后发送 `Transfer` 的目标；
- `-watch-parent=true`：stdin 关闭时退出，由 CubeDock 自动传入；
- `-packs <目录>`：资源包目录，支持 `.mcpack`/`.zip` 或包文件夹；留空时依次查找
  limbo 自身目录和当前目录下的 `packs/`；
- `-packs-required=false`：在 ResourcePacksInfo 中标记资源包为必需；网易客户端
  会忽略该标记，下载由服务端强制触发；
- `-packs-mode netease|standard`：`netease`（默认）启用强制下载 workaround；
  `standard` 走普通 ResourcePacksInfo，供本地标准客户端测试；
- `-packs-settle=3s`：资源包就绪后、显示表单前的等待时间；
- `-form-title` / `-form-content` / `-form-button`：SimpleForm 的标题、正文和
  按钮文字，由 CubeDock 转服与资源包页的 `limbo_form_*` 传入；
- `-pull`：以假网易机器人登录 `-target` 后端服，下载未加密资源包后退出；
- `-pull-network netease-v8|raknet`：拉包传输层，默认 `netease-v8`；`raknet`
  只用于本地测试；
- `-identity-name` / `-identity-uid`：拉包机器人的显示名和 NetEase UID。

行为：

1. 加载资源包并交给 `ListenConfig.ResourcePacks`；
2. `StartGame`，客户端完成资源包下载/应用；
3. Geyser 式空 `LevelChunk`；
4. 出生点下方 `UpdateBlock` 基岩；
5. 若有资源包，等待 `-packs-settle`；
6. 发送 `ModalFormRequest`（SimpleForm，标题/正文/按钮文字可配置）；
7. 玩家点击按钮后发送 `Transfer`；关闭表单不会转服。

资源包必须与目标服务器实际下发的包一致（UUID、版本、SHA-256），否则转服后仍
可能报资源包错误。当前实现不设置 ContentKey，只适用于未加密包；动态资源包
不适用。

CubeDock 在 `connection_mode=transfer` 时自动拉起该进程，并通过 Job Object
（Windows）和 stdin 管道保证主程序退出后不残留。

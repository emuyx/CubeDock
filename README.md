# CubeDock 方块码头

CubeDock 让网易版 Bedrock 玩家通过网易「本地联机」房间加入任何原生支持网易
协议的 Minecraft 基岩版服务器，不绑定特定上游实现。项目独立维护，保留第三方
房间/认证底座。

> 项目主页（GitHub Pages）：https://emuyx.github.io/CubeDock/

```
网易客户端
  │ NetherNet / WebRTC（本地联机房间，房主机器人）
  ▼
CubeDock
  │ 网易 RakNet v8 + 原始游戏包直通 + 代做 Bedrock 加密握手
  ▼
支持网易协议的 Bedrock 服务器（任何兼容实现）
```

> 上游可以是**任何原生支持网易 RakNet v8 协议的 Bedrock 服务器**，不绑定
> 特定服务端实现。标准 BDS 不能直接作为 CubeDock 的上游；游戏包不再翻译，
> 只做登录重签与加密握手。

## 能力

- **互通版邮箱直登**：用网易邮箱和密码在内嵌链路中获取互通版 sauth，无需抓包；
- **互通版会话导入**：也可以粘贴 `sauth_json` 手动导入；
- **多账号 / 多房间**：最多 8 个网易账号，每个账号一个本地联机房间；
- **加密中继**：为每个连接生成 P-384 密钥，重签单段登录链，保留
  `displayName`、`netease_uid`、`netease_uuid` 等身份字段，代替客户端完成
  `ClientToServerHandshake`；
- **连接模式**：代理模式（默认，CubeDock 中继）或转服模式（limbo + Transfer）。
  转服模式会先让玩家进入 limbo 并预加载目标服务器资源包，点击表单按钮后直连
  目标服务器；资源包需与目标服务器一致，当前只支持未加密包；
- **固定直通**：不改写 StartGame、玩家身份、皮肤或资源包；
- **自动同意好友**：房间在线时定时检查好友申请；
- **Web 控制台**：React + Material UI，侧边导航（主页/账号与房间/转服与资源包/
  控制台/设置）、管理员会话、账号表、日志、`help`/`list`/`kick` 交互式控制台、
  `transfer` 转服 POC、1351 安全验证链接与自动重试；
- **凭据保护**：Windows 使用 DPAPI；其他平台使用 0600 文件；登录接口只允许本机。

## 快速开始

### 1. 准备上游

准备一台原生支持网易 RakNet v8 协议的 Bedrock 服务器；自建或第三方实现均可，
CubeDock 不绑定具体服务端。若上游提供单段登录链、转发地址信任等配置，请按其
文档启用并信任 CubeDock 的地址。CubeDock 不修改上游服务端源码。

### 2. 构建或运行

```powershell
# Windows：构建 launcher、主程序与 limbo
.\scripts\build.ps1

# 运行（launcher 首次启动会把 CubeDockMain.exe / limbo.exe 释放到 bin 子目录；
# 数据默认仍在 launcher 所在目录）
.\build\CubeDock.exe
```

Linux/macOS 手动构建：

```bash
mkdir -p build/bin
go build -trimpath -o build/bin/CubeDockMain ./cmd/cubedock
(cd limbo && go build -trimpath -o ../build/bin/limbo .)
mkdir -p cmd/cubedock-launcher/payload
cp build/bin/CubeDockMain cmd/cubedock-launcher/payload/
cp build/bin/limbo cmd/cubedock-launcher/payload/
go build -trimpath -o build/CubeDock ./cmd/cubedock-launcher
rm -f cmd/cubedock-launcher/payload/CubeDockMain cmd/cubedock-launcher/payload/limbo
```

每次启动 launcher 都会校验内置主程序/limbo 的 SHA-256，缺失或变化时覆盖后再
启动；可用 `-install-dir <目录>` 指定 bin 释放目录。

打开终端显示的控制台地址（默认 `http://127.0.0.1:18321/`），管理员账号
`admin`，初始密码在首次启动时打印一次。忘记密码可用
`-reset-admin-password`。

### 3. 添加账号并开房

1. 控制台登录后先用「互通版邮箱直登」（推荐）或粘贴 `sauth_json`；
2. 确认账号表显示游戏名和会话到期日；
3. 在设置页填写上游地址；
4. 点击「启动房间」。网易客户端在「本地联机」中搜索房间号即可加入。

## 数据文件

通过 launcher 启动时，数据目录默认是 launcher 所在目录（不是 `bin/`）；
直接运行 `CubeDockMain.exe` 时默认是其所在目录。可用 `-data-dir` 指定：

| 文件 | 内容 |
| --- | --- |
| `config.json` | 非敏感房间/上游配置 |
| `room-tokens.dat` | 网易账号会话（DPAPI/0600） |
| `accounts.json` | 邮箱/标签、游戏名、UID、等级、平台、到期时间、解析错误（无会话） |
| `interop-device.dat` | 互通版直登的 a50/x19 设备状态（DPAPI/0600） |
| `admin.json` | 管理员 bcrypt 哈希 |
| `logs/cubedock.log` | 运行日志（含来源） |
| `logs/status.json` | 最近一次状态快照（不含房间密码和事件） |

当前不迁移旧版数据目录。

## 配置

```json
{
  "connection_mode": "proxy",
  "limbo_upstream": "127.0.0.1:19133",
  "limbo_packs_dir": "",
  "limbo_auto_pull_packs": true,
  "limbo_form_title": "CubeDock Limbo",
  "limbo_form_content": "点击下方按钮进入服务器。",
  "limbo_form_button": "进入服务器",
  "lobby_protocol_id": 43,
  "upstream": "127.0.0.1:19132",
  "room_name": "来和我一起玩吧！",
  "room_game_version": "1.21.130.0",
  "room_password": "",
  "max_players": 10,
  "friends_only": false,
  "auto_accept_friends": true,
  "allow_pvp": true,
  "auto_start": false,
  "web_port": 18321,
  "retry_seconds": 10,
  "debug": false
}
```

当前适配网易互通版 3.10（引擎/补丁 `3.10.5.301486`，基岩版 `1.21.130`）；
旧配置中的 `1.21.120.0` 会在加载时自动迁移为 `1.21.130.0`，
旧房间协议 ID `42` 会自动迁移为 `43`。
`connection_mode=transfer` 使用内置 limbo；默认仍为 `proxy`。转服模式会
自动创建资源包目录（默认与 `limbo.exe` 同级的 `packs/`），启动终端也会打印
该路径。`limbo_packs_dir` 可覆盖目录，留空时自动查找 `packs/`。
`limbo_auto_pull_packs`（默认开启）会在开房前用假网易机器人登录「本地
Bedrock 地址」拉取未加密资源包；本地相同 UUID+版本的包不会重复下载，拉取完
会发送 RakNet 断开包；重复包会自动删除。设置页也可手动触发拉取。

没有 `auth_mode`、`auth_server` 或手游引擎版本字段：CubeDock 只使用内嵌互通版
认证。

## 命令行

| 参数 | 说明 |
| --- | --- |
| `-data-dir` | 配置/日志目录；launcher 默认 launcher 目录，主程序默认自身目录 |
| `-install-dir` | launcher 专用：bin 释放目录 |
| `-web-port` | 临时覆盖控制台端口 |
| `-headless` | 不自动打开浏览器 |
| `-run` | 启动后自动开房 |
| `-debug` | 开启详细日志 |
| `-check` | 检查上游和账号鉴权后退出 |
| `-reset-admin-password` | 重置管理密码并打印新密码 |
| `-run-seconds` | 运行指定秒数后退出（测试用） |

环境变量：

- `CUBEDOCK_TOKEN`：引导用会话列表（空白分隔），仅在 `room-tokens.dat` 为空时
  使用，并立即写入受保护文件；
- `CUBEDOCK_ADMIN_PASSWORD`：设置管理密码（不会回显）；
- `CUBEDOCK_INTEROP_SYSTEM_VERSION`：可选，覆盖互通版登录上报的 Windows build；
  默认在 Windows 上读取本机 build，其他系统回退到通用值 `10.0.26200`；
  源码和文档不保存操作者本机的具体值。

## HTTP API

除 `/api/login` 和静态资源外，所有接口都需要管理员会话。

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| POST | `/api/login` / `/api/logout` | 管理员登录/退出 |
| POST | `/api/admin/password` | 修改管理密码 |
| GET | `/api/status` | 状态、账号、房间、事件 |
| GET | `/api/packs` | 扫描并返回 limbo 资源包目录列表 |
| POST | `/api/packs/refresh` | 假网易机器人登录后端服拉取资源包并去重 |
| POST | `/api/start` / `/api/stop` | 启动/停止全部房间 |
| POST | `/api/debug` | 即时切换 Debug |
| POST | `/api/console` | 执行控制台命令（help/list/kick/transfer） |
| POST | `/api/exit` | 请求进程退出 |
| POST | `/api/config` | 保存配置 |
| POST | `/api/session/interop-login` | 邮箱密码直登互通版（仅本机） |
| POST | `/api/session/import` | 导入互通版 sauth（仅本机） |
| POST | `/api/account/remove` | 移除账号 |
| POST | `/api/account/profile` | 重新解析游戏名 |

## 常见问题（FAQ）

### 标准 Bedrock Dedicated Server 能直接作为上游吗？

不能。CubeDock 只做登录重签、加密握手和游戏包直通，不翻译网易协议。上游必须
原生支持网易 RakNet v8；标准 BDS 不兼容。若上游要求单段登录链、转发地址信任
等配置，请按其文档启用并信任 CubeDock 的地址。

### 代理模式和转服模式有什么区别？分别适合什么场景？

- **代理模式（默认，主路径）**：玩家经 CubeDock 中继直接进入上游，游戏包原样
  转发。适合上游原生支持网易 RakNet v8、希望长期稳定中继、降低客户端网络要求，
  或需要上游看到网易身份的场景；不需要额外运行 limbo。
- **转服模式（limbo + Transfer）**：玩家先进入本地 limbo 虚空世界，预加载资源包
  并显示表单；点击按钮后通过 `Transfer` 进入目标服务器。适合客户端能直连目标、
  资源包未加密且静态、希望目标看到真实 IP/延迟，或希望先展示 limbo/表单再跳转
  的场景。目标服务器有资源包时，limbo 必须预加载同一份包；加密包、动态资源包和
  频繁变化的目标不适合转服模式。

### 手机看不到房间 / 开房失败怎么办？

先确认账号是互通版会话且未过期，PE 认证是否通过，房间是否进入 `online`；
同一账号被其他设备登录会把当前房间挤下线。若返回 1351，打开控制台显示的
验证链接完成验证后重试。日志中的 `server:` / `client:` 前缀可区分上游侧和
玩家侧错误。

### 玩家进房后马上断开？

查看控制台日志方向：

- `server:` 先断：多为上游或 limbo 侧拒绝/失败，检查单段登录链、转发地址
  信任、上游版本和资源包；
- `client:` 先断：多为玩家侧超时或主动关闭，检查客户端版本和网络。

代理模式要求上游能够接受 CubeDock 重签后的单段登录链并完成 Bedrock 加密
握手；游戏包本身不会被改写。

### limbo 进不去 / 长中文名或皮肤被拒绝？

limbo 已对长中文名和异常皮肤启用宽松登录；仍失败时检查 limbo 是否启动、
`server:` 日志以及 `DisplayName` 长度。加密资源包和动态资源包暂不支持。

### 资源包不下载 / 转服时报资源包错误？

网易客户端会忽略 `ResourcePacksInfo` 里的包列表，limbo 默认使用
`-packs-mode netease` 强制 SendPacks，并在最后一包后补发 Stack。请确认
`limbo_packs_dir` 指向正确目录、资源包与目标服务器一致；加密包/动态包不支持。
`ResourcePackStack` 后主动断开是拉包流程的预期行为。

### 资源包拉取失败 / 机器人被限速？

后端 AuthMe 或登录插件会对高频机器人登录限速。请使用唯一的机器人名、避免
短时间重复拉取，并确认上游可达、账号会话有效。拉取失败不会阻止开房。

### 控制台打不开 / 忘记管理员密码？

默认地址是 `http://127.0.0.1:18321/`。首次启动会在终端打印一次性管理员密码；
也可通过 `CUBEDOCK_ADMIN_PASSWORD` 设置。忘记密码时使用
`-reset-admin-password`。控制台只允许白名单 Host/Origin，`/api/session/*`
只允许本机访问。

### `list` / `kick` 怎么用？

控制台或 `/api/console` 支持：

```text
list
kick p1
kick "Alice Smith"
```

玩家名称含空格时必须加引号。

### 一个网易账号能开多个房间吗？

不能。同一账号重复登录会使已有房间掉线。每个账号对应一个房间，最多同时
运行 8 个账号/房间。

### 数据文件在哪里？会把账号提交到仓库吗？

数据默认在程序数据目录：`config.json`、`room-tokens.dat`、`accounts.json`、
`interop-device.dat`、`admin.json`、`logs/`。Windows 使用 DPAPI 保护会话，
其他平台使用 `0600` 文件。不要提交这些文件；仓库 `.gitignore` 已排除。

### 构建需要什么环境？

Go 1.25+（`go.mod` 使用 1.26.9 toolchain）、Node.js 24 和 npm。
`scripts/build.ps1` 会在缺少 `webui-src/node_modules` 时自动执行 `npm ci`。
Linux/macOS 手动构建步骤见上文。前端构建失败时先删除 `node_modules` 重新
`npm ci`。

### 发布包里的 `SOURCE/` 是什么？

CubeDock 原创代码使用 MIT。`vendor-src/g79client` 和 `internal/funauth` 是
AGPL-3.0；`SOURCE/` 随发布包提供对应源码和许可文件，以满足 AGPL 的分发要求。
公开分发前仍建议自行复核许可与法律义务。

## 开发

```bash
go test ./...
go vet ./...

cd webui-src
npm install
npm run build
```

更多运行说明见本 README；实现细节见源码注释。

## 许可证

- CubeDock 原创代码：MIT，`Copyright (c) 2026 emuyx`；
- 第三方组件的许可与归属见 [NOTICE](NOTICE) 和
  [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md)；
- `vendor-src/g79client` 与 `internal/funauth` 为 AGPL-3.0，见
  [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md)；
- `vendor-src/nemc-tan-lobby-solver` 的原上游仓库已被作者删除；CubeDock 删除了
  未使用的 demo，并把原先无许可证的 `bunker/` 重写为 MIT 实现。保留的子目录
  各自带有 MIT LICENSE，本地胶水见该目录的 `LICENSE` / `NOTICE`。

本项目仅供自用。使用自己的网易账号；自动化开房可能违反网易服务条款，风险由
使用者承担。


# CubeDock 方块码头

CubeDock 让网易版 Bedrock 玩家通过网易「本地联机」房间加入任何原生支持网易
协议的 Minecraft 基岩版服务器，不绑定特定上游实现。项目独立维护，保留第三方
房间/认证底座。

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


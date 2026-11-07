# 更新记录

## 0.1.0（未发布）

CubeDock 首次独立版本。

### 新增 / 调整

- 新 module `github.com/emuyx/cubedock`；`cmd/cubedock` 为主程序，
  `cmd/cubedock-launcher` 为发布 launcher（`build/CubeDock.exe`）；
- 重写 `fileutil`、`secret`、`session`、`gamebatch`、`relay`、`config`、
  `friends`、`room`、`bridge/ping`；
- 业务核心拆到 `internal/core`，HTTP 控制台拆到 `internal/server`；
- 品牌、Cookie、环境变量、主题键、日志文件名、房间 LevelID 全部改为 CubeDock；
- 数据文件改为 `room-tokens.dat` / `accounts.json` / `logs/cubedock.log`，不做旧
  数据迁移；
- 删除 HTTP FGAuth 兼容模式、`auth_server`/`auth_mode`、手游引擎版本下拉、
  `GET /api/netease/versions`、`internal/funauth/versions.go`；
- 保留互通版邮箱直登、会话导入、1351 安全验证重试、多账号、自动同意好友、
  加密握手中继和 RakNet v8 直通；
- 补齐 config/fileutil/secret/session/gamebatch/relay/room/friends/bridge/
  core/server 测试；
- 控制台日志默认可看到玩家加入/离开（含在线时长与包计数）、登录失败、
  房间满员、上游掉线与恢复；事件保留上限提高到 300 条；
- 自动同意好友：每 30 分钟重新发现好友网关，列表失败后强制刷新；单个申请
  失败不再阻塞后续申请；服务端签名格式变化时可重新协商；失败事件定期输出，
  退避上限缩短到 2 分钟；
- 「控制台」页可实时开关 Debug（调用 `/api/debug`），房间运行时也生效；
- Web 控制台改为侧边导航（主页/账号与房间/转服与资源包/控制台/设置），原顶部大信息栏移入
  主页；移动端使用临时抽屉；
- 控制台页升级为交互式终端：`help`、`list`（在线玩家、房间、状态、时长、包数）、
  `kick <ID|名称>`，支持命令历史与当前页面会话内输出；
- 新增每玩家注册表与稳定 ID `pN`，包计数改为原子读取，踢人通过取消该玩家的
  context 完成，并写入持久审计事件；
- 新增 `POST /api/console`；
- 适配网易互通版 3.10：PE 认证引擎/补丁更新到 `3.10.5.301486`，签名参数
  `PeAuthSign` 改为 `2/8`，`sa_data` 的 OS/数值字段与官方客户端对齐；x19
  `gv` 与默认房间版本改为 `1.21.130.0`，旧 `1.21.120.0` 配置自动迁移；
- 房间协议 ID 默认值改为 `43`，旧配置中的 `42` 加载时自动迁移；
- 文档明确 CubeDock 适用于任何支持网易 RakNet v8 协议的 Bedrock 服务器，
  不绑定特定上游实现；
- 更新 `nemc-tan-lobby-solver` 许可记录：原上游仓库已删除，根目录无许可证；
  删除未使用 demo、将原无许可证 `bunker/` 重写为 MIT 实现，保留子包 MIT 声明，
  并新增 `docs/ownership.md` 区分自写/第三方代码；
- 新增 `connection_mode` 设置：代理模式（默认、现有中继）与转服模式
  （limbo + Transfer）；转服模式已开放，转服与资源包页可直接选择，不再需要环境变量；
  启动时自动创建 `bin/packs/` 并打印资源包目录，详见
  `docs/transfer-limbo.md`；
- 新增 `limbo/` 辅助进程：gophertunnel 1.53.0 / protocol 898，StartGame +
  空区块 + 出生点基岩，发送表单，玩家点击按钮后 Transfer；主程序启动/退出时
  自动拉起/结束，Windows 用 Job Object 防止异常退出残留；
- limbo 支持资源包预加载：`-packs` / `limbo_packs_dir` 指定目录（留空自动查找
  `packs/`），通过 `ListenConfig.ResourcePacks` 在 StartGame 阶段下发，并等待
  3 秒后显示表单；资源包必须与目标服务器一致；当前只支持未加密包（不设置
  ContentKey）。本地补丁的 gophertunnel 会把网易客户端的 Completed/Refused/
  AllPacksDownloaded 响应强制改写为 SendPacks，确保资源包真正下载；
- limbo 离线登录允许超过 16 字节的网易中文名，并容忍异常的皮肤/设备字段；
  修复部分玩家登录 limbo 时因 `DisplayName ... 21 characters` 被断开；
- 转服 limbo 的 SimpleForm 支持在转服与资源包页自定义标题、正文和按钮文字
  （`limbo_form_title` / `limbo_form_content` / `limbo_form_button`），保存后
  重新启动房间生效；表单固定带「断开连接」按钮，误关表单会在 1 秒后重新打开；
- 新增 `GET /api/packs`：扫描 limbo 资源包目录并返回包名、UUID、版本、大小和
  解析错误；转服与资源包页新增资源包列表和「刷新列表」按钮（只重新扫描目录，资源包
  仍在 limbo 启动时加载）；
- 新增假网易机器人拉包：limbo `-pull` 通过 NetEase RakNet v8 + gophertunnel
  客户端登录后端服，下载未加密资源包；`limbo_auto_pull_packs`（默认开启）
  会在开房前自动执行，转服与资源包页也可手动触发 `POST /api/packs/refresh`；
- 拉取前按本地资源包的 UUID+版本跳过已存在的包；后端包全部命中时不传输包数据；
- 拉取完成后向服务器发送 RakNet `DisconnectNotification`（0x15），避免后端登录
  插件因机器人会话未释放而阻止下一次登录；
- 拉取前按本地包 UUID+版本跳过已存在的包；收到 `ResourcePackStack` 后、发送
  `Completed` 前发送 RakNet `DisconnectNotification`（0x15），避免机器人进入
  后端世界；
- 拉取后自动删除 UUID、版本和 SHA-256 完全相同的重复资源包（保留最新）；
  同 UUID/版本但内容不同只警告，不删除；
- 新增 `CubeDock.exe` launcher：首次启动释放 `bin/CubeDockMain.exe` 和
  `bin/limbo.exe`，后续启动按 SHA-256 校验并在需要时覆盖；配置/日志留在
  launcher 目录；launcher 被强制结束时主程序与 limbo 一并结束；
- 新增 `transfer <玩家> <host:port>` 控制台 POC：向已登录玩家注入 Bedrock
  `Transfer` 包，用于验证客户端能否直连目标服务器；修正 `Transfer` 包缺少
  `ReloadWorld` 字段导致的客户端立即报错/断开；
- 重写 README、部署/架构文档与许可证说明。

### 已知限制

- 当前只验证了未加密、静态资源包的转服流程；加密包、动态资源包和多房间并发
  尚未覆盖；
- `vendor-src/g79client` 和 `internal/funauth` 为 AGPL-3.0；公开分发或提供
  网络服务时需履行 AGPL 义务；
- 网易互通版升级后需要重新核对 PE 认证常量、`gv` 和房间协议版本。


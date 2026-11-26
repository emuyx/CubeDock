# CubeDock 方块码头

<p align="center">
  <a href="https://github.com/emuyx/CubeDock/releases/latest"><img alt="Release" src="https://img.shields.io/github/v/release/emuyx/CubeDock?color=f2a0bc&label=release"></a>
  <a href="https://github.com/emuyx/CubeDock/releases"><img alt="Total downloads" src="https://img.shields.io/github/downloads/emuyx/CubeDock/total?color=f2a0bc&label=downloads"></a>
  <a href="https://github.com/emuyx/CubeDock/actions/workflows/release.yml?query=branch%3Amain"><img alt="Build" src="https://img.shields.io/github/actions/workflow/status/emuyx/CubeDock/release.yml?branch=main&label=build"></a>
  <img alt="Go" src="https://img.shields.io/badge/Go-1.26%2B-8fe3d2?logo=go&logoColor=white">
  <img alt="MIT" src="https://img.shields.io/badge/license-MIT-f2a0bc">
  <a href="https://emuyx.github.io/CubeDock/"><img alt="Pages" src="https://img.shields.io/badge/docs-GitHub%20Pages-8fe3d2"></a>
</p>

把网易版 Minecraft 的「本地联机」房间，接到你自己的 Bedrock 服务器上。
下载、打开、按向导点几下，就能开房啦 (｡•ᴗ•｡)

[项目主页](https://emuyx.github.io/CubeDock/) · [使用文档](https://emuyx.github.io/CubeDock/docs.html) · [界面演示](https://emuyx.github.io/CubeDock/demo.html) · [下载](https://github.com/emuyx/CubeDock/releases/latest) · [测试版构建](https://github.com/emuyx/CubeDock/actions/workflows/release.yml?query=branch%3Amain) · [Issues](https://github.com/emuyx/CubeDock/issues)

> 当前稳定版是 **v0.4.5**，去 [Releases](https://github.com/emuyx/CubeDock/releases/latest) 下载对应系统的包就好。
> `main` 分支跟随最新发布版本；后续开发改动会先出现在测试版构建里，偶尔也可能不太稳定。
> 想尝鲜的话，普通用户也可以直接去 [GitHub Actions 的测试版构建](https://github.com/emuyx/CubeDock/actions/workflows/release.yml?query=branch%3Amain) 下载 artifacts，解压方式和稳定版一样；只是不保证稳定，长期用还是推荐 Releases。

## 它适合谁

- 服务器已经原生支持网易 RakNet v8，想让网易玩家通过「本地联机」进来。
- 不想研究协议、抓包或者改客户端，希望有个网页控制台点点点。
- 需要同时管理多个网易账号 / 多个房间。
- 想先用最简单的代理模式，之后再按需要切到转服模式。

> 标准 Bedrock Dedicated Server（BDS）不能直接用哦，服务端必须支持网易协议。

## 第一次使用

1. 去 [Releases](https://github.com/emuyx/CubeDock/releases/latest) 下载对应系统的压缩包，解压。
2. Windows 双击 `CubeDock.exe`；Linux / macOS 在终端运行解压出的程序，必要时先 `chmod +x`。
3. 浏览器打开控制台（一般会自动打开，默认 `http://127.0.0.1:18321/`），第一次会让你创建管理员密码。
4. 按页面向导填服务器地址、登录网易账号、启动房间。手机在网易版 Minecraft 的「本地联机」里搜索房间号就能进。

> 在交互式终端里启动时会直接显示字符版控制台（TUI），浏览器控制台仍然可以同时使用，两边操作的是同一套房间。完整的页面、按键和操作步骤在[使用文档的终端控制台部分](https://emuyx.github.io/CubeDock/docs.html#tui)；没把握的话，先用网页控制台就好 (｡•ᴗ•｡)

更细的步骤在 [使用文档](https://emuyx.github.io/CubeDock/docs.html)。第一次用不用着急，慢慢来 (｡•ᴗ•｡)

## 两种连接方式

| | 代理模式（推荐先试） | 转服模式 |
| --- | --- | --- |
| 怎么走 | 玩家 → CubeDock → 你的服务器 | 玩家 → 本地大厅 → 你的服务器 |
| 优点 | 配置最少、开房快 | 可预载资源包、服务器能看到真实 IP |
| 适合 | 想尽快开起来 | 需要大厅、资源包或真实 IP |

拿不准就先用代理模式，之后在「配置方案」里随时能改。

## 常见问题

- **标准 BDS 能直接用吗？** 不能，服务器要原生支持网易 RakNet v8；两种模式都一样。
- **手机搜不到房间 / 开房失败？** 确认账号能登录、没在其他设备上登录、房间已经在线；提示 1351 时按页面完成验证，再点一次「登录并添加」。
- **玩家进来就断？** 先检查服务器协议、版本和资源包；转服模式还要确认玩家能直连服务器。
- **控制台打不开 / 忘记密码？** 默认 `http://127.0.0.1:18321/`；忘记密码就停止程序、备份数据目录、删除 `admin.json`，重启后重新创建。
- **一个账号能开几个房间？** 一个账号一间房；多开几个账号就能多开几间。
- **数据安全吗？** 凭证保存在本机数据目录，Windows 用 DPAPI，其他平台用 `0600` 权限。密码默认不保存；可选保存到 `account-passwords.dat`，同样受本机保护。不要把这些文件发给别人。

更多问题可以看 [文档的故障排查](https://emuyx.github.io/CubeDock/docs.html#troubleshooting) 或控制台里的「常见问题」页。先别急 (・∀・)

## 给开发者

从源码构建：

Windows：

```powershell
git clone https://github.com/emuyx/CubeDock.git
cd CubeDock
.\scripts\build.ps1
.\build\CubeDock.exe
```

Linux / macOS：

```bash
git clone https://github.com/emuyx/CubeDock.git
cd CubeDock
go build -trimpath -o build/bin/CubeDockMain ./cmd/cubedock
(cd limbo && go build -trimpath -o ../build/bin/limbo .)
./build/bin/CubeDockMain
```

需要 Go 1.26+（会自动使用 `go.mod` 指定的 1.27.4 工具链）和 Node.js 24。常用检查：

```bash
go test ./...
go vet ./...
go -C limbo test ./...
go -C limbo vet ./...

npm --prefix webui-src ci
npm --prefix webui-src run build
```

Windows 一键构建：`.\scripts\build.ps1`。站点文件在 `site/`，`docs.html` 是使用文档，`demo.html` 是控制台演示。想提前试新代码可以看 GitHub Actions 的测试版构建，但那边不保证稳定。

## 当前状态

| 能力 | 状态 |
| --- | --- |
| 代理模式真实进服 / 转服 + 资源包 | ✅ 实机验证 |
| 多账号 / 多房间并发 | ✅ 多个真实 4399 账号同时开房 45 秒，0 掉线 |
| 互通版 / 网易手游 / 4399 手游登录 | ✅ 真机验证登录、开房、进服 |
| 配置方案 + 账号仪表盘 + 房间管理 | ✅ 实机验证 |
| 终端控制台（TUI） | ✅ 本地测试与 Windows 冒烟；真实终端/输入法待复测 |
| 房间玩家区分 / 自动刷新 / 头像 / 二维码 / 保活 / 本地密码 | ✅ 本地测试与 demo 验证 |
| 加密 / 动态资源包 | 🚧 暂不支持 |
| Linux WSL2（test/vet/race/构建/运行时冒烟） | ✅ 通过 |
| macOS 实机 | ⏳ 仅交叉编译/vet |
| `go test -race` | ✅ Linux WSL2（root + limbo） |

> 这些是目前的验证范围，不代表所有服务器/客户端组合都一定顺利。
> 遇到问题欢迎带诊断报告来提 Issue，但请不要发账号、token 或数据目录。

## 许可

CubeDock 原创代码（含 `internal/netease` 认证链）使用 MIT，`Copyright (c) 2026 emuyx`。
`vendor-src/nemc-tan-lobby-solver` 和 `limbo/third_party/gophertunnel` 仍是 MIT，各自保留 LICENSE。
完整归属见 [NOTICE](NOTICE)、[THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md) 和 [LICENSE](LICENSE)。

> CubeDock 是独立项目，与 Mojang、Microsoft、网易无隶属关系；不附带游戏客户端或官方资源。
> 使用网易账号与本地联机功能可能受网易服务条款约束，风险由使用者承担。

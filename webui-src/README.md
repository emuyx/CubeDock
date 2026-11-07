# CubeDock 控制台（React + Material UI）

`webui-src` 是 CubeDock 浏览器控制台源码，构建产物写入
`internal/webui/dist`，再由 `internal/webui/embed.go` 内嵌进 Go 二进制。

```bash
cd webui-src
npm install
npm run build

cd ..
.\scripts\build.ps1   # Windows：构建 launcher + 主程序 + limbo
```

开发模式（Vite 5173，API 代理到本机 18321）：

```bash
npm run dev
```

页面结构：

- `/login`：管理员登录；
- `/`：侧边导航 **主页 / 账号与房间 / 控制台 / 设置**；移动端用汉堡按钮打开抽屉；
- 主页显示房间号、启动/停止、上游状态和统计卡；
- 控制台显示事件日志并可输入 `help` / `list` / `kick`，支持 ↑/↓ 历史；
- 账号登录只支持互通版邮箱直登和手动导入互通版 sauth；
- 设置页不再提供手游引擎版本下拉。

安全约束：

- 控制台登录页与主界面是同一个 SPA；服务器对 `/login` 返回 `dist/index.html`；
- 生产 CSP 允许 `style-src 'unsafe-inline'`（MUI/Emotion），脚本仍只允许同源；
- `/assets/` 和 `/favicon.svg` 免登录访问，其余页面和 API 需要管理员会话；
- 前端依赖全部来自本地 `node_modules`，不访问外部 CDN。


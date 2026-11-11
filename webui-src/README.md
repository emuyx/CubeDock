# CubeDock 控制台（React + Material UI）

`webui-src` 是 CubeDock 浏览器控制台源码，构建产物写入
`internal/webui/dist`，再由 `internal/webui/embed.go` 内嵌进 Go 二进制。

```bash
cd webui-src
npm install
npm run build
npm run build:demo   # 生成 site/demo-app/（GitHub Pages 交互演示）

cd ..
.\scripts\build.ps1   # Windows：构建 launcher + 主程序 + limbo
```

开发模式（Vite 5173，API 代理到本机 18321）：

```bash
npm run dev
```

页面结构：

- `/login`：管理员登录；
- `/`：侧边导航 **主页 / 账号与房间 / 连接与资源包 / 控制台 / 诊断 / 常见问题 / 设置**；
  移动端用汉堡按钮打开抽屉；
- 首次启动（无账号）自动进入设置向导：连接设置 → 添加账号 → 运行预检 → 开始房间；
- 主页显示房间号、启动/停止、上游状态和统计卡；
- 账号页支持搜索、状态筛选、排序、批量粘贴 sauth 会话导入；凭据过期时用「更新登录」重新登录，而不是复用旧 cookie；
- 设置页拆成「房间 / 行为 / 高级 / 备份」，连接页拆成「连接方式 / 资源包 / 提示页面」，诊断页按分类分页；
- 左侧栏每个账号都有独立的房间面板，可查看状态、复制房间号；代理模式下主页和账号页也可以快速打开玩家面板，查看、断开或请求转服；
- 设置页常用项在前，房间版本/协议等高级项收进「高级」，关键字段带小问号说明；
- 控制台显示事件日志并可输入 `help` / `list` / `kick`，支持 ↑/↓ 历史；
- 玩家列表、踢人、请求转服只适用于代理模式；转服模式的玩家由中转房间/目标服务器承载；
- 诊断页只读检查服务器、中转房间、资源包、账号、占用和本地环境，可复制/下载脱敏报告；房间停止时不会误报“中转房间不可达”。
- 设置页提供配置备份/恢复；备份不含网易会话 token，恢复只应用配置；
- 账号登录只支持互通版邮箱直登和手动/批量导入互通版 sauth；
- 公开演示使用同一套 React + MUI 组件，配 `src/demoApi.js` 的内存模拟 API，不再维护单独的静态空壳页面。

安全约束：

- 控制台登录页与主界面是同一个 SPA；服务器对 `/login` 返回 `dist/index.html`；
- 生产 CSP 允许 `style-src 'unsafe-inline'`（MUI/Emotion），脚本仍只允许同源；
- `/assets/` 和 `/favicon.svg` 免登录访问，其余页面和 API 需要管理员会话；
- 前端依赖全部来自本地 `node_modules`，不访问外部 CDN。


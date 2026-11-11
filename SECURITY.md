# 安全问题反馈

不要在公开 Issue 中贴出可利用细节、真实会话、Cookie、密码或玩家数据。

报告请包含：版本、影响范围、最小复现步骤、脱敏后的错误信息。涉及凭据泄露时
先通过维护者 GitHub 主页确认私密渠道，再发送详情。

## 部署边界

- 控制台默认监听 `0.0.0.0:18321`；除登录页、静态资源、`/api/login`、
  `/api/setup/status` 和 `/api/setup` 外，其余页面与 API 都需要管理员会话；
- 首个管理员的创建（`/api/setup`）只允许 loopback，远程访问者不能抢占初始化；
  登录有按 IP 的限速与会话上限；
- 管理员密码使用 bcrypt 保存；Cookie 为 HttpOnly、SameSite=Strict、12 小时，
  进程重启后失效；修改密码会使全部旧会话失效；
- 有 Host 白名单、Origin/Content-Type 校验、登录限速和 CSP；
- 默认 HTTP **不加密**。限制为可信网络，跨不可信网络使用 SSH 隧道或 HTTPS
  反向代理；
- `/api/session/import` 与 `/api/session/interop-login` 只允许 loopback，避免
  密码/会话经局域网明文传输；
- 网易会话保存在 `room-tokens.dat`：Windows 用 DPAPI 绑定当前用户，其他平台
  为 0600 文件；这不等于磁盘加密；认证链由第一方 `internal/netease` 实现，
  不再包含 AGPL 组件；
- `interop-device.dat`、`admin.json`、日志和 `accounts.json`（含 `profile_id`）
  也可能含敏感信息，不要提交或分享；
- 房间管理接口（房间踢人、标签、准入等级、好友申请）与其他管理 API 一样需要
  管理员会话；房间踢出不会在服务端封禁玩家，玩家仍可能重新加入；
- 上游服务端的单段链/转发信任配置应限制在 CubeDock 所在地址或可信网络。

## 风险提示

- 自动化登录和开房可能触发网易风控（1351 安全验证）；
- 同一会话在官方客户端登录会挤掉 CubeDock 的房间；
- 使用自己的账号，并自行评估网易服务条款和账号风险。


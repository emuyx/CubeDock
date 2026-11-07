import React from 'react';
import {
  Accordion,
  AccordionDetails,
  AccordionSummary,
  Box,
  Card,
  CardContent,
  Chip,
  Stack,
  Typography,
} from '@mui/material';
import { ExpandMore, Help } from '@mui/icons-material';

const FAQ = [
  {
    question: '代理模式和转服模式有什么区别？分别适合什么场景？',
    answer: (
      <Stack spacing={1.2}>
        <Typography variant="subtitle2" color="text.primary">
          代理模式（默认，主路径）
        </Typography>
        <Typography variant="body2">
          玩家流量经 CubeDock 中继到上游，登录链由 CubeDock 重签，游戏包原样转发。
          上游看到的是网易身份和 CubeDock 的连接，不需要客户端直连上游。
        </Typography>
        <Typography variant="body2">
          适合：上游原生支持网易 RakNet v8；希望长期稳定中继、降低客户端网络要求；
          需要上游看到网易身份；不想额外运行 limbo。
        </Typography>
        <Typography variant="subtitle2" color="text.primary">
          转服模式（limbo + Transfer）
        </Typography>
        <Typography variant="body2">
          玩家先进入本地 limbo 虚空世界，预加载资源包并显示表单；点击按钮后通过
          Transfer 让客户端直连目标服务器。目标服务器看到玩家真实 IP/延迟，
          CubeDock 不再长期中继游戏流量。
        </Typography>
        <Typography variant="body2">
          适合：客户端能直连目标服务器；资源包未加密且静态；希望先展示 limbo/表单
          再进入目标；希望目标看到真实 IP/延迟。目标服务器有资源包时，limbo 必须
          预加载同一份包。加密包、动态资源包和频繁变化的目标不适合转服模式。
        </Typography>
        <Typography variant="body2" color="text.secondary">
          当前实现只覆盖未加密、静态资源包；多房间并发仍未验证。
        </Typography>
      </Stack>
    ),
  },
  {
    question: '标准 Bedrock Dedicated Server 能直接作为上游吗？',
    answer: (
      <Typography variant="body2">
        代理模式不能：上游必须原生支持网易 RakNet v8，标准 BDS 不兼容。
        转服模式的目标服务器由客户端直连，但仍要求客户端能访问目标、资源包与
        limbo 预加载的一致；它不会把网易协议翻译成标准 Bedrock 协议。
      </Typography>
    ),
  },
  {
    question: '手机看不到房间 / 开房失败怎么办？',
    answer: (
      <Stack spacing={0.8}>
        <Typography variant="body2">
          确认账号是互通版会话且未过期、PE 认证通过、房间进入 online；同一账号被
          其他设备登录会把当前房间挤下线。
        </Typography>
        <Typography variant="body2">
          若返回 1351，打开控制台显示的验证链接完成验证后重试。日志中的
          server: / client: 前缀可区分上游侧和玩家侧错误。
        </Typography>
      </Stack>
    ),
  },
  {
    question: '玩家进房后马上断开？',
    answer: (
      <Stack spacing={0.8}>
        <Typography variant="body2">
          server: 先断：多为上游或 limbo 侧拒绝/失败，检查单段登录链、转发地址
          信任、上游版本和资源包。
        </Typography>
        <Typography variant="body2">
          client: 先断：多为玩家侧超时或主动关闭，检查客户端版本和网络。
          代理模式要求上游接受 CubeDock 重签后的单段登录链并完成 Bedrock 加密握手。
        </Typography>
      </Stack>
    ),
  },
  {
    question: 'limbo 进不去 / 长中文名或皮肤被拒绝？',
    answer: (
      <Typography variant="body2">
        limbo 已对长中文名和异常皮肤启用宽松登录。仍失败时检查 limbo 是否启动、
        server: 日志和 DisplayName 长度。加密资源包、动态资源包暂不支持。
      </Typography>
    ),
  },
  {
    question: '资源包不下载 / 转服时报资源包错误？',
    answer: (
      <Stack spacing={0.8}>
        <Typography variant="body2">
          网易客户端会忽略 ResourcePacksInfo，limbo 默认用 netease 模式强制
          SendPacks，并在最后一包后补发 Stack。确认资源包目录正确、资源包与目标
          服务器一致。
        </Typography>
        <Typography variant="body2">
          ResourcePackStack 后主动断开是拉包流程的预期行为。加密包/动态包不支持；
          目标资源包变化时，limbo 无法提前缓存同一份包。
        </Typography>
      </Stack>
    ),
  },
  {
    question: '资源包拉取失败 / 机器人被限速？',
    answer: (
      <Typography variant="body2">
        后端 AuthMe 或登录插件会对高频机器人登录限速。使用唯一机器人名、避免短时间
        重复拉取，并确认上游可达、账号会话有效。拉取失败不会阻止开房。
      </Typography>
    ),
  },
  {
    question: '控制台打不开 / 忘记管理员密码？',
    answer: (
      <Typography variant="body2">
        默认地址是 http://127.0.0.1:18321/。首次启动会在终端打印一次性管理员密码；
        也可通过 CUBEDOCK_ADMIN_PASSWORD 设置。忘记密码时使用
        -reset-admin-password。控制台只允许白名单 Host/Origin，/api/session/* 只允许本机。
      </Typography>
    ),
  },
  {
    question: 'list / kick 怎么用？',
    answer: (
      <Typography variant="body2" component="div">
        控制台或 /api/console 支持：
        <Box component="pre" sx={{ mt: 1, mb: 0, p: 1.2, borderRadius: 1, bgcolor: 'action.hover', overflowX: 'auto' }}>
          {'list\nkick p1\nkick "Alice Smith"'}
        </Box>
        玩家名称含空格时必须加引号。
      </Typography>
    ),
  },
  {
    question: '一个网易账号能开多个房间吗？',
    answer: (
      <Typography variant="body2">
        不能。同一账号重复登录会使已有房间掉线。每个账号对应一个房间，最多同时运行
        8 个账号/房间。
      </Typography>
    ),
  },
  {
    question: '数据文件在哪里？会把账号提交到仓库吗？',
    answer: (
      <Typography variant="body2">
        数据默认在程序数据目录：config.json、room-tokens.dat、accounts.json、
        interop-device.dat、admin.json、logs/。Windows 使用 DPAPI 保护会话，其他平台
        使用 0600 文件。不要提交这些文件。
      </Typography>
    ),
  },
  {
    question: '构建需要什么环境？',
    answer: (
      <Typography variant="body2">
        Go 1.25+（go.mod 使用 1.26.9 toolchain）、Node.js 24 和 npm。
        scripts/build.ps1 会在缺少 webui-src/node_modules 时自动 npm ci。前端构建失败时
        先删除 node_modules 重新 npm ci。
      </Typography>
    ),
  },
  {
    question: '发布包里的 SOURCE/ 是什么？',
    answer: (
      <Typography variant="body2">
        CubeDock 原创代码使用 MIT。vendor-src/g79client 和 internal/funauth 是
        AGPL-3.0；SOURCE/ 随发布包提供对应源码和许可文件，以满足 AGPL 的分发要求。
        公开分发前仍建议自行复核许可与法律义务。
      </Typography>
    ),
  },
];

export default function FaqPanel() {
  return (
    <Stack spacing={2.4}>
      <Card>
        <CardContent sx={{ p: { xs: 2.4, sm: 3.4 } }}>
          <Stack direction="row" spacing={1.6} alignItems="center">
            <Box
              sx={{
                width: 46,
                height: 46,
                borderRadius: 2,
                display: 'grid',
                placeItems: 'center',
                bgcolor: 'action.hover',
              }}
            >
              <Help color="primary" />
            </Box>
            <Box sx={{ flexGrow: 1 }}>
              <Typography variant="h4">常见问题</Typography>
              <Typography color="text.secondary" sx={{ mt: 0.4 }}>
                上游要求、代理/转服选择、资源包、控制台和构建问题。
              </Typography>
            </Box>
            <Chip size="small" variant="outlined" label={`${FAQ.length} 条`} />
          </Stack>
        </CardContent>
      </Card>

      <Card>
        <CardContent sx={{ p: 0, '&:last-child': { pb: 0 } }}>
          {FAQ.map((item, index) => (
            <Accordion
              key={item.question}
              disableGutters
              elevation={0}
              sx={{
                bgcolor: 'transparent',
                '&:before': { display: 'none' },
                borderBottom: index < FAQ.length - 1 ? 1 : 0,
                borderColor: 'divider',
              }}
            >
              <AccordionSummary
                expandIcon={<ExpandMore />}
                sx={{ px: { xs: 2, sm: 3 }, py: 1.3 }}
              >
                <Typography fontWeight={750}>{item.question}</Typography>
              </AccordionSummary>
              <AccordionDetails sx={{ px: { xs: 2, sm: 3 }, pt: 0, pb: 2.4 }}>
                <Box sx={{ color: 'text.secondary' }}>{item.answer}</Box>
              </AccordionDetails>
            </Accordion>
          ))}
        </CardContent>
      </Card>
    </Stack>
  );
}

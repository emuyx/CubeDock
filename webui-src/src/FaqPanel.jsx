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

const text = children => <Typography variant="body2">{children}</Typography>;

const FAQ = [
  {
    question: '开始之前，我需要准备什么？',
    answer: (
      <Stack spacing={0.8}>
        {text('一个可以正常登录的网易账号（互通版），以及一台支持网易协议的服务器。')}
        {text('代理和转服模式都需要服务器支持网易协议；标准 Bedrock 服务器不能直接使用。')}
        {text('如果还没有服务器，可以先在「诊断」页检查服务器是否可达，再决定怎么部署。')}
      </Stack>
    ),
  },
  {
    question: '标准 Bedrock 服务器能直接用吗？',
    answer: (
      <Stack spacing={0.8}>
        {text('不能。CubeDock 不会把网易协议转换成标准 Bedrock，所以服务器本身必须能接受网易互通版客户端。')}
        {text('代理模式由 CubeDock 转发玩家流量；转服模式让玩家先进本地中转房间、再跳转到服务器。两种方式对服务器的要求一样。')}
      </Stack>
    ),
  },
  {
    question: '代理模式和转服模式怎么选？',
    answer: (
      <Stack spacing={0.8}>
        <Typography variant="subtitle2" color="text.primary">代理模式（推荐先试）</Typography>
        {text('玩家的游戏流量一直经过 CubeDock。房间更稳定，玩家不需要能直连服务器，适合大多数情况。')}
        <Typography variant="subtitle2" color="text.primary">转服模式</Typography>
        {text('玩家先进入一个本地中转房间，看到提示页面，点击按钮后才进入你的服务器。服务器能看到玩家真实 IP 和延迟。')}
        {text('转服模式要求玩家能直连服务器，资源包最好是未加密、不会频繁变化的；加密或动态资源包暂时不支持。')}
      </Stack>
    ),
  },
  {
    question: '手机搜不到房间，或者开房一直失败？',
    answer: (
      <Stack spacing={0.8}>
        {text('先在「诊断」页看几项：服务器是否可达、账号是否过期、服务器地址是否正确。')}
        {text('确认这个网易账号没有在手机或其他设备上登录；同一账号重复登录会把当前房间挤下线。')}
        {text('如果提示需要安全验证（1351），按页面提示完成验证后，再点一次「登录并添加」。')}
      </Stack>
    ),
  },
  {
    question: '登录时要求完成网易安全验证（1351），要怎么做？',
    answer: (
      <Stack spacing={0.8}>
        {text('点击「打开网易验证页面」，按网易的提示完成验证，然后回到 CubeDock，再点一次「登录并添加」。')}
        {text('CubeDock 不会自动重试，也不会在后台反复登录。这样你能确认每一步都是自己操作的。')}
        {text('如果验证链接失效，重新点一次登录，刷新出的链接通常还能用。')}
      </Stack>
    ),
  },
  {
    question: '玩家进房后马上断开？',
    answer: (
      <Stack spacing={0.8}>
        {text('先确认服务器确实支持网易协议，并且版本与玩家客户端接近。标准 Bedrock 服务器会在这里失败。')}
        {text('代理模式还要确认服务器能接受 CubeDock 转发后的登录请求；转服模式要确认玩家能直连服务器地址。')}
        {text('资源包不一致也会导致断开。可以在「连接与资源包」页拉取同一份资源包，再重新启动房间。')}
        {text('如果仍然失败，在「诊断」页复制报告，或到「控制台」看最近的错误。')}
      </Stack>
    ),
  },
  {
    question: '资源包不下载，或者进中转房间时报资源包错误？',
    answer: (
      <Stack spacing={0.8}>
        {text('在「连接与资源包」页打开「开房时自动从服务器拉取资源包」，或者手动点「从服务器拉取」。拉取后重新启动房间。')}
        {text('资源包需要和服务器使用的一致；如果服务器换了资源包，本地也要重新拉取。')}
        {text('加密资源包和动态资源包暂不支持，这类包会被跳过。')}
      </Stack>
    ),
  },
  {
    question: '资源包拉取失败，或者提示临时账号被限速？',
    answer: (
      <Stack spacing={0.8}>
        {text('拉取时会用一个临时网易账号登录服务器，部分服务器会限制短时间内频繁登录。稍等一会儿再试。')}
        {text('拉取失败不会阻止开房；也可以手动把资源包放进资源包目录。')}
        {text('如果服务器登录插件限制了临时账号名字，换一个更独特的上游账号名再试。')}
      </Stack>
    ),
  },
  {
    question: '一个网易账号能开几个房间？',
    answer: (
      <Stack spacing={0.8}>
        {text('一个账号对应一个房间。同一个账号登录第二个房间，会让前一个掉线。')}
        {text('多个账号可以同时开多个房间，数量没有固定上限，但实际能用多少取决于电脑资源和网易的限制。')}
        {text('批量添加可以到「账号与房间」页使用「批量导入」。')}
      </Stack>
    ),
  },
  {
    question: '控制台打不开，或者忘记管理员密码？',
    answer: (
      <Stack spacing={0.8}>
        {text('默认地址是 http://127.0.0.1:18321/。首次启动时，终端会显示一次性管理员密码。')}
        {text('忘记密码时，停止程序后运行 -reset-admin-password 重置；也可以在启动前设置 CUBEDOCK_ADMIN_PASSWORD。')}
        {text('如果要从局域网访问，需要输入管理员密码，并确保网络可信；账号登录部分仍然只允许在本机操作。')}
      </Stack>
    ),
  },
  {
    question: '换电脑、重装，或者想备份配置？',
    answer: (
      <Stack spacing={0.8}>
        {text('在「设置」页下载配置备份。备份包含服务器、房间和账号信息，但不会包含网易登录凭证。')}
        {text('换电脑恢复配置后，需要重新登录或导入账号。登录凭证和本机绑定，换机器不能直接复制。')}
        {text('数据默认保存在程序数据目录；不要把 config.json、room-tokens.dat、账号文件发给别人。')}
      </Stack>
    ),
  },
  {
    question: '开了「仅好友可见」，为什么还会有陌生人进来？',
    answer: (
      <Stack spacing={0.8}>
        {text('检查是否同时开启了「自动同意好友申请」。开启仅好友可见时，CubeDock 会跳过自动同意，避免陌生人通过好友申请进入。')}
        {text('如果之前已经同意过陌生人，对方仍在好友列表里；可以到网易好友列表里移除。')}
      </Stack>
    ),
  },
  {
    question: '房间人数、掉线重连、自动开房怎么设置？',
    answer: (
      <Stack spacing={0.8}>
        {text('房间总人数包含 CubeDock 自己占的 1 个名额，所以填 10 时最多进 9 位玩家。')}
        {text('掉线重试间隔默认 10 秒；网络不稳定时可以适当加大。')}
        {text('「打开程序时自动开房」适合长期挂机，但如果账号或服务器还没准备好，建议保持关闭。')}
        {text('房间版本和协议版本一般在「高级设置」里，除非网易更新客户端，否则不需要改。')}
      </Stack>
    ),
  },
  {
    question: '怎么更新 CubeDock？遇到问题怎么反馈？',
    answer: (
      <Stack spacing={0.8}>
        {text('从 Release 下载新版本，先备份配置，再替换程序文件。账号凭证通常可以继续使用。')}
        {text('遇到问题时，先到「诊断」页点「复制报告」，再到「控制台」查看最近错误。')}
        {text('反馈时请附上诊断报告和错误提示，但不要发送账号密码、登录凭证或完整数据目录。')}
      </Stack>
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
                开房、进服、网易验证、资源包、备份和常见报错。
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

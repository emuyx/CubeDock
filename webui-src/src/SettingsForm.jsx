import React from 'react';
import {
  Alert,
  Box,
  Button,
  Card,
  CardContent,
  CircularProgress,
  Divider,
  FormControlLabel,
  Stack,
  Switch,
  TextField,
  Typography,
} from '@mui/material';
import { Save } from '@mui/icons-material';
import { useConfigForm } from './configForm.jsx';

export default function SettingsForm({ status }) {
  const { form, update, save, busy, error } = useConfigForm();
  const running = !!status.running;

  if (!form) {
    return (
      <Box sx={{ py: 8, display: 'grid', placeItems: 'center' }}>
        <CircularProgress />
      </Box>
    );
  }

  const grid = { display: 'grid', gap: 2, gridTemplateColumns: { xs: '1fr', md: '1fr 1fr' } };

  return (
    <Card component="form" onSubmit={save}>
      <CardContent sx={{ p: { xs: 2.4, sm: 3.2 } }}>
        <Stack spacing={2.4}>
          <Box>
            <Typography variant="h6">房间基础设置</Typography>
            <Typography variant="body2" color="text.secondary">
              上游、房间信息和人数限制。连接模式、limbo 与资源包请在「转服与资源包」页配置。
            </Typography>
          </Box>
          {running && <Alert severity="info">房间运行中，设置暂时不可修改。</Alert>}
          {(status.accounts || []).some(account => account.platform === 'interop') && (
            <Alert severity="info">
              互通版会话固定使用官方 a50/PC 认证参数，不需要选择网易客户端版本。
            </Alert>
          )}
          {error && <Alert severity="error">{error}</Alert>}

          <Box sx={grid}>
            <TextField
              label="本地 Bedrock 地址"
              value={form.upstream || ''}
              onChange={event => update('upstream', event.target.value)}
              disabled={running}
              required
              fullWidth
              helperText="IP:UDP 端口，例如 127.0.0.1:19132"
            />
            <TextField
              label="网易房间内核版本"
              value={form.room_game_version || ''}
              onChange={event => update('room_game_version', event.target.value)}
              disabled={running}
              required
              fullWidth
              helperText="当前验证版本 1.21.130.0"
            />
          </Box>

          <Box sx={grid}>
            <TextField
              label="房间名称"
              value={form.room_name || ''}
              onChange={event => update('room_name', event.target.value)}
              disabled={running}
              required
              fullWidth
              inputProps={{ maxLength: 40 }}
            />
            <TextField
              label="房间密码"
              type="password"
              value={form.room_password || ''}
              onChange={event => update('room_password', event.target.value)}
              disabled={running}
              fullWidth
              autoComplete="off"
              helperText="留空表示无密码"
            />
          </Box>

          <Box sx={grid}>
            <TextField
              label="房间总人数（含机器人）"
              type="number"
              value={form.max_players ?? 10}
              onChange={event => update('max_players', Number(event.target.value))}
              disabled={running}
              fullWidth
              inputProps={{ min: 2, max: 10 }}
              helperText="网易房间最多 10 人，机器人占 1 个名额"
            />
            <TextField
              label="掉线重试间隔（秒）"
              type="number"
              value={form.retry_seconds ?? 10}
              onChange={event => update('retry_seconds', Number(event.target.value))}
              disabled={running}
              fullWidth
              inputProps={{ min: 5, max: 300 }}
            />
          </Box>

          <Box sx={grid}>
            <TextField
              label="网易房间协议 ID"
              type="number"
              value={form.lobby_protocol_id ?? 43}
              onChange={event => update('lobby_protocol_id', Number(event.target.value))}
              disabled={running}
              fullWidth
              inputProps={{ min: 1, max: 255 }}
              helperText="默认 43；与房间内核版本一起确认"
            />
          </Box>

          <Divider />
          <Typography variant="subtitle2" color="text.secondary">
            房间行为
          </Typography>
          <Box
            sx={{
              display: 'grid',
              gap: 1.2,
              gridTemplateColumns: { xs: '1fr', sm: '1fr 1fr' },
            }}
          >
            <FormControlLabel
              control={<Switch checked={!!form.friends_only} onChange={event => update('friends_only', event.target.checked)} disabled={running} />}
              label="仅好友可见"
            />
            <FormControlLabel
              control={<Switch checked={!!form.auto_accept_friends} onChange={event => update('auto_accept_friends', event.target.checked)} disabled={running} />}
              label="自动同意好友申请"
            />
            <FormControlLabel
              control={<Switch checked={!!form.allow_pvp} onChange={event => update('allow_pvp', event.target.checked)} disabled={running} />}
              label="允许 PvP"
            />
            <FormControlLabel
              control={<Switch checked={!!form.auto_start} onChange={event => update('auto_start', event.target.checked)} disabled={running} />}
              label="程序启动时自动开房"
            />
          </Box>
          <Typography variant="caption" color="text.secondary">
            Debug 详细日志可在「控制台」页随时开关，立即生效，不需要停止房间。
          </Typography>
          <Alert severity="info" variant="outlined">
            CubeDock 固定为直通模式 + 网易 RakNet v8：不翻译游戏包，由桥接代客户端完成登录重签与加密握手。
          </Alert>

          <Stack direction="row" justifyContent="flex-end">
            <Button type="submit" variant="contained" startIcon={<Save />} disabled={running || busy}>
              {busy ? '保存中…' : '保存设置'}
            </Button>
          </Stack>
        </Stack>
      </CardContent>
    </Card>
  );
}

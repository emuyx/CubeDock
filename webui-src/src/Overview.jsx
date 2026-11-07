import React, { useState } from 'react';
import {
  Alert,
  Box,
  Button,
  Card,
  CardContent,
  Chip,
  Divider,
  IconButton,
  Stack,
  Tooltip,
  Typography,
} from '@mui/material';
import {
  CloudQueue,
  ContentCopy,
  PeopleAlt,
  PlayArrow,
  RoomPreferences,
  Stop,
  Timer,
} from '@mui/icons-material';
import { api } from './api.js';
import { copyText, formatDuration, phaseColor, phaseLabel } from './util.js';
import { useToast } from './App.jsx';

function StatCard({ icon, label, value, helper, color = 'primary' }) {
  return (
    <Card sx={{ height: '100%' }}>
      <CardContent>
        <Stack direction="row" spacing={1.2} alignItems="center" sx={{ mb: 1.2 }}>
          <Box sx={{ color: `${color}.main`, display: 'flex' }}>{icon}</Box>
          <Typography variant="caption" color="text.secondary" fontWeight={750}>
            {label}
          </Typography>
        </Stack>
        <Typography variant="h5" fontWeight={850} sx={{ lineHeight: 1.15 }}>
          {value}
        </Typography>
        {helper && (
          <Typography variant="caption" color="text.secondary" sx={{ mt: 0.6, display: 'block' }}>
            {helper}
          </Typography>
        )}
      </CardContent>
    </Card>
  );
}

export default function Overview({ status, onChanged }) {
  const notify = useToast();
  const [busy, setBusy] = useState(false);
  const accounts = status.accounts || [];
  const rooms = status.rooms || [];
  const roomIds = (rooms.length ? rooms.map(room => room.room_id) : [status.room_id]).filter(Boolean);
  const multiplier = rooms.length > 1;

  const call = async (path, message) => {
    setBusy(true);
    try {
      await api(path, {});
      notify(message, 'success');
      onChanged?.();
    } catch (failure) {
      notify(failure.message || '操作失败', 'error');
    } finally {
      setBusy(false);
    }
  };

  const copyAll = async () => {
    if (!roomIds.length) {
      notify('当前还没有房间号', 'warning');
      return;
    }
    try {
      await copyText(roomIds.join('\n'));
      notify(multiplier ? '全部房间号已复制' : '房间号已复制', 'success');
    } catch (failure) {
      notify(failure.message || '复制失败', 'error');
    }
  };

  return (
    <Stack spacing={2.4}>
      {status.last_error && <Alert severity="warning">{status.last_error}</Alert>}

      <Box
        sx={{
          display: 'grid',
          gap: 2,
          gridTemplateColumns: { xs: '1fr', lg: 'minmax(0, 1.35fr) minmax(280px, .65fr)' },
          alignItems: 'stretch',
        }}
      >
        <Card
          sx={{
            background: theme =>
              `linear-gradient(135deg, ${theme.palette.primary.main}22, ${theme.palette.secondary.main}12)`,
          }}
        >
          <CardContent sx={{ p: { xs: 2.4, sm: 3.2 } }}>
            <Stack direction={{ xs: 'column', sm: 'row' }} spacing={2} justifyContent="space-between">
              <Box>
                <Stack direction="row" spacing={1} alignItems="center">
                  <RoomPreferences color="primary" />
                  <Typography variant="overline" color="text.secondary" fontWeight={800}>
                    本地联机房间
                  </Typography>
                </Stack>
                {roomIds.length === 0 ? (
                  <Typography variant="h3" sx={{ mt: 1.5, color: 'text.secondary' }}>
                    —
                  </Typography>
                ) : multiplier ? (
                  <Stack direction="row" spacing={1} flexWrap="wrap" useFlexGap sx={{ mt: 1.5 }}>
                    {roomIds.map((roomId, index) => (
                      <Chip
                        key={roomId}
                        label={`${index + 1} · ${roomId}`}
                        color="primary"
                        variant="outlined"
                        sx={{ fontSize: 16, py: 2.2, px: 1 }}
                      />
                    ))}
                  </Stack>
                ) : (
                  <Typography variant="h2" sx={{ mt: 1, fontFamily: 'ui-monospace, monospace' }}>
                    {roomIds[0]}
                  </Typography>
                )}
                <Typography variant="body2" color="text.secondary" sx={{ mt: 1 }}>
                  {multiplier
                    ? '网易玩家搜索任一房间号即可进入同一个世界。'
                    : roomIds.length
                      ? '在网易我的世界“本地联机”中搜索此房间号。'
                      : '启动房间后，房间号会显示在这里。'}
                </Typography>
              </Box>
              <Stack direction="row" spacing={1} alignItems="flex-start">
                <Tooltip title="复制房间号">
                  <span>
                    <IconButton onClick={copyAll} disabled={!roomIds.length} color="primary">
                      <ContentCopy />
                    </IconButton>
                  </span>
                </Tooltip>
              </Stack>
            </Stack>
            <Divider sx={{ my: 2.4 }} />
            <Stack direction={{ xs: 'column', sm: 'row' }} spacing={1.2}>
              <Button
                variant="contained"
                size="large"
                startIcon={<PlayArrow />}
                disabled={
                  busy || status.running ||
                  accounts.filter(account => account.email).length === 0
                }
                onClick={() => call('start', '房间正在启动')}
              >
                {status.running ? '房间运行中' : '启动房间'}
              </Button>
              <Button
                variant="outlined"
                size="large"
                color="inherit"
                startIcon={<Stop />}
                disabled={busy || !status.running}
                onClick={() => call('stop', '正在停止房间')}
              >
                停止房间
              </Button>
              <Chip
                sx={{ alignSelf: 'center' }}
                color={phaseColor(status.phase)}
                label={phaseLabel(status.phase)}
              />
            </Stack>
          </CardContent>
        </Card>

        <Card>
          <CardContent>
            <Stack direction="row" spacing={1} alignItems="center" sx={{ mb: 1 }}>
              <CloudQueue color={status.upstream_online ? 'success' : 'warning'} />
              <Typography variant="h6">网易协议上游</Typography>
            </Stack>
            <Chip
              color={status.upstream_online ? 'success' : 'warning'}
              label={status.upstream_online ? '在线' : '未连接'}
              size="small"
            />
            <Typography variant="body2" color="text.secondary" sx={{ mt: 1.5, wordBreak: 'break-all' }}>
              {status.config?.upstream || '—'}
            </Typography>
            <Typography variant="caption" color="text.secondary" display="block" sx={{ mt: 0.5 }}>
              {status.upstream_version ? `版本 ${status.upstream_version}` : '等待服务器响应'}
            </Typography>
            <Divider sx={{ my: 2 }} />
            <Typography variant="caption" color="text.secondary">
              网易玩家通过本地联机房间进入；其他版本客户端请连接上游服务端的对外地址。
            </Typography>
          </CardContent>
        </Card>
      </Box>

      <Box
        sx={{
          display: 'grid',
          gap: 2,
          gridTemplateColumns: { xs: '1fr 1fr', md: 'repeat(4, minmax(0, 1fr))' },
        }}
      >
        <StatCard icon={<PeopleAlt />} label="在线玩家" value={status.players ?? 0} helper="所有房间合计" />
        <StatCard
          icon={<RoomPreferences />}
          label="网易账号"
          value={accounts.filter(account => account.email).length}
          helper={`最多 8 个 · 已用 ${accounts.length}`}
          color="secondary"
        />
        <StatCard
          icon={<CloudQueue />}
          label="后端状态"
          value={status.upstream_online ? '在线' : '离线'}
          helper={status.upstream_version || '等待响应'}
          color={status.upstream_online ? 'success' : 'warning'}
        />
        <StatCard
          icon={<Timer />}
          label="在线时间"
          value={status.running ? formatDuration(status.connected_at) : '—'}
          helper={status.retry_at ? '等待自动重连' : '自动断线重连已启用'}
        />
      </Box>
    </Stack>
  );
}


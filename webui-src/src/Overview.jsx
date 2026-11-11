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
import PlayersDialog from './PlayersDialog.jsx';

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

export default function Overview({ status, onChanged, onOpenSetup }) {
  const notify = useToast();
  const [busy, setBusy] = useState(false);
  const [playersRoom, setPlayersRoom] = useState(null);
  const accounts = status.accounts || [];
  const rooms = status.rooms || [];
  const proxy = status.config?.connection_mode === 'proxy';
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

  const copyRoom = async roomId => {
    try {
      await copyText(String(roomId));
      notify('房间号已复制', 'success');
    } catch (failure) {
      notify(failure.message || '复制失败', 'error');
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

      {accounts.length === 0 && onOpenSetup && (
        <Card variant="outlined">
          <CardContent
            sx={{
              display: 'flex',
              gap: 2,
              flexDirection: { xs: 'column', sm: 'row' },
              alignItems: { xs: 'stretch', sm: 'center' },
              justifyContent: 'space-between',
            }}
          >
            <Box>
              <Typography fontWeight={800}>还没有完成首次设置？</Typography>
              <Typography variant="body2" color="text.secondary">
                按向导配置服务器、添加账号并运行预检。
              </Typography>
            </Box>
            <Button variant="contained" onClick={onOpenSetup}>
              打开首次设置
            </Button>
          </CardContent>
        </Card>
      )}

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
              <Typography variant="h6">服务器状态</Typography>
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
              网易玩家通过本地联机房间进入；其他版本客户端请直接连接服务器对外地址。
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
        <StatCard
          icon={<PeopleAlt />}
          label="在线玩家"
          value={proxy ? status.players ?? 0 : '—'}
          helper={proxy ? '所有房间合计' : '转服模式由目标服务器管理'}
        />
        <StatCard
          icon={<RoomPreferences />}
          label="网易账号"
          value={accounts.filter(account => account.email).length}
          helper={`一账号一房间 · 已用 ${accounts.length}`}
          color="secondary"
        />
        <StatCard
          icon={<CloudQueue />}
          label="服务器状态"
          value={status.upstream_online ? '已连接' : '未连接'}
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

      {rooms.length > 0 && (
        <Card>
          <CardContent>
            <Stack direction="row" justifyContent="space-between" alignItems="center">
              <Box>
                <Typography variant="h6">房间实例</Typography>
                <Typography variant="caption" color="text.secondary">
                  每个房间可以单独查看玩家、复制房间号。
                </Typography>
              </Box>
              <Chip size="small" variant="outlined" label={`${rooms.length} 个`} />
            </Stack>
            <Box
              sx={{
                display: 'grid',
                gap: 1.5,
                mt: 1.8,
                gridTemplateColumns: { xs: '1fr', md: 'repeat(2, minmax(0, 1fr))' },
              }}
            >
              {rooms.map((room, index) => (
                <Box
                  key={index}
                  sx={{ border: 1, borderColor: 'divider', borderRadius: 2, p: 1.6 }}
                >
                  <Stack direction="row" justifyContent="space-between" alignItems="flex-start">
                    <Box sx={{ minWidth: 0 }}>
                      <Typography fontWeight={800}>房间 {index + 1}</Typography>
                      <Typography variant="caption" color="text.secondary" noWrap>
                        {room.account || accounts[index]?.name || '—'}
                      </Typography>
                    </Box>
                    <Chip size="small" color={phaseColor(room.phase)} label={phaseLabel(room.phase)} />
                  </Stack>
                  <Stack direction="row" spacing={1} alignItems="center" sx={{ mt: 1 }}>
                    <Typography variant="body2" fontFamily="ui-monospace, monospace">
                      {room.room_id || '—'}
                    </Typography>
                    <Typography variant="caption" color="text.secondary">
                      {proxy ? `${room.players ?? 0} 人` : '转服模式'}
                    </Typography>
                  </Stack>
                  <Stack direction="row" spacing={0.5} sx={{ mt: 1 }}>
                    {proxy ? (
                      <Button
                        size="small"
                        startIcon={<PeopleAlt />}
                        onClick={() => setPlayersRoom(index)}
                      >
                        玩家
                      </Button>
                    ) : (
                      <Typography variant="caption" color="text.secondary" sx={{ alignSelf: 'center' }}>
                        玩家由目标服务器管理
                      </Typography>
                    )}
                    <Button
                      size="small"
                      startIcon={<ContentCopy />}
                      disabled={!room.room_id}
                      onClick={() => copyRoom(room.room_id)}
                    >
                      复制房间号
                    </Button>
                  </Stack>
                </Box>
              ))}
            </Box>
          </CardContent>
        </Card>
      )}

      <PlayersDialog
        open={playersRoom !== null}
        roomIndex={playersRoom}
        status={status}
        onClose={() => setPlayersRoom(null)}
      />
    </Stack>
  );
}


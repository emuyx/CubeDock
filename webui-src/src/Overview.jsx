import React, { useState } from 'react';
import {
  Alert,
  Box,
  Button,
  Divider,
  IconButton,
  Tooltip,
} from '@mui/material';
import Typography from './Typography.jsx';
import Stack from './Stack.jsx';
import { ContentCopy, PlayArrow, QrCode2, Stop } from '@mui/icons-material';
import { api } from './api.js';
import { copyText } from './util.js';
import { useToast } from './App.jsx';
import AccountAvatar from './AccountAvatar.jsx';
import LiveDuration from './LiveDuration.jsx';
import ShareRoomDialog from './ShareRoomDialog.jsx';
import PhaseDot from './StatusDot.jsx';

function Metric({ label, value, hint }) {
  return (
    <Box sx={{ minWidth: 92 }}>
      <Typography variant="caption" color="text.secondary" display="block">
        {label}
      </Typography>
      <Typography variant="h6" fontWeight={800} sx={{ lineHeight: 1.25 }}>
        {value}
      </Typography>
      {hint && (
        <Typography variant="caption" color="text.secondary" display="block" noWrap>
          {hint}
        </Typography>
      )}
    </Box>
  );
}

export default function Overview({ status, onChanged, onOpenSetup, onOpenAccount }) {
  const notify = useToast();
  const [busy, setBusy] = useState(false);
  const [share, setShare] = useState(null);
  const accounts = status.accounts || [];
  const rooms = status.rooms || [];
  const profiles = status.config?.profiles || [];
  const profileOf = index =>
    profiles.find(profile => profile.id === (accounts[index]?.profile_id || profiles[0]?.id)) || profiles[0];
  const activeRooms = rooms
    .map((room, index) => ({ room, index }))
    .filter(({ room }) => room.phase && room.phase !== 'stopped');
  const roomIds = activeRooms.map(({ room }) => room.room_id).filter(Boolean);
  const multiplier = activeRooms.length > 1;
  const allStarted = accounts.length > 0 && activeRooms.length === accounts.length;
  const allOnline = accounts.length > 0 && activeRooms.every(({ room }) => room.phase === 'online');

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

  return (
    <Stack spacing={{ xs: 2.2, sm: 3 }}>
      {status.last_error && <Alert severity="warning">{status.last_error}</Alert>}

      {accounts.length === 0 && onOpenSetup && (
        <Alert
          severity="info"
          action={
            <Button color="inherit" size="small" onClick={onOpenSetup}>
              打开首次设置
            </Button>
          }
        >
          还没有账号。按向导添加一个网易账号并运行预检。
        </Alert>
      )}

      <Box>
        <Stack
          direction={{ xs: 'column', md: 'row' }}
          spacing={{ xs: 2, md: 3 }}
          justifyContent="space-between"
          alignItems={{ xs: 'stretch', md: 'flex-start' }}
        >
          <Box sx={{ minWidth: 0 }}>
            <Stack direction="row" spacing={1} alignItems="center">
              <PhaseDot phase={status.phase} />
              <Typography variant="overline" color="text.secondary" fontWeight={800}>
                本地联机房间
              </Typography>
            </Stack>
            {roomIds.length === 0 ? (
              <Typography variant="h5" sx={{ mt: 1, color: 'text.disabled' }}>
                未启动
              </Typography>
            ) : (
              <Stack spacing={0.2} sx={{ mt: 0.5 }}>
                {roomIds.map((roomId, index) => (
                  <Stack key={`${roomId}-${index}`} direction="row" spacing={0.6} alignItems="center">
                    <Typography
                      variant={multiplier ? 'h4' : 'h2'}
                      fontFamily="ui-monospace, monospace"
                      sx={{ wordBreak: 'break-all', lineHeight: 1.15 }}
                    >
                      {roomId}
                    </Typography>
                    {multiplier && (
                      <Typography variant="caption" color="text.secondary" sx={{ ml: 0.5 }}>
                        房间 {index + 1}
                      </Typography>
                    )}
                    <Tooltip title="复制房间号">
                      <IconButton size="small" onClick={() => copyRoom(roomId)} aria-label="复制房间号">
                        <ContentCopy sx={{ fontSize: 16 }} />
                      </IconButton>
                    </Tooltip>
                  </Stack>
                ))}
              </Stack>
            )}
            <Typography variant="body2" color="text.secondary" sx={{ mt: 1 }}>
              {multiplier
                ? '玩家搜索任一房间号都可以进入同一个世界。'
                : roomIds.length
                  ? '在网易我的世界「本地联机」中搜索这个房间号。'
                  : '启动房间后，房间号会显示在这里。'}
            </Typography>
          </Box>

          <Stack
            direction={{ xs: 'column', sm: 'row' }}
            spacing={1.2}
            sx={{ flexShrink: 0, alignSelf: { xs: 'stretch', md: 'flex-start' } }}
          >
            <Button
              variant="contained"
              size="large"
              startIcon={<PlayArrow />}
              disabled={busy || allStarted || accounts.length === 0}
              onClick={() => call('start', '剩余房间正在启动')}
            >
              {allStarted
                ? allOnline ? '全部房间在线' : '全部房间已启动'
                : activeRooms.length ? '启动其余房间' : '启动全部房间'}
            </Button>
            <Button
              variant="outlined"
              size="large"
              color="inherit"
              startIcon={<Stop />}
              disabled={busy || activeRooms.length === 0}
              onClick={() => call('stop', '正在停止全部房间')}
            >
              停止全部房间
            </Button>
          </Stack>
        </Stack>

        <Divider sx={{ my: { xs: 2, sm: 2.5 } }} />

        <Stack direction="row" spacing={{ xs: 2.4, sm: 5 }} flexWrap="wrap" useFlexGap rowGap={1.6}>
          <Metric label="在线玩家" value={status.players ?? 0} hint="仅代理模式" />
          <Metric label="账号" value={accounts.length} hint={`${activeRooms.length} 个房间运行中`} />
          <Metric
            label="在线时间"
            value={<LiveDuration since={status.running ? status.connected_at : ''} />}
            hint={status.retry_at ? '等待自动重连' : undefined}
          />
        </Stack>
      </Box>

      {activeRooms.length > 0 && (
        <Box>
          <Stack direction="row" justifyContent="space-between" alignItems="baseline">
            <Typography variant="h6">运行中的房间</Typography>
            <Typography variant="caption" color="text.secondary">
              {activeRooms.length} 个
            </Typography>
          </Stack>
          <Box>
            {activeRooms.map(({ room, index }) => (
              <Box
                key={index}
                sx={{ py: 1.4, borderTop: 1, borderColor: 'divider' }}
              >
                <Stack
                  direction={{ xs: 'column', sm: 'row' }}
                  spacing={{ xs: 1, sm: 2 }}
                  justifyContent="space-between"
                  alignItems={{ xs: 'stretch', sm: 'center' }}
                >
                  <Box sx={{ minWidth: 0 }}>
                    <Stack direction="row" spacing={1} alignItems="center" flexWrap="wrap" useFlexGap>
                      <AccountAvatar account={accounts[index]} size={28} />
                      <PhaseDot phase={room.phase} />
                      <Typography fontWeight={750} noWrap>
                        房间 {index + 1}
                        <Box component="span" sx={{ color: 'text.secondary', fontWeight: 400 }}>
                          {' · '}
                          {room.account || accounts[index]?.name || accounts[index]?.label || '—'}
                        </Box>
                      </Typography>
                    </Stack>
                    <Stack direction="row" spacing={0.8} alignItems="center" sx={{ mt: 0.3 }} flexWrap="wrap" useFlexGap>
                      <Typography variant="caption" fontFamily="ui-monospace, monospace">
                        {room.room_id || '未开房'}
                      </Typography>
                      {room.room_id ? (
                        <Tooltip title="复制房间号">
                          <IconButton
                            size="small"
                            aria-label="复制房间号"
                            onClick={() => copyRoom(room.room_id)}
                            sx={{ p: 0.3 }}
                          >
                            <ContentCopy sx={{ fontSize: 14 }} />
                          </IconButton>
                        </Tooltip>
                      ) : null}
                      <Typography variant="caption" color="text.secondary">
                        {room.connection_mode === 'proxy' ? `${room.players ?? 0} 人` : '人数不可见'}
                      </Typography>
                      <Typography variant="caption" color="text.secondary" noWrap>
                        · {room.profile_name || '默认配置'}
                      </Typography>
                    </Stack>
                  </Box>
                  <Stack direction="row" spacing={0.5} alignItems="center" justifyContent="flex-end">
                    <Tooltip title="分享房间二维码">
                      <span>
                        <IconButton
                          size="small"
                          aria-label="分享房间二维码"
                          title="分享房间二维码"
                          disabled={!room.room_id || !accounts[index]?.uid}
                          onClick={() =>
                            setShare({
                              roomId: room.room_id,
                              ownerUid: accounts[index]?.uid,
                              roomName: profileOf(index)?.room_name,
                            })
                          }
                        >
                          <QrCode2 fontSize="small" />
                        </IconButton>
                      </span>
                    </Tooltip>
                    <Button size="small" onClick={() => onOpenAccount?.(index)}>
                      仪表盘
                    </Button>
                  </Stack>
                </Stack>
              </Box>
            ))}
          </Box>
        </Box>
      )}

      <ShareRoomDialog
        open={!!share}
        onClose={() => setShare(null)}
        roomId={share?.roomId}
        ownerUid={share?.ownerUid}
        roomName={share?.roomName}
      />
    </Stack>
  );
}

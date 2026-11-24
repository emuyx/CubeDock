import React, { useCallback, useEffect, useState } from 'react';
import {
  Alert,
  Box,
  Button,
  Card,
  CardContent,
  CircularProgress,
  IconButton,
  Tooltip,
} from '@mui/material';
import Typography from './Typography.jsx';
import Stack from './Stack.jsx';
import { Block, PersonOff, Refresh } from '@mui/icons-material';
import { api, roomKickPlayer } from './api.js';
import { useConfirm, useToast } from './App.jsx';
import { StatusDot } from './StatusDot.jsx';

const STATE = {
  connecting: { label: '连接中', color: 'info' },
  authenticating: { label: '登录中', color: 'info' },
  joined: { label: '在线', color: 'success' },
  login_failed: { label: '登录失败', color: 'error' },
  kicked: { label: '已断开', color: 'default' },
};

function stateMeta(state) {
  return STATE[state] || { label: state || '未知', color: 'default' };
}

function formatUptime(seconds) {
  const total = Math.max(0, Number(seconds) || 0);
  const hours = Math.floor(total / 3600);
  const minutes = Math.floor((total % 3600) / 60);
  if (hours > 0) return `${hours} 时 ${minutes} 分`;
  if (minutes > 0) return `${minutes} 分 ${Math.floor(total % 60)} 秒`;
  return `${Math.floor(total)} 秒`;
}

export default function RoomPlayersPanel({ roomIndex, proxy }) {
  const notify = useToast();
  const confirm = useConfirm();
  const [players, setPlayers] = useState([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [busy, setBusy] = useState('');

  const load = useCallback(async () => {
    if (!proxy || roomIndex === null || roomIndex === undefined) return;
    setLoading(true);
    try {
      const list = await api('players', undefined, 'GET');
      setPlayers((list || []).filter(player => player.room === roomIndex + 1));
      setError('');
    } catch (failure) {
      setError(failure.message || '读取玩家列表失败');
    } finally {
      setLoading(false);
    }
  }, [proxy, roomIndex]);

  useEffect(() => {
    if (!proxy) {
      setPlayers([]);
      setError('');
      return undefined;
    }
    load();
    const timer = setInterval(load, 2000);
    return () => clearInterval(timer);
  }, [load, proxy]);

  const kick = async player => {
    const confirmed = await confirm({
      title: '断开玩家',
      message: `确定断开 ${player.name || player.id} 吗？`,
      confirmLabel: '断开玩家',
      severity: 'warning',
    });
    if (!confirmed) return;
    setBusy(player.id);
    try {
      await api('player/kick', { target: player.id });
      notify(`已请求断开 ${player.name || player.id}`, 'success');
      load();
    } catch (failure) {
      notify(failure.message || '断开玩家失败', 'error');
    } finally {
      setBusy('');
    }
  };

  const roomKick = async player => {
    if (!player.room_player_id) {
      notify('该玩家还没有分配房间玩家 ID，暂时不能从房间踢出', 'warning');
      return;
    }
    const confirmed = await confirm({
      title: '从房间踢出玩家',
      message: `通过网易房间控制通道踢出 ${player.name || player.id}？`,
      confirmLabel: '房间踢出',
      severity: 'warning',
    });
    if (!confirmed) return;
    setBusy(player.id);
    try {
      await roomKickPlayer(player.id);
      notify(`已请求从房间踢出 ${player.name || player.id}`, 'success');
      load();
    } catch (failure) {
      notify(failure.message || '房间踢出失败', 'error');
    } finally {
      setBusy('');
    }
  };

  return (
    <Card>
      <CardContent>
        <Stack
          direction={{ xs: 'column', sm: 'row' }}
          justifyContent="space-between"
          alignItems={{ xs: 'stretch', sm: 'center' }}
          spacing={1}
        >
          <Box>
            <Typography variant="h6">在线玩家</Typography>
            <Typography variant="caption" color="text.secondary">
              仅本房间 · 每 2 秒刷新
            </Typography>
          </Box>
          {proxy && (
            <Stack direction="row" spacing={0.5} alignItems="center" justifyContent="flex-end">
              <Typography variant="caption" color="text.secondary">
                {players.length} 人
              </Typography>
              <Tooltip title="刷新玩家列表">
                <span>
                  <IconButton size="small" onClick={load} disabled={loading} aria-label="刷新玩家列表">
                    {loading ? <CircularProgress size={16} /> : <Refresh fontSize="small" />}
                  </IconButton>
                </span>
              </Tooltip>
            </Stack>
          )}
        </Stack>

        {!proxy ? (
          <Alert severity="info" sx={{ mt: 1.6 }}>
            转服模式下玩家由目标服务器管理，CubeDock 无法查看或管理玩家。
          </Alert>
        ) : (
          <Box sx={{ mt: 1 }}>
            {error && <Alert severity="error" sx={{ my: 1 }}>{error}</Alert>}
            {loading && players.length === 0 && (
              <Box sx={{ py: 3, display: 'grid', placeItems: 'center' }}>
                <CircularProgress size={22} />
              </Box>
            )}
            {!loading && players.length === 0 && !error && (
              <Typography variant="body2" color="text.secondary" sx={{ py: 2, textAlign: 'center' }}>
                当前没有在线玩家。
              </Typography>
            )}
            {players.map(player => {
              const meta = stateMeta(player.state);
              return (
                <Box
                  key={player.id}
                  sx={{ py: 1.2, borderTop: 1, borderColor: 'divider' }}
                >
                  <Stack
                    direction={{ xs: 'column', sm: 'row' }}
                    spacing={1}
                    justifyContent="space-between"
                    alignItems={{ xs: 'stretch', sm: 'center' }}
                  >
                    <Box sx={{ minWidth: 0 }}>
                      <Stack direction="row" spacing={0.8} alignItems="center">
                        <StatusDot color={meta.color} />
                        <Typography fontWeight={750} noWrap>
                          {player.name || player.id}
                        </Typography>
                        <Typography variant="caption" color="text.secondary">
                          {meta.label}
                        </Typography>
                      </Stack>
                      <Typography variant="caption" color="text.secondary" display="block" sx={{ mt: 0.2 }}>
                        在线 {formatUptime(player.uptime_seconds)} · 上行 {player.up_packets ?? 0} 包 /
                        下行 {player.down_packets ?? 0} 包
                      </Typography>
                    </Box>
                    <Stack direction="row" spacing={1} flexWrap="wrap" useFlexGap sx={{ flexShrink: 0 }}>
                      <Button
                        size="small"
                        color="warning"
                        startIcon={<Block fontSize="small" />}
                        disabled={!player.room_player_id || busy === player.id}
                        onClick={() => roomKick(player)}
                      >
                        房间踢出
                      </Button>
                      <Button
                        size="small"
                        color="error"
                        startIcon={<PersonOff fontSize="small" />}
                        disabled={busy === player.id}
                        onClick={() => kick(player)}
                      >
                        断开
                      </Button>
                    </Stack>
                  </Stack>
                </Box>
              );
            })}
          </Box>
        )}
      </CardContent>
    </Card>
  );
}

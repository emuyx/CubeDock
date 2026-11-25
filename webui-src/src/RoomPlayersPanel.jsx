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
import { api, getRoomMembers, kickRoomMember, roomKickPlayer } from './api.js';
import { useConfirm, useToast } from './App.jsx';
import LiveDuration from './LiveDuration.jsx';
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

function memberName(member) {
  return member.name || `房间玩家 ${member.room_player_id || member.id}`;
}

export default function RoomPlayersPanel({ roomIndex, proxy, phase = 'online', autoRefresh = true }) {
  const notify = useToast();
  const confirm = useConfirm();
  const [players, setPlayers] = useState([]);
  const [members, setMembers] = useState([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [busy, setBusy] = useState('');
  const running = phase !== 'stopped';

  const load = useCallback(async () => {
    if (!running || roomIndex === null || roomIndex === undefined) return;
    setLoading(true);
    try {
      const [memberResult, worldList] = await Promise.all([
        getRoomMembers(roomIndex),
        proxy ? api('players', undefined, 'GET') : Promise.resolve([]),
      ]);
      setMembers(
        Array.isArray(memberResult) ? memberResult : memberResult?.members || [],
      );
      setPlayers(
        proxy && Array.isArray(worldList)
          ? worldList.filter(player => player.room === roomIndex + 1)
          : [],
      );
      setError('');
    } catch (failure) {
      setError(failure.message || '读取房间玩家失败');
    } finally {
      setLoading(false);
    }
  }, [proxy, roomIndex, running]);

  useEffect(() => {
    if (!running) {
      setPlayers([]);
      setMembers([]);
      setError('');
      return undefined;
    }
    load();
    if (!autoRefresh) return undefined;
    const timer = setInterval(load, 2000);
    return () => clearInterval(timer);
  }, [load, running, autoRefresh]);

  const worldIDs = new Set(players.map(player => player.room_player_id).filter(Boolean));
  const waiting = members.filter(member => !member.in_world && !worldIDs.has(member.room_player_id));
  const worldFromMembers = members.filter(
    member => member.in_world && !worldIDs.has(member.room_player_id),
  );
  const worldCount = players.length + worldFromMembers.length;
  const roomCount = members.length;

  const kick = async player => {
    const confirmed = await confirm({
      title: '断开玩家',
      message: `确定断开 ${player.name || player.id} 吗？`,
      confirmLabel: '断开玩家',
      severity: 'warning',
    });
    if (!confirmed) return;
    setBusy(`world-${player.id}`);
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

  const roomKickWorld = async player => {
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
    setBusy(`world-${player.id}`);
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

  const roomKickMember = async member => {
    if (!member.room_player_id) {
      notify('该成员还没有房间玩家 ID', 'warning');
      return;
    }
    const confirmed = await confirm({
      title: '从房间踢出成员',
      message: `通过网易房间控制通道踢出 ${memberName(member)}？`,
      confirmLabel: '房间踢出',
      severity: 'warning',
    });
    if (!confirmed) return;
    setBusy(`member-${member.id}`);
    try {
      await kickRoomMember(roomIndex, member.room_player_id);
      notify(`已请求从房间踢出 ${memberName(member)}`, 'success');
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
            <Typography variant="h6">房间玩家</Typography>
            <Typography variant="caption" color="text.secondary">
              {running
                ? `${roomCount} 人在房间 · ${worldCount} 人已进入服务器`
                : '房间未运行'}
              {' · '}
              {autoRefresh ? '每 2 秒刷新' : '自动刷新已暂停'}
            </Typography>
          </Box>
          <Stack direction="row" spacing={0.5} alignItems="center" justifyContent="flex-end">
            <Tooltip title="刷新玩家列表">
              <span>
                <IconButton size="small" onClick={load} disabled={loading || !running} aria-label="刷新玩家列表">
                  {loading ? <CircularProgress size={16} /> : <Refresh fontSize="small" />}
                </IconButton>
              </span>
            </Tooltip>
          </Stack>
        </Stack>

        {!running ? (
          <Typography variant="body2" color="text.secondary" sx={{ py: 2, textAlign: 'center' }}>
            启动房间后，这里会区分「只加入房间」和「已进入服务器」的玩家。
          </Typography>
        ) : (
          <Box sx={{ mt: 1 }}>
            {error && <Alert severity="error" sx={{ my: 1 }}>{error}</Alert>}

            <Stack direction="row" spacing={1} alignItems="baseline" sx={{ mt: 1.2 }}>
              <Typography variant="subtitle2" fontWeight={800}>已进入服务器</Typography>
              <Typography variant="caption" color="text.secondary">{worldCount} 人</Typography>
            </Stack>
            {players.length === 0 && worldFromMembers.length === 0 && (
              <Typography variant="body2" color="text.secondary" sx={{ py: 1.2 }}>
                还没有玩家进入服务器。
              </Typography>
            )}
            {players.map(player => {
              const meta = stateMeta(player.state);
              return (
                <Box key={player.id} sx={{ py: 1.1, borderTop: 1, borderColor: 'divider' }}>
                  <Stack
                    direction={{ xs: 'column', sm: 'row' }}
                    spacing={1}
                    justifyContent="space-between"
                    alignItems={{ xs: 'stretch', sm: 'center' }}
                  >
                    <Box sx={{ minWidth: 0 }}>
                      <Stack direction="row" spacing={0.8} alignItems="center">
                        <StatusDot color={meta.color} />
                        <Typography fontWeight={750} noWrap>{player.name || player.id}</Typography>
                        <Typography variant="caption" color="text.secondary">{meta.label}</Typography>
                      </Stack>
                      <Typography variant="caption" color="text.secondary" display="block" sx={{ mt: 0.2 }}>
                        在线 {player.joined_at ? <LiveDuration since={player.joined_at} /> : formatUptime(player.uptime_seconds)}
                        {' · '}上行 {player.up_packets ?? 0} 包 / 下行 {player.down_packets ?? 0} 包
                      </Typography>
                    </Box>
                    <Stack direction="row" spacing={1} flexWrap="wrap" useFlexGap sx={{ flexShrink: 0 }}>
                      <Button
                        size="small"
                        color="warning"
                        startIcon={<Block fontSize="small" />}
                        disabled={!player.room_player_id || busy === `world-${player.id}`}
                        onClick={() => roomKickWorld(player)}
                      >
                        房间踢出
                      </Button>
                      <Button
                        size="small"
                        color="error"
                        startIcon={<PersonOff fontSize="small" />}
                        disabled={busy === `world-${player.id}`}
                        onClick={() => kick(player)}
                      >
                        断开
                      </Button>
                    </Stack>
                  </Stack>
                </Box>
              );
            })}
            {worldFromMembers.map(member => (
              <Box key={member.id} sx={{ py: 1.1, borderTop: 1, borderColor: 'divider' }}>
                <Stack direction="row" spacing={0.8} alignItems="center" justifyContent="space-between">
                  <Stack direction="row" spacing={0.8} alignItems="center" sx={{ minWidth: 0 }}>
                    <StatusDot color="success" />
                    <Typography fontWeight={750} noWrap>{memberName(member)}</Typography>
                    <Typography variant="caption" color="text.secondary">已进入世界</Typography>
                  </Stack>
                  <Button
                    size="small"
                    color="warning"
                    startIcon={<Block fontSize="small" />}
                    disabled={busy === `member-${member.id}`}
                    onClick={() => roomKickMember(member)}
                  >
                    房间踢出
                  </Button>
                </Stack>
              </Box>
            ))}

            <Stack direction="row" spacing={1} alignItems="baseline" sx={{ mt: 2 }}>
              <Typography variant="subtitle2" fontWeight={800}>仅加入房间</Typography>
              <Typography variant="caption" color="text.secondary">{waiting.length} 人</Typography>
            </Stack>
            {waiting.length === 0 ? (
              <Typography variant="body2" color="text.secondary" sx={{ py: 1.2 }}>
                没有停留在房间界面的玩家。
              </Typography>
            ) : (
              waiting.map(member => (
                <Box key={member.id} sx={{ py: 1.1, borderTop: 1, borderColor: 'divider' }}>
                  <Stack
                    direction={{ xs: 'column', sm: 'row' }}
                    spacing={1}
                    justifyContent="space-between"
                    alignItems={{ xs: 'stretch', sm: 'center' }}
                  >
                    <Box sx={{ minWidth: 0 }}>
                      <Stack direction="row" spacing={0.8} alignItems="center">
                        <StatusDot color="info" />
                        <Typography fontWeight={750} noWrap>{memberName(member)}</Typography>
                        <Typography variant="caption" color="text.secondary">等待进入服务器</Typography>
                      </Stack>
                      {member.joined_at && (
                        <Typography variant="caption" color="text.secondary" display="block" sx={{ mt: 0.2 }}>
                          已在房间 <LiveDuration since={member.joined_at} />
                        </Typography>
                      )}
                    </Box>
                    <Button
                      size="small"
                      color="warning"
                      startIcon={<Block fontSize="small" />}
                      disabled={busy === `member-${member.id}`}
                      onClick={() => roomKickMember(member)}
                    >
                      房间踢出
                    </Button>
                  </Stack>
                </Box>
              ))
            )}

            {!proxy && (
              <Typography variant="caption" color="text.secondary" display="block" sx={{ mt: 1.4 }}>
                转服模式下，进入服务器后的玩家由目标服务器管理；这里仍会显示停留在本地房间的玩家。
              </Typography>
            )}
          </Box>
        )}
      </CardContent>
    </Card>
  );
}

import React, { useCallback, useEffect, useState } from 'react';
import {
  Alert,
  Box,
  Button,
  Chip,
  CircularProgress,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  IconButton,
  Stack,
  Tab,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  Tabs,
  TextField,
  Tooltip,
  Typography,
  useMediaQuery,
  useTheme,
} from '@mui/material';
import { Block, ContentCopy, ExitToApp, PersonOff, Refresh } from '@mui/icons-material';
import { api, getRoomMembers, kickRoomMember, roomKickPlayer } from './api.js';
import { copyText } from './util.js';
import { useConfirm, useToast } from './App.jsx';

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

export default function PlayersDialog({ open, onClose, status, roomIndex = null, proxy: proxyOverride }) {
  const notify = useToast();
  const confirm = useConfirm();
  const theme = useTheme();
  const fullScreen = useMediaQuery(theme.breakpoints.down('sm'));
  const [players, setPlayers] = useState([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [busy, setBusy] = useState('');
  const [transfer, setTransfer] = useState(null);
  const [address, setAddress] = useState('');
  const [tab, setTab] = useState(0);
  const [members, setMembers] = useState([]);
  const [membersLoading, setMembersLoading] = useState(false);
  const [membersError, setMembersError] = useState('');
  const [memberBusy, setMemberBusy] = useState(null);
  // Player management is only meaningful in proxy mode. In transfer mode the
  // guest is handed to limbo/the target server and is no longer ours to manage.
  const resolvedMode =
    status?.rooms?.[roomIndex]?.connection_mode || status?.accounts?.[roomIndex]?.connection_mode;
  const proxy =
    proxyOverride !== undefined
      ? proxyOverride
      : resolvedMode
        ? resolvedMode === 'proxy'
        : true;

  const load = useCallback(async () => {
    setLoading(true);
    try {
      setPlayers(await api('players', undefined, 'GET'));
      setError('');
    } catch (failure) {
      setError(failure.message || '读取玩家列表失败');
    } finally {
      setLoading(false);
    }
  }, []);

  const loadMembers = useCallback(async () => {
    if (roomIndex === null) return;
    setMembersLoading(true);
    try {
      const result = await getRoomMembers(roomIndex);
      setMembers(result.members || []);
      setMembersError('');
    } catch (failure) {
      setMembersError(failure.message || '读取房间成员失败');
    } finally {
      setMembersLoading(false);
    }
  }, [roomIndex]);

  useEffect(() => {
    if (!open) return undefined;
    setTransfer(null);
    setAddress('');
    setTab(0);
    load();
    loadMembers();
    const timer = setInterval(() => {
      load();
      loadMembers();
    }, 2000);
    return () => clearInterval(timer);
  }, [open, load, loadMembers]);

  // PlayerStatus.Room is 1-based; roomIndex is the 0-based account index.
  const visible = players.filter(player => roomIndex === null || player.room === roomIndex + 1);

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
      message: `通过网易房间控制通道踢出 ${player.name || player.id}？该操作会由房间节点执行。`,
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

  const kickMember = async member => {
    const confirmed = await confirm({
      title: '从网易房间踢出',
      message: `通过房间控制通道踢出 ${member.name || member.room_player_id}？该玩家可能还没有进入世界。`,
      confirmLabel: '房间踢出',
      severity: 'warning',
    });
    if (!confirmed) return;
    setMemberBusy(member.room_player_id);
    try {
      await kickRoomMember(roomIndex, member.room_player_id);
      notify(`已请求从房间踢出 ${member.name || member.room_player_id}`, 'success');
      loadMembers();
      load();
    } catch (failure) {
      notify(failure.message || '房间踢出失败', 'error');
    } finally {
      setMemberBusy(null);
    }
  };

  const sendTransfer = async () => {
    if (!transfer) return;
    if (!address.trim()) {
      notify('请填写目标服务器地址', 'warning');
      return;
    }
    setBusy(transfer.id);
    try {
      await api('player/transfer', { target: transfer.id, address: address.trim() });
      notify(`已请求 ${transfer.name || transfer.id} 转服`, 'success');
      setTransfer(null);
      setAddress('');
      load();
    } catch (failure) {
      notify(failure.message || '发送转服失败', 'error');
    } finally {
      setBusy('');
    }
  };

  const copy = async text => {
    try {
      await copyText(String(text));
      notify('已复制', 'success');
    } catch (failure) {
      notify(failure.message || '复制失败', 'error');
    }
  };

  return (
    <Dialog open={open} onClose={onClose} fullWidth maxWidth="md" fullScreen={fullScreen}>
      <DialogTitle>
        {roomIndex === null ? '在线玩家' : `房间 ${roomIndex + 1} 的玩家`}
        {proxy && (
          <Typography variant="caption" color="text.secondary" sx={{ ml: 1 }}>
            {visible.length} 人
          </Typography>
        )}
      </DialogTitle>
      <DialogContent>
        <Stack spacing={1.6}>
          {roomIndex !== null && (
            <Tabs value={tab} onChange={(_, value) => setTab(value)} variant="scrollable" allowScrollButtonsMobile>
              <Tab label={`已进入世界（${visible.length}）`} />
              <Tab label={`网易房间成员（${members.length}）`} />
            </Tabs>
          )}
          {tab === 0 && (
            <>
          {!proxy && (
            <Alert severity="info">
              当前为转服模式。玩家会被送到目标服务器，CubeDock 不再持有玩家连接，
              因此这里不提供玩家列表、踢人或转服操作。请在目标服务器管理玩家。
            </Alert>
          )}
          {proxy && error && <Alert severity="error">{error}</Alert>}
          {transfer && (
            <Alert severity="warning">
              <Stack spacing={1}>
                <Typography variant="body2">
                  将 {transfer.name || transfer.id} 转服到其他服务器。这是实验功能，目标服务器仍需支持网易协议。
                </Typography>
                <Stack direction={{ xs: 'column', sm: 'row' }} spacing={1}>
                  <TextField
                    size="small"
                    label="目标地址"
                    placeholder="例如 127.0.0.1:19132"
                    value={address}
                    onChange={event => setAddress(event.target.value)}
                    fullWidth
                  />
                  <Button variant="contained" color="warning" onClick={sendTransfer} disabled={busy === transfer.id}>
                    发送转服
                  </Button>
                  <Button onClick={() => setTransfer(null)}>取消</Button>
                </Stack>
              </Stack>
            </Alert>
          )}
          {!proxy ? null : loading && !players.length ? (
            <Box sx={{ py: 5, display: 'grid', placeItems: 'center' }}>
              <CircularProgress />
            </Box>
          ) : visible.length === 0 ? (
            <Typography color="text.secondary" sx={{ py: 4, textAlign: 'center' }}>
              {roomIndex === null ? '当前没有在线玩家。' : '这个房间暂时没有玩家。'}
            </Typography>
          ) : (
            <TableContainer sx={{ maxHeight: 420 }}>
              <Table size="small" stickyHeader sx={{ minWidth: 680 }}>
                <TableHead>
                  <TableRow>
                    <TableCell>玩家</TableCell>
                    <TableCell>房间</TableCell>
                    <TableCell>状态</TableCell>
                    <TableCell>在线时长</TableCell>
                    <TableCell>流量</TableCell>
                    <TableCell align="right">操作</TableCell>
                  </TableRow>
                </TableHead>
                <TableBody>
                  {visible.map(player => {
                    const meta = stateMeta(player.state);
                    return (
                      <TableRow key={player.id} hover>
                        <TableCell>
                          <Stack direction="row" spacing={0.5} alignItems="center">
                            <Typography variant="body2" fontWeight={700}>
                              {player.name || player.id}
                            </Typography>
                            <Tooltip title="复制玩家 ID">
                              <IconButton size="small" aria-label="复制玩家 ID" onClick={() => copy(player.id)}>
                                <ContentCopy fontSize="inherit" />
                              </IconButton>
                            </Tooltip>
                          </Stack>
                          <Typography variant="caption" color="text.secondary">
                            {player.account || '未知账号'}
                          </Typography>
                        </TableCell>
                        <TableCell>
                          <Typography variant="body2">房间 {player.room}</Typography>
                          {player.room_id ? (
                            <Typography variant="caption" color="text.secondary">
                              {player.room_id}
                            </Typography>
                          ) : null}
                        </TableCell>
                        <TableCell>
                          <Chip size="small" color={meta.color} label={meta.label} />
                        </TableCell>
                        <TableCell>{formatUptime(player.uptime_seconds)}</TableCell>
                        <TableCell>
                          <Typography variant="caption" color="text.secondary">
                            ↑ {player.up_packets ?? 0} / ↓ {player.down_packets ?? 0}
                          </Typography>
                        </TableCell>
                        <TableCell align="right">
                          {proxy && player.state === 'joined' && (
                            <Tooltip title="转服到其他服务器（实验）">
                              <span>
                                <IconButton
                                  size="small"
                                  color="warning"
                                  aria-label="转服玩家"
                                  onClick={() => {
                                    setTransfer(player);
                                    setAddress('');
                                  }}
                                  disabled={busy === player.id}
                                >
                                  <ExitToApp fontSize="small" />
                                </IconButton>
                              </span>
                            </Tooltip>
                          )}
                          <Tooltip title="从房间踢出（网易房间管理）">
                            <span>
                              <IconButton
                                size="small"
                                color="warning"
                                aria-label="从房间踢出"
                                onClick={() => roomKick(player)}
                                disabled={busy === player.id || !player.room_player_id}
                              >
                                <Block fontSize="small" />
                              </IconButton>
                            </span>
                          </Tooltip>
                          <Tooltip title="断开玩家">
                            <span>
                              <IconButton
                                size="small"
                                color="error"
                                aria-label="断开玩家"
                                onClick={() => kick(player)}
                                disabled={busy === player.id}
                              >
                                <PersonOff fontSize="small" />
                              </IconButton>
                            </span>
                          </Tooltip>
                        </TableCell>
                      </TableRow>
                    );
                  })}
                </TableBody>
              </Table>
            </TableContainer>
          )}
            </>
          )}
          {tab === 1 && (
            <Stack spacing={1.4}>
              <Alert severity="info" variant="outlined">
                这里显示网易房间里的成员，包括已进房但还没进入世界的玩家；状态来自房间控制连接。
              </Alert>
              {membersError && <Alert severity="error">{membersError}</Alert>}
              {membersLoading && members.length === 0 ? (
                <Box sx={{ py: 5, display: 'grid', placeItems: 'center' }}>
                  <CircularProgress />
                </Box>
              ) : members.length === 0 ? (
                <Typography color="text.secondary" sx={{ py: 4, textAlign: 'center' }}>
                  当前没有网易房间成员。
                </Typography>
              ) : (
                <TableContainer sx={{ maxHeight: 420 }}>
                  <Table size="small" stickyHeader>
                    <TableHead>
                      <TableRow>
                        <TableCell>房间玩家</TableCell>
                        <TableCell>状态</TableCell>
                        <TableCell>加入时间</TableCell>
                        <TableCell align="right">操作</TableCell>
                      </TableRow>
                    </TableHead>
                    <TableBody>
                      {members.map(member => {
                        const status = member.in_world
                          ? { label: '已进入世界', color: 'success' }
                          : member.connected
                            ? { label: '已连接，等待进入世界', color: 'info' }
                            : { label: '在房间中', color: 'warning' };
                        return (
                          <TableRow key={member.id || member.room_player_id} hover>
                            <TableCell>
                              <Typography variant="body2" fontWeight={700}>
                                {member.name || `房间玩家 ${member.room_player_id}`}
                              </Typography>
                              <Typography variant="caption" color="text.secondary">
                                ID {member.room_player_id}
                              </Typography>
                            </TableCell>
                            <TableCell>
                              <Chip size="small" color={status.color} label={status.label} />
                            </TableCell>
                            <TableCell>{member.joined_at || '—'}</TableCell>
                            <TableCell align="right">
                              <Tooltip title="从网易房间踢出">
                                <span>
                                  <IconButton
                                    size="small"
                                    color="warning"
                                    aria-label="从网易房间踢出"
                                    onClick={() => kickMember(member)}
                                    disabled={memberBusy === member.room_player_id}
                                  >
                                    <Block fontSize="small" />
                                  </IconButton>
                                </span>
                              </Tooltip>
                            </TableCell>
                          </TableRow>
                        );
                      })}
                    </TableBody>
                  </Table>
                </TableContainer>
              )}
            </Stack>
          )}
        </Stack>
      </DialogContent>
      <DialogActions>
        <Button
          startIcon={<Refresh />}
          onClick={() => {
            load();
            loadMembers();
          }}
          disabled={loading || membersLoading}
        >
          {loading || membersLoading ? '刷新中…' : '刷新'}
        </Button>
        <Button onClick={onClose}>关闭</Button>
      </DialogActions>
    </Dialog>
  );
}

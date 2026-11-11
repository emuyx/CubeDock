import React, { useEffect, useState } from 'react';
import {
  Alert,
  Box,
  Button,
  Card,
  CardContent,
  Chip,
  CircularProgress,
  Divider,
  IconButton,
  InputAdornment,
  MenuItem,
  Stack,
  TextField,
  Tooltip,
  Typography,
} from '@mui/material';
import {
  ArrowBack,
  Check,
  CloudDownload,
  ContentCopy,
  Download,
  MonitorHeart,
  PeopleAlt,
  PersonAdd,
  PlayArrow,
  Refresh,
  RestartAlt,
  Search,
  Stop,
  Tune,
} from '@mui/icons-material';
import {
  acceptFriendRequest,
  assignProfile,
  getDiagnostics,
  getFriendRequests,
  getPacks,
  refreshPacks,
  restartRoom,
  startRoom,
  stopRoom,
} from './api.js';
import {
  copyText,
  downloadText,
  formatDuration,
  phaseColor,
  phaseLabel,
  timestamp,
} from './util.js';
import { useToast } from './App.jsx';
import PlayersDialog from './PlayersDialog.jsx';

const STATUS = {
  ok: { label: '通过', color: 'success' },
  warn: { label: '注意', color: 'warning' },
  fail: { label: '失败', color: 'error' },
  skip: { label: '跳过', color: 'default' },
};

function statusMeta(status) {
  return STATUS[status] || STATUS.skip;
}

function StatCard({ label, value, helper, color = 'text.primary' }) {
  return (
    <Card sx={{ height: '100%' }}>
      <CardContent sx={{ py: 1.6 }}>
        <Typography variant="caption" color="text.secondary" fontWeight={750}>
          {label}
        </Typography>
        <Typography variant="h6" fontWeight={850} color={color} sx={{ mt: 0.3, wordBreak: 'break-all' }}>
          {value}
        </Typography>
        {helper && (
          <Typography variant="caption" color="text.secondary" display="block" sx={{ mt: 0.4 }}>
            {helper}
          </Typography>
        )}
      </CardContent>
    </Card>
  );
}

export default function AccountDashboard({
  status,
  accountIndex,
  onChanged,
  onBack,
  onOpenProfiles,
}) {
  const notify = useToast();
  const account = status.accounts?.[accountIndex];
  const room = status.rooms?.[accountIndex] || { phase: 'stopped' };
  const profiles = status.config?.profiles || [];
  const profile =
    profiles.find(item => item.id === (account?.profile_id || status.config?.profiles?.[0]?.id)) ||
    profiles[0];
  const proxy = (room.connection_mode || account?.connection_mode || 'proxy') === 'proxy';
  const transfer = profile?.connection_mode === 'transfer';
  const phase = room.phase || 'stopped';
  const busyPhase = phase === 'starting' || phase === 'stopping';
  const [busy, setBusy] = useState('');
  const [assigning, setAssigning] = useState(false);
  const [playersOpen, setPlayersOpen] = useState(false);
  const [diagnostics, setDiagnostics] = useState(null);
  const [diagnosticsBusy, setDiagnosticsBusy] = useState(false);
  const [diagnosticsError, setDiagnosticsError] = useState('');
  const [packs, setPacks] = useState(null);
  const [packsBusy, setPacksBusy] = useState(false);
  const [packsError, setPacksError] = useState('');
  const [packsNotice, setPacksNotice] = useState('');
  const [friendRequests, setFriendRequests] = useState([]);
  const [friendsLoading, setFriendsLoading] = useState(false);
  const [friendsError, setFriendsError] = useState('');
  const [friendBusy, setFriendBusy] = useState('');
  const [query, setQuery] = useState('');

  const loadPacks = async () => {
    if (!profile || profile.connection_mode !== 'transfer') return;
    setPacksBusy(true);
    setPacksError('');
    try {
      setPacks(await getPacks(profile.id));
    } catch (failure) {
      setPacksError(failure.message || '读取资源包失败');
    } finally {
      setPacksBusy(false);
    }
  };

  useEffect(() => {
    setDiagnostics(null);
    setDiagnosticsError('');
    setPacks(null);
    setPacksError('');
    setPacksNotice('');
    if (profile?.connection_mode === 'transfer') loadPacks();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [profile?.id]);

  const copy = async text => {
    try {
      await copyText(String(text));
      notify('已复制', 'success');
    } catch (failure) {
      notify(failure.message || '复制失败', 'error');
    }
  };

  const changeProfile = async event => {
    const next = event.target.value;
    setAssigning(true);
    try {
      await assignProfile(accountIndex, next);
      notify('已切换配置方案', 'success');
      onChanged?.();
    } catch (failure) {
      notify(failure.message || '切换配置方案失败', 'error');
    } finally {
      setAssigning(false);
    }
  };

  const act = async (kind, action, message) => {
    setBusy(kind);
    try {
      await action(accountIndex);
      notify(message, 'success');
      onChanged?.();
    } catch (failure) {
      notify(failure.message || '操作失败', 'error');
    } finally {
      setBusy('');
    }
  };

  const loadFriends = async () => {
    if (phase === 'stopped' || phase === 'starting' || phase === 'authenticating' || phase === 'creating') {
      setFriendRequests([]);
      return;
    }
    setFriendsLoading(true);
    setFriendsError('');
    try {
      const result = await getFriendRequests(accountIndex);
      setFriendRequests(result.requests || []);
    } catch (failure) {
      setFriendsError(failure.message || '读取好友申请失败');
    } finally {
      setFriendsLoading(false);
    }
  };

  useEffect(() => {
    if (phase === 'online' || phase === 'upstream_offline') {
      loadFriends();
    } else {
      setFriendRequests([]);
      setFriendsError('');
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [accountIndex, phase]);

  const acceptFriend = async request => {
    setFriendBusy(request.uid);
    try {
      await acceptFriendRequest(accountIndex, request.uid);
      notify(`已同意 ${request.name || request.uid} 的好友申请`, 'success');
      loadFriends();
    } catch (failure) {
      notify(failure.message || '同意好友申请失败', 'error');
    } finally {
      setFriendBusy('');
    }
  };

  const loadDiagnostics = async () => {
    if (!profile) return;
    setDiagnosticsBusy(true);
    setDiagnosticsError('');
    try {
      setDiagnostics(await getDiagnostics(profile.id));
    } catch (failure) {
      setDiagnosticsError(failure.message || '运行诊断失败');
    } finally {
      setDiagnosticsBusy(false);
    }
  };

  const pullPacks = async () => {
    if (!profile) return;
    setPacksBusy(true);
    setPacksError('');
    setPacksNotice('');
    try {
      const result = await refreshPacks(profile.id);
      setPacks(result);
      setPacksNotice(`已拉取 ${result.pulled || 0} 个资源包，跳过 ${result.skipped || 0} 个。`);
      if (result.warnings?.length) setPacksError(result.warnings.join('；'));
    } catch (failure) {
      setPacksError(failure.message || '拉取资源包失败');
    } finally {
      setPacksBusy(false);
    }
  };

  const events = (status.events || []).filter(event => !event.room || event.room === accountIndex + 1);
  const needle = query.trim().toLowerCase();
  const visibleEvents = needle
    ? events.filter(event =>
        `${event.time} ${event.source} ${event.message}`.toLowerCase().includes(needle),
      )
    : events;

  const exportLogs = () => {
    if (!visibleEvents.length) {
      notify('当前没有日志可导出', 'warning');
      return;
    }
    const text = visibleEvents
      .map(event => `${event.time} [${event.source || '系统'}] ${event.message}`)
      .join('\n');
    downloadText(`cubedock-room-${accountIndex + 1}-${timestamp()}.txt`, text);
    notify('日志已导出', 'success');
  };

  const summary = diagnostics?.summary;
  const checks = diagnostics?.checks || [];

  return (
    <Stack spacing={2.2}>
      <Stack spacing={1}>
        <Stack direction="row" spacing={1} alignItems="center">
          <IconButton
            size="small"
            aria-label="返回"
            onClick={onBack}
            sx={{ display: { xs: 'inline-flex', sm: 'none' }, ml: -0.8 }}
          >
            <ArrowBack />
          </IconButton>
          <Button
            startIcon={<ArrowBack />}
            onClick={onBack}
            color="inherit"
            sx={{ display: { xs: 'none', sm: 'inline-flex' }, flexShrink: 0 }}
          >
            返回
          </Button>
          <Typography variant="h5" fontWeight={850} sx={{ minWidth: 0, flexGrow: 1 }} noWrap>
            {account?.name || account?.label || `房间 ${accountIndex + 1}`} 的仪表盘
          </Typography>
        </Stack>
        <Stack direction="row" spacing={1} flexWrap="wrap" useFlexGap>
          <Chip size="small" color={phaseColor(phase)} label={phaseLabel(phase)} />
          {profile && (
            <Chip
              size="small"
              variant="outlined"
              color={transfer ? 'secondary' : 'primary'}
              icon={<Tune fontSize="small" />}
              label={profile.name}
            />
          )}
        </Stack>
      </Stack>

      <Box
        sx={{
          display: 'grid',
          gap: 2,
          gridTemplateColumns: { xs: '1fr', lg: 'minmax(0, 1.2fr) minmax(300px, .8fr)' },
        }}
      >
        <Card>
          <CardContent sx={{ p: { xs: 2.2, sm: 3 } }}>
            <Stack direction={{ xs: 'column', sm: 'row' }} spacing={2} justifyContent="space-between">
              <Box sx={{ minWidth: 0 }}>
                <Typography variant="overline" color="text.secondary" fontWeight={800}>
                  本地联机房间
                </Typography>
                <Stack direction="row" spacing={1} alignItems="center" sx={{ mt: 0.4 }}>
                  <Typography variant="h3" fontFamily="ui-monospace, monospace" sx={{ wordBreak: 'break-all' }}>
                    {room.room_id || '—'}
                  </Typography>
                  <Tooltip title="复制房间号">
                    <span>
                      <IconButton size="small" disabled={!room.room_id} onClick={() => copy(room.room_id)}>
                        <ContentCopy />
                      </IconButton>
                    </span>
                  </Tooltip>
                </Stack>
                <Typography variant="body2" color="text.secondary" sx={{ mt: 0.8 }}>
                  {profile?.room_name || '未设置房间名'}
                </Typography>
              </Box>
              <Stack direction="row" spacing={1} alignItems="flex-start" flexWrap="wrap" useFlexGap>
                <Button
                  variant="contained"
                  startIcon={<PlayArrow />}
                  disabled={!!busy || busyPhase || phase !== 'stopped'}
                  onClick={() => act('start', startRoom, '房间正在启动')}
                >
                  启动
                </Button>
                <Button
                  variant="outlined"
                  color="inherit"
                  startIcon={<Stop />}
                  disabled={!!busy || busyPhase || phase === 'stopped'}
                  onClick={() => act('stop', stopRoom, '正在停止房间')}
                >
                  停止
                </Button>
                <Button
                  variant="outlined"
                  color="warning"
                  startIcon={<RestartAlt />}
                  disabled={!!busy || busyPhase || phase === 'stopped'}
                  onClick={() => act('restart', restartRoom, '正在重连房间')}
                >
                  重连
                </Button>
              </Stack>
            </Stack>
            <Divider sx={{ my: 2 }} />
            <Box sx={{ display: 'grid', gap: 1.4, gridTemplateColumns: { xs: '1fr 1fr', md: 'repeat(4, minmax(0, 1fr))' } }}>
              <StatCard
                label="在线玩家"
                value={proxy ? room.players ?? 0 : '—'}
                helper={proxy ? '该房间合计' : '由目标服务器管理'}
              />
              <StatCard
                label="在线时间"
                value={room.connected_at ? formatDuration(room.connected_at) : '—'}
                helper={room.retry_at ? '等待自动重连' : '断线自动重连已启用'}
              />
              <StatCard
                label="连接方式"
                value={transfer ? '转服' : '代理'}
                helper={profile?.upstream || '未设置服务器'}
              />
              <StatCard
                label="服务器"
                value={room.upstream_online ? '已连接' : phase === 'stopped' ? '—' : '未连接'}
                helper={room.upstream_version || profile?.upstream || '—'}
                color={room.upstream_online ? 'success.main' : 'text.primary'}
              />
            </Box>
            {room.retry_at && (
              <Alert severity="warning" sx={{ mt: 1.6 }}>
                房间正在等待重连（{new Date(room.retry_at).toLocaleTimeString('zh-CN', { hour12: false })}）。
              </Alert>
            )}
            {room.last_error && (
              <Alert severity="error" sx={{ mt: 1.6 }}>{room.last_error}</Alert>
            )}
            <Divider sx={{ my: 2 }} />
            <Stack spacing={1}>
              <TextField
                select
                size="small"
                label="该账号使用的配置方案"
                value={account?.profile_id || profile?.id || ''}
                onChange={changeProfile}
                disabled={status.running || assigning || profiles.length === 0}
                sx={{ maxWidth: 360 }}
                helperText={
                  status.running
                    ? '需要先停止全部房间才能切换方案'
                    : '切换后下次启动该账号的房间时生效'
                }
              >
                {profiles.map(item => (
                  <MenuItem key={item.id} value={item.id}>
                    {item.name}（{item.connection_mode === 'transfer' ? '转服' : '代理'} · {item.upstream}）
                  </MenuItem>
                ))}
              </TextField>
              <Typography variant="caption" color="text.secondary">
                {profile?.name || '—'}
                {transfer
                  ? ` · 中转 ${profile?.limbo_upstream || '—'} → ${profile?.upstream || '—'}`
                  : ` · 服务器 ${profile?.upstream || '—'}`}
              </Typography>
              <Stack direction="row" spacing={1} flexWrap="wrap" useFlexGap>
                <Button size="small" startIcon={<Tune />} onClick={onOpenProfiles}>
                  编辑配置方案
                </Button>
                <Button
                  size="small"
                  startIcon={<PeopleAlt />}
                  disabled={!proxy}
                  onClick={() => setPlayersOpen(true)}
                >
                  管理玩家
                </Button>
                <Button size="small" startIcon={<Refresh />} onClick={onChanged}>
                  刷新状态
                </Button>
              </Stack>
            </Stack>
          </CardContent>
        </Card>

        <Stack spacing={2}>
          <Card>
            <CardContent>
              <Stack direction="row" justifyContent="space-between" alignItems="center">
                <Typography variant="h6">资源包</Typography>
                {profile && (
                  <Chip size="small" variant="outlined" label={profile.name} />
                )}
              </Stack>
              {transfer ? (
                <>
                  <Typography variant="body2" color="text.secondary" sx={{ mt: 0.8, wordBreak: 'break-all' }}>
                    {packs?.dir || profile?.limbo_packs_dir || '保存方案后自动创建目录'}
                  </Typography>
                  <Typography variant="caption" color="text.secondary">
                    {packs ? `${packs.packs?.length || 0} 个资源包` : '尚未读取'}
                  </Typography>
                  {packsError && <Alert severity="error" sx={{ mt: 1 }}>{packsError}</Alert>}
                  {packsNotice && <Alert severity="success" sx={{ mt: 1 }}>{packsNotice}</Alert>}
                  <Stack direction="row" spacing={1} sx={{ mt: 1.2 }} flexWrap="wrap" useFlexGap>
                    <Button
                      size="small"
                      variant="contained"
                      startIcon={<CloudDownload />}
                      onClick={pullPacks}
                      disabled={packsBusy}
                    >
                      {packsBusy ? '处理中…' : '从服务器拉取'}
                    </Button>
                    <Button size="small" variant="outlined" startIcon={<Refresh />} onClick={loadPacks} disabled={packsBusy}>
                      刷新列表
                    </Button>
                  </Stack>
                </>
              ) : (
                <Typography variant="body2" color="text.secondary" sx={{ mt: 0.8 }}>
                  代理模式不使用本地资源包；资源包由服务器直接下发给玩家。
                </Typography>
              )}
            </CardContent>
          </Card>

          <Card>
            <CardContent>
              <Stack direction="row" justifyContent="space-between" alignItems="center">
                <Typography variant="h6">好友申请</Typography>
                <Button
                  size="small"
                  startIcon={friendsLoading ? <CircularProgress size={14} /> : <Refresh />}
                  onClick={loadFriends}
                  disabled={friendsLoading || phase === 'stopped'}
                >
                  刷新
                </Button>
              </Stack>
              {phase === 'stopped' || phase === 'starting' || phase === 'authenticating' || phase === 'creating' ? (
                <Typography variant="body2" color="text.secondary" sx={{ mt: 0.8 }}>
                  房间运行后可以在这里读取并手动同意好友申请。
                </Typography>
              ) : (
                <>
                  {friendsError && <Alert severity="error" sx={{ mt: 1 }}>{friendsError}</Alert>}
                  {friendRequests.length === 0 && !friendsError && (
                    <Typography variant="body2" color="text.secondary" sx={{ mt: 0.8 }}>
                      {friendsLoading ? '读取中…' : '没有待处理的好友申请。'}
                    </Typography>
                  )}
                  {friendRequests.length > 0 && (
                    <Stack spacing={0.8} sx={{ mt: 1 }}>
                      {friendRequests.map(request => (
                        <Stack
                          key={request.uid}
                          direction="row"
                          spacing={1}
                          alignItems="center"
                          justifyContent="space-between"
                          sx={{ p: 0.9, borderRadius: 1.5, bgcolor: 'action.hover' }}
                        >
                          <Stack direction="row" spacing={1} alignItems="center" sx={{ minWidth: 0 }}>
                            <PersonAdd fontSize="small" color="primary" />
                            <Box sx={{ minWidth: 0 }}>
                              <Typography variant="body2" fontWeight={700} noWrap>
                                {request.name || '未知玩家'}
                              </Typography>
                              {request.apply_time && (
                                <Typography variant="caption" color="text.secondary" noWrap display="block">
                                  申请时间 {String(request.apply_time)}
                                </Typography>
                              )}
                            </Box>
                          </Stack>
                          <Button
                            size="small"
                            variant="outlined"
                            startIcon={<Check />}
                            onClick={() => acceptFriend(request)}
                            disabled={friendBusy === request.uid}
                          >
                            同意
                          </Button>
                        </Stack>
                      ))}
                    </Stack>
                  )}
                </>
              )}
            </CardContent>
          </Card>

          <Card>
            <CardContent>
              <Stack direction="row" justifyContent="space-between" alignItems="center">
                <Typography variant="h6">诊断</Typography>
                <Button
                  size="small"
                  startIcon={diagnosticsBusy ? <CircularProgress size={14} /> : <MonitorHeart />}
                  onClick={loadDiagnostics}
                  disabled={diagnosticsBusy || !profile}
                >
                  {diagnostics ? '重新检测' : '检测该方案'}
                </Button>
              </Stack>
              {diagnosticsError && <Alert severity="error" sx={{ mt: 1 }}>{diagnosticsError}</Alert>}
              {summary && (
                <Stack direction="row" spacing={0.8} flexWrap="wrap" useFlexGap sx={{ mt: 1 }}>
                  <Chip size="small" color="success" variant="outlined" label={`通过 ${summary.ok || 0}`} />
                  <Chip size="small" color="warning" variant="outlined" label={`注意 ${summary.warn || 0}`} />
                  <Chip size="small" color="error" variant="outlined" label={`失败 ${summary.fail || 0}`} />
                  <Chip size="small" variant="outlined" label={`跳过 ${summary.skip || 0}`} />
                </Stack>
              )}
              {checks.length > 0 && (
                <Stack spacing={0.8} sx={{ mt: 1.2, maxHeight: 220, overflowY: 'auto' }}>
                  {checks.map(check => {
                    const meta = statusMeta(check.status);
                    return (
                      <Box key={check.id} sx={{ p: 0.8, borderRadius: 1, bgcolor: 'action.hover' }}>
                        <Stack direction="row" spacing={1} alignItems="center" flexWrap="wrap" useFlexGap>
                          <Typography variant="body2" fontWeight={700}>{check.label}</Typography>
                          <Chip size="small" color={meta.color} variant="outlined" label={meta.label} />
                        </Stack>
                        {check.detail && (
                          <Typography variant="caption" color="text.secondary" display="block" sx={{ wordBreak: 'break-all' }}>
                            {check.detail}
                          </Typography>
                        )}
                      </Box>
                    );
                  })}
                </Stack>
              )}
              {!diagnostics && !diagnosticsBusy && !diagnosticsError && (
                <Typography variant="body2" color="text.secondary" sx={{ mt: 0.8 }}>
                  检测该账号所用方案的配置、服务器、中转房间和资源包；不会登录网易账号。
                </Typography>
              )}
            </CardContent>
          </Card>
        </Stack>
      </Box>

      <Card>
        <CardContent>
          <Stack direction={{ xs: 'column', sm: 'row' }} spacing={1.5} justifyContent="space-between" alignItems={{ xs: 'stretch', sm: 'center' }}>
            <Box>
              <Typography variant="h6">房间日志</Typography>
              <Typography variant="caption" color="text.secondary">
                只显示该房间和系统级事件；完整日志在「控制台」页。
              </Typography>
            </Box>
            <Stack direction="row" spacing={1} flexWrap="wrap" useFlexGap>
              <TextField
                size="small"
                placeholder="搜索日志"
                value={query}
                onChange={event => setQuery(event.target.value)}
                slotProps={{
                  input: {
                    startAdornment: (
                      <InputAdornment position="start">
                        <Search fontSize="small" />
                      </InputAdornment>
                    ),
                  },
                }}
                sx={{ minWidth: 220 }}
              />
              <Button variant="outlined" startIcon={<Download />} onClick={exportLogs}>
                导出日志
              </Button>
            </Stack>
          </Stack>
          <Box
            sx={{
              mt: 1.6,
              p: 1.4,
              minHeight: 180,
              maxHeight: 360,
              overflowY: 'auto',
              borderRadius: 2,
              bgcolor: 'action.hover',
              fontFamily: 'ui-monospace, SFMono-Regular, Menlo, Consolas, monospace',
              fontSize: 12.5,
              lineHeight: 1.7,
            }}
          >
            {visibleEvents.length === 0 ? (
              <Typography color="text.secondary" sx={{ py: 4, textAlign: 'center' }}>
                暂无该房间的日志
              </Typography>
            ) : (
              visibleEvents.map((event, index) => (
                <Box key={`${event.time}-${index}`} sx={{ display: 'flex', gap: 1, alignItems: 'flex-start' }}>
                  <Box component="span" sx={{ color: 'text.disabled', flexShrink: 0 }}>{event.time}</Box>
                  <Box component="span" sx={{ color: 'secondary.main', flexShrink: 0, minWidth: 64, fontWeight: 700 }}>
                    [{event.source || '系统'}]
                  </Box>
                  <Box component="span" sx={{ wordBreak: 'break-word' }}>{event.message}</Box>
                </Box>
              ))
            )}
          </Box>
        </CardContent>
      </Card>

      <PlayersDialog
        open={playersOpen}
        roomIndex={accountIndex}
        status={status}
        proxy={proxy}
        onClose={() => setPlayersOpen(false)}
      />
    </Stack>
  );
}

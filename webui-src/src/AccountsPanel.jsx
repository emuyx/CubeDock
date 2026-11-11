import React, { useState } from 'react';
import {
  Alert,
  Box,
  Button,
  Card,
  CardContent,
  Chip,
  Dialog,
  DialogContent,
  DialogTitle,
  IconButton,
  InputAdornment,
  MenuItem,
  Stack,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  TextField,
  Tooltip,
  Typography,
  useMediaQuery,
  useTheme,
} from '@mui/material';
import {
  Add,
  Close,
  ContentCopy,
  Dashboard,
  DeleteOutlined,
  Key,
  PeopleAlt,
  Search,
  UploadFile,
} from '@mui/icons-material';
import { api, assignProfile } from './api.js';
import { copyText, phaseColor, phaseLabel } from './util.js';
import { useConfirm, useToast } from './App.jsx';
import AccountLogin from './AccountLogin.jsx';
import BulkImportDialog from './BulkImportDialog.jsx';
import PlayersDialog from './PlayersDialog.jsx';

export default function AccountsPanel({ status, onChanged, onOpenDashboard }) {
  const notify = useToast();
  const confirm = useConfirm();
  const theme = useTheme();
  const mobile = useMediaQuery(theme.breakpoints.down('md'));
  const phone = useMediaQuery(theme.breakpoints.down('sm'));
  const [addOpen, setAddOpen] = useState(false);
  const [bulkOpen, setBulkOpen] = useState(false);
  const [playersRoom, setPlayersRoom] = useState(null);
  const [reloginIndex, setReloginIndex] = useState(null);
  const [reloginEmail, setReloginEmail] = useState('');
  const [busy, setBusy] = useState(false);
  const [query, setQuery] = useState('');
  const [filter, setFilter] = useState('all');
  const [sort, setSort] = useState('room');
  const accounts = status.accounts || [];
  const rooms = status.rooms || [];
  const profiles = status.config?.profiles || [];
  const defaultProfileID = profiles[0]?.id || 'default';
  const running = !!status.running;
  const canAdd = !running;
  const profileIDOf = account => account.profile_id || defaultProfileID;
  const profileOf = account =>
    profiles.find(profile => profile.id === profileIDOf(account)) || profiles[0];
  const isProxy = account =>
    (rooms[account.index]?.connection_mode || account.connection_mode || 'proxy') === 'proxy';

  const phaseOf = account => rooms[account.index]?.phase || 'stopped';
  const expired = account => account.expires_at && new Date(account.expires_at) < new Date();
  const hasError = account =>
    Boolean(account.profile_error || rooms[account.index]?.last_error || expired(account));
  const needle = query.trim().toLowerCase();
  const visible = accounts
    .filter(account => {
      if (needle) {
        const haystack = [
          account.name,
          account.email,
          account.label,
          account.uid,
          rooms[account.index]?.room_id,
        ]
          .filter(Boolean)
          .join(' ')
          .toLowerCase();
        if (!haystack.includes(needle)) return false;
      }
      const phase = phaseOf(account);
      switch (filter) {
        case 'online':
          return phase === 'online';
        case 'error':
          return hasError(account);
        case 'stopped':
          return phase === 'stopped' && !hasError(account);
        default:
          return true;
      }
    })
    .sort((left, right) => {
      if (sort === 'name') {
        return (left.name || left.label || left.email || '').localeCompare(
          right.name || right.label || right.email || '',
          'zh-CN',
        );
      }
      if (sort === 'expiry') {
        const expiry = account =>
          account.expires_at ? new Date(account.expires_at).getTime() : Number.MAX_SAFE_INTEGER;
        return expiry(left) - expiry(right);
      }
      return left.index - right.index;
    });

  const assign = async (index, profileId) => {
    setBusy(true);
    try {
      await assignProfile(index, profileId);
      notify('配置方案已切换', 'success');
      onChanged?.();
    } catch (failure) {
      notify(failure.message || '切换配置方案失败', 'error');
    } finally {
      setBusy(false);
    }
  };

  const remove = async index => {
    if (running) {
      notify('请先停止房间，再移除账号', 'warning');
      return;
    }
    const confirmed = await confirm({
      title: '移除网易账号',
      message: `移除房间 ${index + 1} 的网易账号？该房间保存的登录凭证也会删除。`,
      confirmLabel: '移除账号',
      severity: 'error',
    });
    if (!confirmed) return;
    setBusy(true);
    try {
      await api('account/remove', { index });
      notify('账号已移除', 'success');
      onChanged?.();
    } catch (failure) {
      notify(failure.message || '移除账号失败', 'error');
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
    <Stack spacing={2}>
      <Stack spacing={1.5}>
        <Stack
          direction={{ xs: 'column', md: 'row' }}
          justifyContent="space-between"
          alignItems={{ xs: 'stretch', md: 'center' }}
          spacing={1.5}
        >
          <Box>
            <Typography variant="h6">网易账号与房间</Typography>
            <Typography variant="body2" color="text.secondary">
              每个账号对应一个房间；同一账号同时开两个房间会互相挤下线。账号数量不设固定上限。
            </Typography>
          </Box>
          <Stack direction="row" spacing={1} flexWrap="wrap" useFlexGap>
            <Button
              variant="outlined"
              startIcon={<UploadFile />}
              disabled={!canAdd}
              onClick={() => setBulkOpen(true)}
            >
              批量导入
            </Button>
            <Tooltip title={running ? '请先停止房间' : ''}>
              <span>
                <Button variant="contained" startIcon={<Add />} disabled={!canAdd} onClick={() => setAddOpen(true)}>
                  添加网易账号
                </Button>
              </span>
            </Tooltip>
          </Stack>
        </Stack>
        <Stack direction={{ xs: 'column', sm: 'row' }} spacing={1} alignItems={{ xs: 'stretch', sm: 'center' }}>
          <TextField
            size="small"
            placeholder="搜索游戏名 / 邮箱 / UID / 房间号"
            value={query}
            onChange={event => setQuery(event.target.value)}
            sx={{ flexGrow: 1, minWidth: 220 }}
            slotProps={{
              input: {
                startAdornment: (
                  <InputAdornment position="start">
                    <Search fontSize="small" />
                  </InputAdornment>
                ),
              },
            }}
          />
          <TextField
            size="small"
            select
            label="状态"
            value={filter}
            onChange={event => setFilter(event.target.value)}
            sx={{ width: { xs: '100%', sm: 130 } }}
          >
            <MenuItem value="all">全部</MenuItem>
            <MenuItem value="online">在线</MenuItem>
            <MenuItem value="error">异常</MenuItem>
            <MenuItem value="stopped">未运行</MenuItem>
          </TextField>
          <TextField
            size="small"
            select
            label="排序"
            value={sort}
            onChange={event => setSort(event.target.value)}
            sx={{ width: { xs: '100%', sm: 140 } }}
          >
            <MenuItem value="room">按房间</MenuItem>
            <MenuItem value="name">按游戏名</MenuItem>
            <MenuItem value="expiry">按到期时间</MenuItem>
          </TextField>
          <Typography variant="caption" color="text.secondary" sx={{ whiteSpace: 'nowrap' }}>
            显示 {visible.length} / {accounts.length}
          </Typography>
        </Stack>
      </Stack>

      {running && <Alert severity="info">房间运行中，停止后才能添加或移除账号。</Alert>}

      {mobile ? (
        <Stack spacing={1.5}>
          {visible.length === 0 && (
            <Card variant="outlined">
              <CardContent sx={{ py: 5, textAlign: 'center' }}>
                <Typography color="text.secondary">
                  {accounts.length ? '没有匹配的账号，请调整搜索或筛选条件。' : '还没有网易账号，请先登录一个账号。'}
                </Typography>
              </CardContent>
            </Card>
          )}
          {visible.map(account => {
            const room = rooms[account.index];
            const phase = room?.phase || 'stopped';
            const sessionExpired = account.expires_at && new Date(account.expires_at) < new Date();
            return (
              <Card key={account.index} variant="outlined">
                <CardContent sx={{ p: 1.7, '&:last-child': { pb: 1.7 } }}>
                  <Stack direction="row" justifyContent="space-between" alignItems="flex-start" spacing={1}>
                    <Box sx={{ minWidth: 0 }}>
                      <Typography fontWeight={800}>房间 {account.index + 1}</Typography>
                      <Typography variant="body2" color="text.secondary" noWrap>
                        {account.name || account.email || account.label || '未命名'}
                      </Typography>
                    </Box>
                    <Chip size="small" color={phaseColor(phase)} label={phaseLabel(phase)} />
                  </Stack>

                  <Stack direction="row" spacing={0.6} flexWrap="wrap" useFlexGap sx={{ mt: 1 }}>
                    <Chip size="small" variant="outlined" label={profileOf(account)?.name || '默认配置'} />
                    <Chip
                      size="small"
                      variant="outlined"
                      label={profileOf(account)?.connection_mode === 'transfer' ? '转服' : '代理'}
                    />
                    {account.platform === 'interop' && (
                      <Chip size="small" color="secondary" label="互通版会话" />
                    )}
                    {sessionExpired && <Chip size="small" color="error" label="登录已过期" />}
                  </Stack>

                  <Stack direction="row" spacing={0.5} alignItems="center" sx={{ mt: 1.2 }}>
                    <Typography variant="body2" fontFamily="ui-monospace, monospace">
                      {room?.room_id || '—'}
                    </Typography>
                    {room?.room_id && (
                      <IconButton size="small" aria-label="复制房间号" onClick={() => copyRoom(room.room_id)}>
                        <ContentCopy fontSize="inherit" />
                      </IconButton>
                    )}
                    <Box sx={{ flexGrow: 1 }} />
                    <Typography variant="caption" color="text.secondary">
                      {isProxy(account) ? `${room?.players ?? 0} 人` : '转服模式'}
                    </Typography>
                  </Stack>

                  <TextField
                    select
                    size="small"
                    fullWidth
                    label="配置方案"
                    value={profileIDOf(account)}
                    onChange={event => assign(account.index, event.target.value)}
                    disabled={running || busy}
                    sx={{ mt: 1.2 }}
                  >
                    {profiles.map(profile => (
                      <MenuItem key={profile.id} value={profile.id}>
                        {profile.name}
                      </MenuItem>
                    ))}
                  </TextField>

                  <Stack direction="row" spacing={0.5} flexWrap="wrap" useFlexGap sx={{ mt: 1 }}>
                    <Button size="small" startIcon={<Dashboard fontSize="small" />} onClick={() => onOpenDashboard?.(account.index)}>
                      仪表盘
                    </Button>
                    <Button
                      size="small"
                      startIcon={<PeopleAlt fontSize="small" />}
                      disabled={!isProxy(account)}
                      onClick={() => setPlayersRoom(account.index)}
                    >
                      玩家
                    </Button>
                    <Button
                      size="small"
                      startIcon={<Key fontSize="small" />}
                      disabled={running || busy}
                      onClick={() => {
                        setReloginEmail(account.email || '');
                        setReloginIndex(account.index);
                      }}
                    >
                      更新登录
                    </Button>
                    <Button
                      size="small"
                      color="error"
                      startIcon={<DeleteOutlined fontSize="small" />}
                      disabled={running || busy}
                      onClick={() => remove(account.index)}
                    >
                      移除
                    </Button>
                  </Stack>
                </CardContent>
              </Card>
            );
          })}
        </Stack>
      ) : (
      <TableContainer sx={{ border: 1, borderColor: 'divider', borderRadius: 3 }}>
        <Table size="small">
          <TableHead>
            <TableRow>
              <TableCell>房间</TableCell>
              <TableCell>配置方案</TableCell>
              <TableCell>网易账号</TableCell>
              <TableCell>游戏名</TableCell>
              <TableCell>状态</TableCell>
              <TableCell>房间号</TableCell>
              <TableCell align="right">玩家</TableCell>
              <TableCell align="right">操作</TableCell>
            </TableRow>
          </TableHead>
          <TableBody>
            {visible.length === 0 && (
              <TableRow>
                <TableCell colSpan={8}>
                  <Box sx={{ py: 5, textAlign: 'center' }}>
                    <Typography color="text.secondary">
                      {accounts.length ? '没有匹配的账号，请调整搜索或筛选条件。' : '还没有网易账号，请先登录一个账号。'}
                    </Typography>
                  </Box>
                </TableCell>
              </TableRow>
            )}
            {visible.map(account => {
              const room = rooms[account.index];
              const phase = room?.phase || 'stopped';
              const sessionExpired = account.expires_at && new Date(account.expires_at) < new Date();
              return (
                <TableRow key={account.index} hover>
                  <TableCell>
                    <Typography fontWeight={750}>房间 {account.index + 1}</Typography>
                  </TableCell>
                  <TableCell>
                    <TextField
                      select
                      size="small"
                      value={profileIDOf(account)}
                      onChange={event => assign(account.index, event.target.value)}
                      disabled={running || busy}
                      sx={{ minWidth: 130 }}
                    >
                      {profiles.map(profile => (
                        <MenuItem key={profile.id} value={profile.id}>
                          {profile.name}
                        </MenuItem>
                      ))}
                    </TextField>
                    <Typography variant="caption" color="text.secondary" display="block" sx={{ mt: 0.3 }}>
                      {profileOf(account)?.connection_mode === 'transfer' ? '转服' : '代理'}
                    </Typography>
                  </TableCell>
                  <TableCell>
                    {account.email ? (
                      <Typography variant="body2">{account.email}</Typography>
                    ) : (
                      <Chip size="small" color="warning" label={account.label || '未命名'} />
                    )}
                    {account.platform === 'interop' && (
                      <Stack direction="row" spacing={0.6} alignItems="center" sx={{ mt: 0.4 }}>
                        <Chip size="small" color="secondary" label="互通版会话" />
                        {account.expires_at && (
                          <Typography variant="caption" color={sessionExpired ? 'error' : 'text.secondary'}>
                            {sessionExpired
                              ? '登录已过期，请更新登录'
                              : `${new Date(account.expires_at).toLocaleDateString('zh-CN')} 到期`}
                          </Typography>
                        )}
                      </Stack>
                    )}
                  </TableCell>
                  <TableCell>
                    {account.name ? (
                      <Tooltip title={account.profile_error ? `上次错误：${account.profile_error}` : account.uid ? `UID ${account.uid}` : ''}>
                        <Typography variant="body2" fontWeight={650} noWrap sx={{ maxWidth: 180 }}>
                          {account.name}
                        </Typography>
                      </Tooltip>
                    ) : (
                      <Tooltip title={account.profile_error || '尚未读取游戏名，可以用右侧「更新登录」重新登录'}>
                        <Typography variant="body2" color={account.profile_error ? 'error' : 'text.secondary'} noWrap sx={{ maxWidth: 180 }}>
                          {account.profile_error ? '解析失败' : '未解析'}
                        </Typography>
                      </Tooltip>
                    )}
                  </TableCell>
                  <TableCell>
                    <Chip size="small" color={phaseColor(phase)} label={phaseLabel(phase)} />
                  </TableCell>
                  <TableCell>
                    {room?.room_id ? (
                      <Stack direction="row" spacing={0.5} alignItems="center">
                        <Typography variant="body2" fontFamily="ui-monospace, monospace">
                          {room.room_id}
                        </Typography>
                        <Tooltip title="复制房间号">
                          <IconButton size="small" onClick={() => copyRoom(room.room_id)}>
                            <ContentCopy fontSize="inherit" />
                          </IconButton>
                        </Tooltip>
                      </Stack>
                    ) : (
                      <Typography variant="body2" color="text.secondary">—</Typography>
                    )}
                  </TableCell>
                  <TableCell align="right">
                    {isProxy(account) ? (
                      <Typography variant="body2">{room?.players ?? 0}</Typography>
                    ) : (
                      <Tooltip title="转服模式下玩家由目标服务器管理">
                        <Typography variant="body2" color="text.secondary">—</Typography>
                      </Tooltip>
                    )}
                  </TableCell>
                  <TableCell align="right">
                    <Tooltip title="打开该账号仪表盘">
                      <span>
                        <IconButton
                          size="small"
                          color="secondary"
                          aria-label="打开仪表盘"
                          onClick={() => onOpenDashboard?.(account.index)}
                        >
                          <Dashboard fontSize="small" />
                        </IconButton>
                      </span>
                    </Tooltip>
                    <Tooltip
                      title={isProxy(account) ? '管理这个房间的玩家' : '转服模式下玩家由目标服务器管理'}
                    >
                      <span>
                        <IconButton
                          size="small"
                          color="info"
                          aria-label="管理玩家"
                          disabled={!isProxy(account)}
                          onClick={() => setPlayersRoom(account.index)}
                        >
                          <PeopleAlt fontSize="small" />
                        </IconButton>
                      </span>
                    </Tooltip>
                    <Tooltip
                      title={
                        running
                          ? '请先停止房间'
                          : '更新登录凭据：旧凭据过期后，用邮箱密码或新的 sauth 重新登录'
                      }
                    >
                      <span>
                        <IconButton
                          size="small"
                          color="primary"
                          aria-label="更新登录"
                          disabled={running || busy}
                          onClick={() => {
                            setReloginEmail(account.email || '');
                            setReloginIndex(account.index);
                          }}
                        >
                          <Key fontSize="small" />
                        </IconButton>
                      </span>
                    </Tooltip>
                    <Tooltip title={running ? '请先停止房间' : '移除账号'}>
                      <span>
                        <IconButton size="small" color="error" aria-label="移除账号" disabled={running || busy} onClick={() => remove(account.index)}>
                          <DeleteOutlined fontSize="small" />
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

      <BulkImportDialog
        open={bulkOpen}
        onClose={() => setBulkOpen(false)}
        onChanged={onChanged}
        running={running}
        profiles={profiles}
        defaultProfileId={defaultProfileID}
      />

      <PlayersDialog
        open={playersRoom !== null}
        roomIndex={playersRoom}
        status={status}
        onClose={() => setPlayersRoom(null)}
      />

      <Dialog open={reloginIndex !== null} onClose={() => setReloginIndex(null)} fullWidth maxWidth="sm" fullScreen={phone}>
        <DialogTitle sx={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 1 }}>
          <span>更新房间 {reloginIndex !== null ? reloginIndex + 1 : ''} 的登录</span>
          <IconButton aria-label="关闭" onClick={() => setReloginIndex(null)} size="small">
            <Close />
          </IconButton>
        </DialogTitle>
        <DialogContent>
          {reloginIndex !== null && (
            <AccountLogin
              compact
              replaceIndex={reloginIndex}
              initialEmail={reloginEmail}
              profiles={profiles}
              defaultProfileId={defaultProfileID}
              initialProfileId={accounts[reloginIndex]?.profile_id || defaultProfileID}
              onAdded={() => {
                setReloginIndex(null);
                onChanged?.();
              }}
            />
          )}
        </DialogContent>
      </Dialog>

      <Dialog open={addOpen} onClose={() => setAddOpen(false)} fullWidth maxWidth="sm" fullScreen={phone}>
        <DialogTitle sx={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 1 }}>
          <span>登录网易账号</span>
          <IconButton aria-label="关闭" onClick={() => setAddOpen(false)} size="small">
            <Close />
          </IconButton>
        </DialogTitle>
        <DialogContent>
          <Box sx={{ pt: 1 }}>
            <AccountLogin
              compact
              profiles={profiles}
              defaultProfileId={defaultProfileID}
              initialProfileId={defaultProfileID}
              onAdded={() => {
                setAddOpen(false);
                onChanged?.();
              }}
            />
          </Box>
        </DialogContent>
      </Dialog>
    </Stack>
  );
}


import React, { useState } from 'react';
import {
  Alert,
  Box,
  Button,
  Chip,
  Dialog,
  DialogContent,
  DialogTitle,
  IconButton,
  Stack,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  Tooltip,
  Typography,
} from '@mui/material';
import { Add, ContentCopy, DeleteOutlined, Refresh } from '@mui/icons-material';
import { api } from './api.js';
import { copyText, phaseColor, phaseLabel } from './util.js';
import { useToast } from './App.jsx';
import AccountLogin from './AccountLogin.jsx';

export default function AccountsPanel({ status, onChanged }) {
  const notify = useToast();
  const [addOpen, setAddOpen] = useState(false);
  const [busy, setBusy] = useState(false);
  const accounts = status.accounts || [];
  const rooms = status.rooms || [];
  const running = !!status.running;
  const canAdd = !running && accounts.length < 8;

  const remove = async index => {
    if (running) {
      notify('请先停止房间，再移除账号', 'warning');
      return;
    }
    if (!window.confirm(`移除房间 ${index + 1} 的网易账号？该房间保存的会话也会删除。`)) return;
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

  const verify = async index => {
    if (running) {
      notify('请先停止房间，再验证账号', 'warning');
      return;
    }
    setBusy(true);
    try {
      const result = await api('account/profile', { index });
      notify(`房间 ${index + 1} 游戏名：${result.name}`, 'success');
      onChanged?.();
    } catch (failure) {
      notify(failure.message || '游戏名解析失败', 'error');
      onChanged?.();
    } finally {
      setBusy(false);
    }
  };

  return (
    <Stack spacing={2}>
      <Stack direction={{ xs: 'column', sm: 'row' }} justifyContent="space-between" alignItems={{ xs: 'stretch', sm: 'center' }} spacing={1.5}>
        <Box>
          <Typography variant="h6">网易账号与房间</Typography>
          <Typography variant="body2" color="text.secondary">
            每个账号对应一个房间，最多 8 个。同一账号同时开两个房间会互相挤下线。
          </Typography>
        </Box>
        <Tooltip title={running ? '请先停止房间' : accounts.length >= 8 ? '最多 8 个账号' : ''}>
          <span>
            <Button variant="contained" startIcon={<Add />} disabled={!canAdd} onClick={() => setAddOpen(true)}>
              添加网易账号
            </Button>
          </span>
        </Tooltip>
      </Stack>

      {running && <Alert severity="info">房间运行中，停止后才能添加或移除账号。</Alert>}

      <TableContainer sx={{ border: 1, borderColor: 'divider', borderRadius: 3 }}>
        <Table size="small">
          <TableHead>
            <TableRow>
              <TableCell>房间</TableCell>
              <TableCell>网易账号</TableCell>
              <TableCell>游戏名</TableCell>
              <TableCell>状态</TableCell>
              <TableCell>房间号</TableCell>
              <TableCell align="right">玩家</TableCell>
              <TableCell align="right">操作</TableCell>
            </TableRow>
          </TableHead>
          <TableBody>
            {accounts.length === 0 && (
              <TableRow>
                <TableCell colSpan={7}>
                  <Box sx={{ py: 5, textAlign: 'center' }}>
                    <Typography color="text.secondary">还没有网易账号，请先登录一个账号。</Typography>
                  </Box>
                </TableCell>
              </TableRow>
            )}
            {accounts.map(account => {
              const room = rooms[account.index];
              const phase = room?.phase || 'stopped';
              const sessionExpired = account.expires_at && new Date(account.expires_at) < new Date();
              return (
                <TableRow key={account.index} hover>
                  <TableCell>
                    <Typography fontWeight={750}>房间 {account.index + 1}</Typography>
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
                              ? '会话已过期，请重新导入'
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
                      <Tooltip title={account.profile_error || '尚未解析游戏名，点击右侧刷新按钮验证'}>
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
                    <Typography variant="body2">{room?.players ?? 0}</Typography>
                  </TableCell>
                  <TableCell align="right">
                    <Tooltip title={running ? '请先停止房间' : '解析/更新游戏名'}>
                      <span>
                        <IconButton
                          size="small"
                          color="primary"
                          disabled={running || busy || !account.email}
                          onClick={() => verify(account.index)}
                        >
                          <Refresh fontSize="small" />
                        </IconButton>
                      </span>
                    </Tooltip>
                    <Tooltip title={running ? '请先停止房间' : '移除账号'}>
                      <span>
                        <IconButton size="small" color="error" disabled={running || busy} onClick={() => remove(account.index)}>
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

      <Dialog open={addOpen} onClose={() => setAddOpen(false)} fullWidth maxWidth="sm">
        <DialogTitle>登录网易账号</DialogTitle>
        <DialogContent>
          <Box sx={{ pt: 1 }}>
            <AccountLogin
              compact
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


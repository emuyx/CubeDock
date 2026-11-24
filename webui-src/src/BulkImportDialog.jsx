import React, { useRef, useState } from 'react';
import {
  Alert,
  Box,
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  LinearProgress,
  MenuItem,
  TextField,
  useMediaQuery,
  useTheme,
} from '@mui/material';
import Typography from './Typography.jsx';
import Stack from './Stack.jsx';
import { PlayArrow, Stop } from '@mui/icons-material';
import { api } from './api.js';
import { useToast } from './App.jsx';

const MAX_LINES = 50;
const MAX_CHARS = 64 * 1024;

export default function BulkImportDialog({
  open,
  onClose,
  onChanged,
  running,
  profiles = [],
  defaultProfileId = '',
}) {
  const notify = useToast();
  const theme = useTheme();
  const fullScreen = useMediaQuery(theme.breakpoints.down('sm'));
  const [text, setText] = useState('');
  const [label, setLabel] = useState('');
  const [profileId, setProfileId] = useState(defaultProfileId || profiles[0]?.id || '');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [results, setResults] = useState([]);
  const abort = useRef(false);
  const controller = useRef(null);

  const reset = () => {
    if (busy) return;
    setText('');
    setLabel('');
    setProfileId(defaultProfileId || profiles[0]?.id || '');
    setError('');
    setResults([]);
  };

  const close = () => {
    if (busy) return;
    reset();
    onClose();
  };

  const run = async () => {
    const lines = text
      .split(/\r?\n/)
      .map(line => line.trim())
      .filter(line => line && !line.startsWith('#'));
    if (!lines.length) {
      setError('请至少粘贴一个登录凭证');
      return;
    }
    if (lines.length > MAX_LINES) {
      setError(`一次最多导入 ${MAX_LINES} 条`);
      return;
    }
    if (text.length > MAX_CHARS) {
      setError('粘贴内容过大，请分批导入');
      return;
    }
    setError('');
    abort.current = false;
    setBusy(true);
    const next = lines.map((_, index) => ({ index, status: 'pending', message: '' }));
    setResults(next);
    let ok = 0;
    let failed = 0;
    let needNames = 0;
    for (let index = 0; index < lines.length; index += 1) {
      if (abort.current) {
        setResults(current => current.map(row => (row.status === 'pending' ? { ...row, status: 'skipped' } : row)));
        break;
      }
      setResults(current => current.map(row => (row.index === index ? { ...row, status: 'running' } : row)));
      const request = new AbortController();
      controller.current = request;
      try {
        const result = await api('session/import', {
          sauth: lines[index],
          label: label.trim() ? `${label.trim()}-${index + 1}` : '',
          profile_id: profileId,
        }, 'POST', { signal: request.signal });
        ok += 1;
        if (result.status === 'need_name') needNames += 1;
        setResults(current =>
          current.map(row =>
            row.index === index
              ? {
                  ...row,
                  status: 'ok',
                  message:
                    result.status === 'need_name'
                      ? `房间 ${(result.index ?? 0) + 1} · 需要设置游戏名`
                      : `房间 ${(result.index ?? 0) + 1} · ${result.name || '已添加'}`,
                }
              : row,
          ),
        );
      } catch (failure) {
        if (abort.current || failure.name === 'AbortError') {
          setResults(current =>
            current.map(row =>
              row.index === index ? { ...row, status: 'skipped', message: '已停止' } : row,
            ),
          );
          break;
        }
        failed += 1;
        setResults(current =>
          current.map(row =>
            row.index === index ? { ...row, status: 'error', message: failure.message || '导入失败' } : row,
          ),
        );
      } finally {
        if (controller.current === request) controller.current = null;
      }
    }
    setBusy(false);
    onChanged?.();
    if (abort.current) {
      notify(`批量导入已停止：成功 ${ok}，失败 ${failed}`, 'warning');
      return;
    }
    const suffix = needNames ? `，${needNames} 个需要设置游戏名` : '';
    notify(`批量导入完成：成功 ${ok}，失败 ${failed}${suffix}`, failed || needNames ? 'warning' : 'success');
  };

  const done = results.filter(row => row.status !== 'pending' && row.status !== 'running').length;
  const progress = results.length ? Math.round((done / results.length) * 100) : 0;

  return (
    <Dialog open={open} onClose={close} fullWidth maxWidth="sm" fullScreen={fullScreen}>
      <DialogTitle>批量导入账号</DialogTitle>
      <DialogContent>
        <Stack spacing={2} sx={{ pt: 1 }}>
          <Typography variant="body2" color="text.secondary">
            每行粘贴一个登录凭证（sauth 会话）。空行和 # 开头的行会被忽略。凭证只在本机内存中处理，不会保存到浏览器。
            新号如未设置游戏名，导入后会提示需要起名，可到账号列表点击「起名」。
          </Typography>
          {running && <Alert severity="info">房间运行中，停止后才能导入账号。</Alert>}
          <TextField
            multiline
            minRows={8}
            maxRows={14}
            fullWidth
            value={text}
            onChange={event => setText(event.target.value)}
            placeholder={'{"sauth_json":"..."}\n{"sauth_json":"..."}'}
            disabled={busy}
            inputProps={{ style: { fontFamily: 'ui-monospace, monospace', fontSize: 12 } }}
            helperText={`${text.length} / ${MAX_CHARS} 字符`}
          />
          {profiles.length > 0 && (
            <TextField
              select
              label="配置方案"
              value={profileId}
              onChange={event => setProfileId(event.target.value)}
              disabled={busy}
              fullWidth
            >
              {profiles.map(profile => (
                <MenuItem key={profile.id} value={profile.id}>
                  {profile.name}（{profile.connection_mode === 'transfer' ? '转服' : '代理'} · {profile.upstream}）
                </MenuItem>
              ))}
            </TextField>
          )}
          <TextField
            label="标签前缀（可选）"
            value={label}
            onChange={event => setLabel(event.target.value)}
            disabled={busy}
            helperText="例如填写 alts，会给每条会话标记为 alts-1、alts-2"
          />
          {error && <Alert severity="error">{error}</Alert>}
          {busy && <LinearProgress variant="determinate" value={progress} />}
          {results.length > 0 && (
            <Box sx={{ maxHeight: 220, overflowY: 'auto', border: 1, borderColor: 'divider', borderRadius: 2, p: 1 }}>
              <Stack spacing={0.6}>
                {results.map(row => (
                  <Stack key={row.index} direction="row" spacing={1} alignItems="center">
                    <Typography variant="caption" color="text.secondary" sx={{ width: 64, flexShrink: 0 }}>
                      第 {row.index + 1} 条
                    </Typography>
                    <Typography
                      variant="caption"
                      fontWeight={700}
                      color={row.status === 'ok' ? 'success.main' : row.status === 'error' ? 'error.main' : 'text.secondary'}
                    >
                      {row.status === 'pending'
                        ? '等待'
                        : row.status === 'running'
                          ? '导入中…'
                          : row.status === 'ok'
                            ? '成功'
                            : row.status === 'skipped'
                              ? '已跳过'
                              : '失败'}
                    </Typography>
                    {row.message && (
                      <Typography variant="caption" color="text.secondary" noWrap sx={{ minWidth: 0 }}>
                        {row.message}
                      </Typography>
                    )}
                  </Stack>
                ))}
              </Stack>
            </Box>
          )}
        </Stack>
      </DialogContent>
      <DialogActions>
        {busy ? (
          <Button
            color="warning"
            startIcon={<Stop />}
            onClick={() => {
              abort.current = true;
              controller.current?.abort();
            }}
          >
            停止
          </Button>
        ) : (
          <Button onClick={close}>关闭</Button>
        )}
        <Button variant="contained" startIcon={<PlayArrow />} onClick={run} disabled={busy || running}>
          开始导入
        </Button>
      </DialogActions>
    </Dialog>
  );
}

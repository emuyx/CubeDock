import React, { useEffect, useState } from 'react';
import {
  Alert,
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  IconButton,
  InputAdornment,
  Stack,
  TextField,
} from '@mui/material';
import { Key, Visibility, VisibilityOff } from '@mui/icons-material';
import { api } from './api.js';
import { useToast } from './App.jsx';

export default function ChangePasswordDialog({ open, onClose }) {
  const notify = useToast();
  const [current, setCurrent] = useState('');
  const [next, setNext] = useState('');
  const [confirm, setConfirm] = useState('');
  const [show, setShow] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');

  useEffect(() => {
    if (open) {
      setCurrent('');
      setNext('');
      setConfirm('');
      setShow(false);
      setBusy(false);
      setError('');
    }
  }, [open]);

  const close = () => {
    if (!busy) onClose();
  };

  const submit = async event => {
    event.preventDefault();
    setError('');
    if (next.length < 8 || next.length > 72) {
      setError('新密码长度必须为 8–72 字节');
      return;
    }
    if (next !== confirm) {
      setError('两次输入的新密码不一致');
      return;
    }
    setBusy(true);
    try {
      await api('admin/password', { current, next });
      notify('管理密码已修改，请使用新密码重新登录', 'success');
      onClose();
      setTimeout(() => window.location.replace('/login'), 600);
    } catch (failure) {
      setError(failure.message || '修改密码失败');
      setBusy(false);
    }
  };

  const endAdornment = (
    <InputAdornment position="end">
      <IconButton onClick={() => setShow(value => !value)} edge="end" aria-label="显示或隐藏密码">
        {show ? <VisibilityOff /> : <Visibility />}
      </IconButton>
    </InputAdornment>
  );

  return (
    <Dialog open={open} onClose={close} fullWidth maxWidth="xs">
      <DialogTitle>
        <Stack direction="row" spacing={1} alignItems="center">
          <Key color="primary" />
          修改管理密码
        </Stack>
      </DialogTitle>
      <DialogContent>
        <Stack component="form" id="change-password-form" onSubmit={submit} spacing={2} sx={{ pt: 1 }}>
          <TextField
            label="当前密码"
            type={show ? 'text' : 'password'}
            value={current}
            onChange={event => setCurrent(event.target.value)}
            autoComplete="current-password"
            required
            fullWidth
            slotProps={{ input: { endAdornment } }}
          />
          <TextField
            label="新密码"
            type={show ? 'text' : 'password'}
            value={next}
            onChange={event => setNext(event.target.value)}
            autoComplete="new-password"
            required
            fullWidth
            helperText="8–72 字节"
          />
          <TextField
            label="确认新密码"
            type={show ? 'text' : 'password'}
            value={confirm}
            onChange={event => setConfirm(event.target.value)}
            autoComplete="new-password"
            required
            fullWidth
          />
          {error && <Alert severity="error">{error}</Alert>}
          <Alert severity="info" variant="outlined">
            修改成功后所有控制台会话都会失效，需要使用新密码重新登录。
          </Alert>
        </Stack>
      </DialogContent>
      <DialogActions sx={{ px: 3, pb: 2.5 }}>
        <Button onClick={close} color="inherit" disabled={busy}>
          取消
        </Button>
        <Button type="submit" form="change-password-form" variant="contained" disabled={busy}>
          {busy ? '保存中…' : '修改密码'}
        </Button>
      </DialogActions>
    </Dialog>
  );
}

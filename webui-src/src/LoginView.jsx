import React, { useState } from 'react';
import {
  Alert,
  Box,
  Button,
  Card,
  CardContent,
  IconButton,
  InputAdornment,
  Stack,
  TextField,
  Tooltip,
  Typography,
} from '@mui/material';
import {
  ArrowForward,
  DarkMode,
  LightMode,
  LockOutlined,
  Visibility,
  VisibilityOff,
} from '@mui/icons-material';
import { login } from './api.js';

export default function LoginView({ mode, toggleMode }) {
  const [username, setUsername] = useState('admin');
  const [password, setPassword] = useState('');
  const [show, setShow] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');

  const submit = async event => {
    event.preventDefault();
    setBusy(true);
    setError('');
    try {
      await login(username.trim(), password);
      setPassword('');
      window.location.replace('/');
    } catch (failure) {
      setError(failure.message || '登录失败');
      setBusy(false);
    }
  };

  return (
    <Box
      sx={{
        minHeight: '100vh',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        p: 2,
        position: 'relative',
      }}
    >
      <Tooltip title={mode === 'dark' ? '切换浅色主题' : '切换深色主题'}>
        <IconButton onClick={toggleMode} sx={{ position: 'absolute', top: 18, right: 18 }}>
          {mode === 'dark' ? <LightMode /> : <DarkMode />}
        </IconButton>
      </Tooltip>
      <Card sx={{ width: '100%', maxWidth: 470, boxShadow: 16 }}>
        <CardContent sx={{ p: { xs: 3, sm: 4.5 } }}>
          <Stack spacing={3.2}>
            <Stack direction="row" spacing={1.6} alignItems="center">
              <Box
                sx={{
                  width: 46,
                  height: 46,
                  borderRadius: 2.5,
                  display: 'grid',
                  placeItems: 'center',
                  background: 'linear-gradient(135deg,#8b5cf6,#2dd4bf)',
                  color: '#fff',
                  fontWeight: 900,
                  fontSize: 23,
                  boxShadow: '0 10px 30px rgba(139,92,246,.35)',
                }}
              >
                C
              </Box>
              <Box>
                <Typography variant="h6" fontWeight={850}>
                  CubeDock <span style={{ fontWeight: 500, opacity: 0.72 }}>方块码头</span>
                </Typography>
                <Typography variant="caption" color="text.secondary" letterSpacing=".12em">
                  CONTROL PANEL
                </Typography>
              </Box>
            </Stack>
            <Box>
              <Stack direction="row" spacing={1} alignItems="center" sx={{ mb: 1 }}>
                <LockOutlined color="primary" fontSize="small" />
                <Typography variant="overline" color="primary" fontWeight={800}>
                  管理员登录
                </Typography>
              </Stack>
              <Typography variant="h4" sx={{ letterSpacing: '-0.035em' }}>
                连接世界，从这里开始
              </Typography>
              <Typography color="text.secondary" sx={{ mt: 1 }}>
                管理网易账号、本地联机房间和网易协议服务器。
              </Typography>
            </Box>
            <Box component="form" onSubmit={submit}>
              <Stack spacing={2}>
                <TextField
                  label="管理员账号"
                  value={username}
                  onChange={event => setUsername(event.target.value)}
                  autoComplete="username"
                  fullWidth
                  required
                />
                <TextField
                  label="登录密码"
                  type={show ? 'text' : 'password'}
                  value={password}
                  onChange={event => setPassword(event.target.value)}
                  autoComplete="current-password"
                  fullWidth
                  required
                  autoFocus
                  slotProps={{
                    input: {
                      endAdornment: (
                        <InputAdornment position="end">
                          <IconButton onClick={() => setShow(current => !current)} edge="end" aria-label="显示或隐藏密码">
                            {show ? <VisibilityOff /> : <Visibility />}
                          </IconButton>
                        </InputAdornment>
                      ),
                    },
                  }}
                />
                {error && <Alert severity="error">{error}</Alert>}
                <Button type="submit" variant="contained" size="large" disabled={busy} endIcon={<ArrowForward />}>
                  {busy ? '正在验证…' : '登录控制台'}
                </Button>
              </Stack>
            </Box>
            <Typography variant="body2" color="text.secondary">
              首次启动时管理员密码显示在终端。忘记密码可停止程序后使用 <code>-reset-admin-password</code> 重置。
            </Typography>
          </Stack>
        </CardContent>
      </Card>
    </Box>
  );
}

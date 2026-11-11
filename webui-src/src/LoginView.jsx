import React, { useEffect, useState } from 'react';
import {
  Alert,
  Box,
  Button,
  Card,
  CardContent,
  CircularProgress,
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
import { completeSetup, getSetupStatus, login } from './api.js';

export default function LoginView({ mode, toggleMode }) {
  const [setup, setSetup] = useState(null);
  const [username, setUsername] = useState('admin');
  const [password, setPassword] = useState('');
  const [setupUsername, setSetupUsername] = useState('admin');
  const [setupPassword, setSetupPassword] = useState('');
  const [confirm, setConfirm] = useState('');
  const [show, setShow] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');

  useEffect(() => {
    let active = true;
    getSetupStatus()
      .then(status => {
        if (!active) return;
        setSetup(status);
        if (status.username) {
          setUsername(status.username);
          setSetupUsername(status.username);
        }
      })
      .catch(failure => {
        if (!active) return;
        setSetup({ required: false, local: true });
        setError(failure.message || '读取初始化状态失败');
      });
    return () => {
      active = false;
    };
  }, []);

  const setupMode = !!setup?.required;

  const submitLogin = async event => {
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

  const submitSetup = async event => {
    event.preventDefault();
    setError('');
    if (!setup?.local) {
      setError('初始化只允许在运行 CubeDock 的电脑上完成。');
      return;
    }
    if (setupPassword !== confirm) {
      setError('两次输入的密码不一致');
      return;
    }
    setBusy(true);
    try {
      await completeSetup(setupUsername.trim(), setupPassword, confirm);
      // A freshly created administrator should see the room setup wizard even
      // if this browser previously dismissed it for another install.
      localStorage.removeItem('cubedock-setup-done');
      window.location.replace('/');
    } catch (failure) {
      setError(failure.message || '创建管理员账户失败');
      setBusy(false);
    }
  };

  const passwordField = (label, value, onChange, autoComplete) => (
    <TextField
      label={label}
      type={show ? 'text' : 'password'}
      value={value}
      onChange={event => onChange(event.target.value)}
      autoComplete={autoComplete}
      fullWidth
      required
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
  );

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
                  background: 'linear-gradient(135deg,#f59e0b,#34d399)',
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
                  {setupMode ? '管理员初始化' : '管理员登录'}
                </Typography>
              </Stack>
              <Typography variant="h4" sx={{ letterSpacing: '-0.035em' }}>
                {setupMode ? '先创建管理员账户' : '连接世界，从这里开始'}
              </Typography>
              <Typography color="text.secondary" sx={{ mt: 1 }}>
                {setupMode
                  ? '账号和密码只保存在本机，之后用它登录控制台。'
                  : '管理网易账号、本地联机房间和网易协议服务器。'}
              </Typography>
            </Box>

            {!setup ? (
              <Box sx={{ py: 4, display: 'grid', placeItems: 'center' }}>
                <CircularProgress />
              </Box>
            ) : setupMode ? (
              <Box component="form" onSubmit={submitSetup}>
                <Stack spacing={2}>
                  <TextField
                    label="管理员账号"
                    value={setupUsername}
                    onChange={event => setSetupUsername(event.target.value)}
                    autoComplete="username"
                    fullWidth
                    required
                    autoFocus
                    helperText="用于登录，不能包含空格；建议不要继续使用默认的 admin"
                  />
                  {passwordField('设置密码', setupPassword, setSetupPassword, 'new-password')}
                  {passwordField('确认密码', confirm, setConfirm, 'new-password')}
                  <Typography variant="caption" color="text.secondary">
                    密码为 8–72 字节。请妥善保存，忘记后只能在本机删除管理员凭据并重新初始化。
                  </Typography>
                  {!setup.local && (
                    <Alert severity="warning">
                      初始化只允许在运行 CubeDock 的电脑上完成。请在本机打开控制台完成设置。
                    </Alert>
                  )}
                  {error && <Alert severity="error">{error}</Alert>}
                  <Button
                    type="submit"
                    variant="contained"
                    size="large"
                    disabled={busy || !setup.local}
                    endIcon={<ArrowForward />}
                  >
                    {busy ? '正在创建…' : '创建管理员并进入'}
                  </Button>
                </Stack>
              </Box>
            ) : (
              <Box component="form" onSubmit={submitLogin}>
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
            )}

            <Typography variant="body2" color="text.secondary">
              {setupMode
                ? '初始化只允许在运行 CubeDock 的这台电脑上完成。'
                : '忘记密码：停止程序后备份并删除数据目录中的 admin.json，重启后在控制台重新创建管理员账户。'}
            </Typography>
          </Stack>
        </CardContent>
      </Card>
    </Box>
  );
}

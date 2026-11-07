import React, { useEffect, useRef, useState } from 'react';
import {
  Alert,
  Box,
  Button,
  Divider,
  IconButton,
  InputAdornment,
  Stack,
  TextField,
  Typography,
} from '@mui/material';
import { ContentCopy, Visibility, VisibilityOff } from '@mui/icons-material';
import { api } from './api.js';
import { useToast } from './App.jsx';
import { copyText } from './util.js';

export default function AccountLogin({ onAdded, compact = false }) {
  const notify = useToast();
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [showPassword, setShowPassword] = useState(false);
  const [label, setLabel] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [message, setMessage] = useState('');
  const [verifyURL, setVerifyURL] = useState('');
  const [sauth, setSauth] = useState('');
  const [sauthLabel, setSauthLabel] = useState('');
  const [sauthBusy, setSauthBusy] = useState(false);
  const [sauthError, setSauthError] = useState('');
  const [sauthProfile, setSauthProfile] = useState(null);
  const [autoWaiting, setAutoWaiting] = useState(false);
  const autoTimer = useRef(null);
  const autoStop = useRef(null);
  const requestRef = useRef(false);
  const credentialsRef = useRef({ email: '', password: '', label: '' });

  const stopAuto = (notice = '') => {
    if (autoTimer.current) {
      clearInterval(autoTimer.current);
      autoTimer.current = null;
    }
    if (autoStop.current) {
      clearTimeout(autoStop.current);
      autoStop.current = null;
    }
    setAutoWaiting(false);
    if (notice) setMessage(notice);
  };

  useEffect(() => () => {
    if (autoTimer.current) clearInterval(autoTimer.current);
    if (autoStop.current) clearTimeout(autoStop.current);
  }, []);

  const runLogin = async (auto = false) => {
    if (requestRef.current) return;
    const credentials = auto
      ? credentialsRef.current
      : { email: email.trim(), password, label: label.trim() };
    if (!credentials.email || !credentials.password) {
      if (!auto) setError('请填写网易邮箱和密码');
      return;
    }
    if (!auto) {
      credentialsRef.current = credentials;
      setBusy(true);
      setError('');
      setMessage('');
      setVerifyURL('');
      stopAuto();
    }
    requestRef.current = true;
    try {
      const result = await api('session/interop-login', credentials);
      if (result.status === 'need_verify') {
        setVerifyURL(current => result.verify_url || current || '');
        setMessage(result.message || '需要完成网易安全验证');
        if (!auto) startAuto();
        return;
      }
      stopAuto();
      setVerifyURL('');
      setMessage('');
      notify(`已添加互通版账号：${result.name} → 房间 ${(result.index ?? 0) + 1}`, 'success');
      setPassword('');
      if (onAdded) onAdded(result);
    } catch (failure) {
      if (auto) {
        stopAuto();
        setMessage('');
      }
      setError(failure.message || '互通版直登失败');
    } finally {
      requestRef.current = false;
      if (!auto) setBusy(false);
    }
  };

  const startAuto = () => {
    if (!credentialsRef.current.email) return;
    stopAuto();
    setAutoWaiting(true);
    autoTimer.current = setInterval(() => {
      runLogin(true);
    }, 6000);
    autoStop.current = setTimeout(() => {
      stopAuto('自动重试已超时，请完成验证后手动点击“登录并添加”。');
    }, 120000);
  };

  const copyVerify = async () => {
    try {
      await copyText(verifyURL);
      notify('验证链接已复制', 'success');
    } catch (failure) {
      notify(failure.message || '复制失败', 'error');
    }
  };

  const importSauth = async () => {
    const value = sauth.trim();
    if (!value) {
      setSauthError('请粘贴互通版会话 sauth_json');
      return;
    }
    setSauthBusy(true);
    setSauthError('');
    setSauthProfile(null);
    try {
      const result = await api('session/import', { sauth: value, label: sauthLabel.trim() });
      notify(`已导入互通版会话：${result.name} → 房间 ${(result.index ?? 0) + 1}`, 'success');
      setSauthProfile(result);
      setSauth('');
      setSauthLabel('');
      if (onAdded) onAdded(result);
    } catch (failure) {
      setSauthError(failure.message || '互通版会话验证失败');
    } finally {
      setSauthBusy(false);
    }
  };

  return (
    <Stack spacing={compact ? 2 : 2.4}>
      <Box>
        <Alert severity="success" sx={{ mb: 1.5 }}>
          推荐：用网易邮箱和密码直接登录互通版（官方发烧平台 a50 链路），无需抓包。
        </Alert>
        <Stack spacing={1.4}>
          <Stack direction={{ xs: 'column', sm: 'row' }} spacing={1.2}>
            <TextField
              label="网易邮箱"
              type="email"
              value={email}
              onChange={event => setEmail(event.target.value)}
              autoComplete="off"
              fullWidth
            />
            <TextField
              label="密码"
              type={showPassword ? 'text' : 'password'}
              value={password}
              onChange={event => setPassword(event.target.value)}
              autoComplete="new-password"
              fullWidth
              slotProps={{
                input: {
                  endAdornment: (
                    <InputAdornment position="end">
                      <IconButton
                        edge="end"
                        onClick={() => setShowPassword(current => !current)}
                        aria-label="显示或隐藏密码"
                      >
                        {showPassword ? <VisibilityOff /> : <Visibility />}
                      </IconButton>
                    </InputAdornment>
                  ),
                },
              }}
            />
          </Stack>
          <Stack direction={{ xs: 'column', sm: 'row' }} spacing={1.2}>
            <TextField
              size="small"
              label="备注（可选）"
              value={label}
              onChange={event => setLabel(event.target.value)}
              fullWidth
            />
            <Button variant="contained" onClick={() => runLogin(false)} disabled={busy} sx={{ whiteSpace: 'nowrap' }}>
              {busy ? '登录中…' : '登录并添加'}
            </Button>
          </Stack>
          {verifyURL && (
            <Alert severity="warning">
              <Stack spacing={1.2}>
                <Typography variant="body2">
                  {message || '需要完成网易安全验证。'}
                </Typography>
                <TextField
                  value={verifyURL}
                  multiline
                  minRows={2}
                  maxRows={4}
                  fullWidth
                  slotProps={{ input: { readOnly: true } }}
                  sx={{ '& textarea': { fontFamily: 'ui-monospace, monospace', fontSize: 12 } }}
                />
                <Stack direction={{ xs: 'column', sm: 'row' }} spacing={1} flexWrap="wrap" useFlexGap>
                  <Button
                    variant="contained"
                    color="warning"
                    href={verifyURL}
                    target="_blank"
                    rel="noopener noreferrer"
                  >
                    打开网易验证页面
                  </Button>
                  <Button
                    variant="outlined"
                    color="warning"
                    startIcon={<ContentCopy />}
                    onClick={copyVerify}
                  >
                    复制链接
                  </Button>
                  {autoWaiting ? (
                    <Button color="inherit" onClick={() => stopAuto()}>
                      停止自动重试
                    </Button>
                  ) : (
                    <Button color="inherit" onClick={startAuto}>
                      自动等待验证并重试
                    </Button>
                  )}
                </Stack>
                {autoWaiting && (
                  <Typography variant="caption" color="text.secondary">
                    正在每 6 秒自动检查验证状态，最多 2 分钟；完成后会自动添加账号。
                  </Typography>
                )}
              </Stack>
            </Alert>
          )}
          {!verifyURL && message && <Alert severity="warning">{message}</Alert>}
          {error && <Alert severity="error">{error}</Alert>}
        </Stack>
      </Box>

      <Divider />

      <Box>
        <Alert severity="info" sx={{ mb: 1.5 }}>
          或粘贴已有的互通版会话（高级）：当前网易只接受从网易发烧平台启动的基岩互通版会话。
        </Alert>
        <Stack spacing={1.4}>
          <TextField
            multiline
            minRows={3}
            maxRows={8}
            label="互通版会话 sauth_json"
            placeholder='粘贴 sauth_json 对象或 {"sauth_json":"..."}'
            value={sauth}
            onChange={event => setSauth(event.target.value)}
            fullWidth
            sx={{ '& textarea': { fontFamily: 'ui-monospace, monospace', fontSize: 12 } }}
          />
          <Stack direction={{ xs: 'column', sm: 'row' }} spacing={1.2}>
            <TextField
              size="small"
              label="备注（可选）"
              value={sauthLabel}
              onChange={event => setSauthLabel(event.target.value)}
              fullWidth
            />
            <Button variant="contained" onClick={importSauth} disabled={sauthBusy} sx={{ whiteSpace: 'nowrap' }}>
              {sauthBusy ? '验证中…' : '验证并添加'}
            </Button>
          </Stack>
          {sauthProfile && (
            <Alert severity="success">
              游戏名：<b>{sauthProfile.name}</b>
              {sauthProfile.uid ? ` ｜ UID ${sauthProfile.uid}` : ''}
              {sauthProfile.expires_at ? ` ｜ 会话到期 ${new Date(sauthProfile.expires_at).toLocaleString('zh-CN')}` : ''}
            </Alert>
          )}
          {sauthError && <Alert severity="error">{sauthError}</Alert>}
        </Stack>
      </Box>

      {!compact && (
        <>
          <Divider />
          <Typography variant="caption" color="text.secondary">
            登录和导入接口仅允许本机控制台访问；密码不会保存，会话保存在本机加密存储中。
          </Typography>
        </>
      )}
    </Stack>
  );
}

import React, { useEffect, useRef, useState } from 'react';
import {
  Alert,
  Box,
  Button,
  Divider,
  IconButton,
  InputAdornment,
  MenuItem,
  Stack,
  TextField,
  Typography,
} from '@mui/material';
import { ContentCopy, Visibility, VisibilityOff } from '@mui/icons-material';
import { api } from './api.js';
import { useToast } from './App.jsx';
import { copyText } from './util.js';

export default function AccountLogin({
  onAdded,
  compact = false,
  replaceIndex = null,
  initialEmail = '',
  profiles = [],
  defaultProfileId = '',
  initialProfileId = '',
}) {
  const notify = useToast();
  const [email, setEmail] = useState(initialEmail);
  const [profileId, setProfileId] = useState(
    initialProfileId || defaultProfileId || profiles[0]?.id || '',
  );
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
  const requestRef = useRef(false);

  useEffect(() => {
    if (initialEmail) setEmail(initialEmail);
  }, [initialEmail, replaceIndex]);

  useEffect(() => {
    const next = initialProfileId || defaultProfileId || profiles[0]?.id || '';
    if (next) setProfileId(current => current || next);
  }, [initialProfileId, defaultProfileId, profiles]);

  const runLogin = async () => {
    if (requestRef.current) return;
    const credentials = { email: email.trim(), password, label: label.trim(), profile_id: profileId };
    if (replaceIndex !== null) credentials.index = replaceIndex;
    if (!credentials.email || !credentials.password) {
      setError('请填写网易邮箱和密码');
      return;
    }
    requestRef.current = true;
    setBusy(true);
    setError('');
    setMessage('');
    setVerifyURL('');
    try {
      const result = await api('session/interop-login', credentials);
      if (result.status === 'need_verify') {
        setVerifyURL(current => result.verify_url || current || '');
        setMessage(result.message || '需要完成网易安全验证');
        return;
      }
      setVerifyURL('');
      setMessage('');
      notify(
        replaceIndex !== null
          ? `已更新房间 ${(result.index ?? replaceIndex) + 1} 的登录：${result.name}`
          : `已添加互通版账号：${result.name} → 房间 ${(result.index ?? 0) + 1}`,
        'success',
      );
      setPassword('');
      if (onAdded) onAdded(result);
    } catch (failure) {
      setError(failure.message || '互通版直登失败');
    } finally {
      requestRef.current = false;
      setBusy(false);
    }
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
      setSauthError('请粘贴登录凭证');
      return;
    }
    setSauthBusy(true);
    setSauthError('');
    setSauthProfile(null);
    try {
      const payload = { sauth: value, label: sauthLabel.trim(), profile_id: profileId };
      if (replaceIndex !== null) payload.index = replaceIndex;
      const result = await api('session/import', payload);
      notify(
        replaceIndex !== null
          ? `已更新房间 ${(result.index ?? replaceIndex) + 1} 的登录：${result.name}`
          : `已导入互通版会话：${result.name} → 房间 ${(result.index ?? 0) + 1}`,
        'success',
      );
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
        <Alert severity={replaceIndex !== null ? 'info' : 'success'} sx={{ mb: 1.5 }}>
          {replaceIndex !== null
            ? `正在更新房间 ${replaceIndex + 1} 的登录。旧凭据过期后，点击刷新不会续期；请用邮箱密码或新的 sauth 重新登录。`
            : '推荐直接用网易邮箱和密码登录。密码只用于换取登录凭证，CubeDock 不会保存密码。'}
        </Alert>
        <Stack spacing={1.4}>
          {profiles.length > 0 && (
            <TextField
              select
              size="small"
              label="配置方案"
              value={profileId}
              onChange={event => setProfileId(event.target.value)}
              fullWidth
              helperText="这个网易账号使用哪套服务器和房间设置"
            >
              {profiles.map(profile => (
                <MenuItem key={profile.id} value={profile.id}>
                  {profile.name}（{profile.connection_mode === 'transfer' ? '转服' : '代理'} · {profile.upstream}）
                </MenuItem>
              ))}
            </TextField>
          )}
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
            <Button variant="contained" onClick={runLogin} disabled={busy} sx={{ whiteSpace: 'nowrap' }}>
              {busy
                ? replaceIndex !== null ? '更新中…' : '登录中…'
                : replaceIndex !== null ? '登录并更新' : '登录并添加'}
            </Button>
          </Stack>
          {verifyURL && (
            <Alert severity="warning">
              <Stack spacing={1.2}>
                <Typography variant="body2">
                  {message || '网易要求完成一次安全验证。'}
                </Typography>
                <Typography variant="body2">
                  请点击下方按钮完成验证，然后回到这里再次点击
                  {replaceIndex !== null ? '「登录并更新」' : '「登录并添加」'}。
                  CubeDock 不会自动重试，也不会在后台重复登录。
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
                </Stack>
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
          已经有登录凭证？可以粘贴 sauth 会话导入（高级用法）。
        </Alert>
        <Stack spacing={1.4}>
          <TextField
            multiline
            minRows={3}
            maxRows={8}
            label="登录凭证（sauth 会话）"
            placeholder="每行一个会话，或粘贴完整 JSON"
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
              {sauthBusy ? '验证中…' : replaceIndex !== null ? '验证并更新' : '验证并添加'}
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
            账号登录只允许在本机控制台操作；密码不会保存，登录凭证保存在本机。
          </Typography>
        </>
      )}
    </Stack>
  );
}

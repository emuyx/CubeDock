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
  Tab,
  Tabs,
  TextField,
  ToggleButton,
  ToggleButtonGroup,
  Typography,
} from '@mui/material';
import { ContentCopy, Visibility, VisibilityOff } from '@mui/icons-material';
import { api } from './api.js';
import { useToast } from './App.jsx';
import { copyText } from './util.js';

const normalizePlatform = value => (value === 'mobile_netease' ? 'mobile_netease' : 'interop');

export default function AccountLogin({
  onAdded,
  compact = false,
  replaceIndex = null,
  initialEmail = '',
  profiles = [],
  defaultProfileId = '',
  initialProfileId = '',
  initialAuthMethod = '',
}) {
  const notify = useToast();
  const [mode, setMode] = useState('password');
  const [platform, setPlatform] = useState(normalizePlatform(initialAuthMethod));
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

  const platformLabel = platform === 'mobile_netease' ? '网易手游' : '基岩互通版';
  const locked = busy || sauthBusy;

  useEffect(() => {
    if (initialEmail) setEmail(initialEmail);
  }, [initialEmail, replaceIndex]);

  useEffect(() => {
    setPlatform(normalizePlatform(initialAuthMethod));
  }, [initialAuthMethod, replaceIndex]);

  useEffect(() => {
    const next = initialProfileId || defaultProfileId || profiles[0]?.id || '';
    if (next) setProfileId(current => current || next);
  }, [initialProfileId, defaultProfileId, profiles]);

  const changePlatform = (_, next) => {
    if (!next || next === platform) return;
    setPlatform(next);
    setError('');
    setMessage('');
    setVerifyURL('');
    setSauth('');
    setSauthLabel('');
    setSauthError('');
    setSauthProfile(null);
  };

  const changeMode = (_, next) => {
    if (!next) return;
    setMode(next);
  };

  const runLogin = async () => {
    if (requestRef.current) return;
    const credentials = {
      email: email.trim(),
      password,
      label: label.trim(),
      profile_id: profileId,
    };
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
      const endpoint = platform === 'mobile_netease'
        ? 'session/mobile-login'
        : 'session/interop-login';
      const result = await api(endpoint, credentials);
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
          : `已添加${platformLabel}账号：${result.name} → 房间 ${(result.index ?? 0) + 1}`,
        'success',
      );
      setPassword('');
      if (onAdded) onAdded(result);
    } catch (failure) {
      setError(failure.message || `${platformLabel}直登失败`);
    } finally {
      requestRef.current = false;
      setBusy(false);
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
      const payload = {
        sauth: value,
        auth_method: platform,
        label: sauthLabel.trim(),
        profile_id: profileId,
      };
      if (replaceIndex !== null) payload.index = replaceIndex;
      const result = await api('session/import', payload);
      notify(
        replaceIndex !== null
          ? `已更新房间 ${(result.index ?? replaceIndex) + 1} 的登录：${result.name}`
          : `已导入${platformLabel}会话：${result.name} → 房间 ${(result.index ?? 0) + 1}`,
        'success',
      );
      setSauthProfile(result);
      setSauth('');
      setSauthLabel('');
      if (onAdded) onAdded(result);
    } catch (failure) {
      setSauthError(failure.message || `${platformLabel}会话验证失败`);
    } finally {
      setSauthBusy(false);
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

  const copyError = async text => {
    if (!text) return;
    try {
      await copyText(text);
      notify('完整错误已复制', 'success');
    } catch (failure) {
      notify(failure.message || '复制失败', 'error');
    }
  };

  const errorAlert = text => (
    <Alert severity="error">
      <Stack
        direction={{ xs: 'column', sm: 'row' }}
        spacing={1}
        alignItems={{ xs: 'stretch', sm: 'flex-start' }}
        justifyContent="space-between"
      >
        <Typography variant="body2" sx={{ whiteSpace: 'pre-wrap', wordBreak: 'break-word', flex: 1 }}>
          {text}
        </Typography>
        <Button
          size="small"
          color="inherit"
          startIcon={<ContentCopy />}
          onClick={() => copyError(text)}
          sx={{ flexShrink: 0, whiteSpace: 'nowrap', alignSelf: { xs: 'flex-end', sm: 'auto' } }}
        >
          复制完整错误
        </Button>
      </Stack>
    </Alert>
  );

  const profileField = profiles.length > 0 && (
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
  );

  const passwordPanel = (
    <Stack spacing={1.6}>
      {profileField}
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
      <TextField
        size="small"
        label="备注（可选）"
        value={label}
        onChange={event => setLabel(event.target.value)}
        fullWidth
      />
      <Button variant="contained" onClick={runLogin} disabled={busy} fullWidth>
        {busy
          ? replaceIndex !== null ? '更新中…' : '登录中…'
          : replaceIndex !== null ? '登录并更新' : '登录并添加'}
      </Button>
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
              <Button
                variant="outlined"
                color="warning"
                startIcon={<ContentCopy />}
                onClick={() => copyError([message, verifyURL].filter(Boolean).join('\n'))}
              >
                复制完整信息
              </Button>
            </Stack>
          </Stack>
        </Alert>
      )}
      {!verifyURL && message && <Alert severity="warning">{message}</Alert>}
      {error && errorAlert(error)}
      <Typography variant="caption" color="text.secondary">
        密码只用于换取登录凭证，CubeDock 不会保存密码。
      </Typography>
    </Stack>
  );

  const sauthPanel = (
    <Stack spacing={1.6}>
      {profileField}
      <Typography variant="caption" color="text.secondary">
        已有 {platformLabel} 登录凭证时，可直接粘贴 sauth 会话导入。
      </Typography>
      <TextField
        multiline
        minRows={4}
        maxRows={10}
        label={platform === 'mobile_netease' ? '手游 sauth/session JSON' : '登录凭证（互通版 sauth 会话）'}
        placeholder={
          platform === 'mobile_netease'
            ? '粘贴包含 platform=ad、app_channel、sessionid、access_token 的 JSON'
            : '每行一个会话，或粘贴完整 JSON'
        }
        value={sauth}
        onChange={event => setSauth(event.target.value)}
        fullWidth
        sx={{ '& textarea': { fontFamily: 'ui-monospace, monospace', fontSize: 12 } }}
      />
      <TextField
        size="small"
        label="备注（可选）"
        value={sauthLabel}
        onChange={event => setSauthLabel(event.target.value)}
        fullWidth
      />
      <Button variant="contained" onClick={importSauth} disabled={sauthBusy} fullWidth>
        {sauthBusy ? '验证中…' : replaceIndex !== null ? '验证并更新' : '验证并添加'}
      </Button>
      {sauthProfile && (
        <Alert severity="success" sx={{ wordBreak: 'break-word' }}>
          游戏名：<b>{sauthProfile.name}</b>
          {sauthProfile.uid ? ` ｜ UID ${sauthProfile.uid}` : ''}
          {sauthProfile.auth_method === 'mobile_netease'
            ? ' ｜ 网易手游'
            : sauthProfile.auth_method === 'mobile_4399'
              ? ' ｜ 4399 手游'
              : ' ｜ 基岩互通版'}
          {sauthProfile.expires_at ? ` ｜ 会话到期 ${new Date(sauthProfile.expires_at).toLocaleString('zh-CN')}` : ''}
        </Alert>
      )}
      {sauthError && errorAlert(sauthError)}
    </Stack>
  );

  return (
    <Stack spacing={compact ? 1.8 : 2.2}>
      {replaceIndex !== null && (
        <Alert severity="info">
          正在更新房间 {replaceIndex + 1} 的登录。旧凭据过期后，请用邮箱密码或新的 sauth 重新登录。
        </Alert>
      )}

      <Box>
        <Tabs
          value={mode}
          onChange={changeMode}
          variant="fullWidth"
          aria-label="登录方式"
          sx={{
            minHeight: 42,
            '& .MuiTab-root': { minHeight: 42, py: 1, px: 1 },
          }}
        >
          <Tab value="password" label="普通登录" disabled={locked} />
          <Tab value="sauth" label="Sauth 登录" disabled={locked} />
        </Tabs>
        <Box sx={{ pt: 1.5 }}>
          <ToggleButtonGroup
            exclusive
            fullWidth
            size="small"
            value={platform}
            onChange={changePlatform}
            aria-label="登录平台"
          >
            <ToggleButton value="interop" disabled={locked}>
              基岩互通版
            </ToggleButton>
            <ToggleButton value="mobile_netease" disabled={locked}>
              网易手游
            </ToggleButton>
          </ToggleButtonGroup>
          <Typography variant="caption" color="text.secondary" sx={{ display: 'block', mt: 0.75 }}>
            {platform === 'mobile_netease'
              ? '手游邮箱/密码，或手游 sauth 会话'
              : '互通版邮箱/密码，或互通版 sauth 会话'}
          </Typography>
        </Box>
      </Box>

      <Divider />

      {mode === 'password' ? passwordPanel : sauthPanel}

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

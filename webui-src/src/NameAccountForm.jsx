import React, { useState } from 'react';
import { Alert, Box, Button, Chip, TextField } from '@mui/material';
import Typography from './Typography.jsx';
import Stack from './Stack.jsx';
import { Casino, Refresh } from '@mui/icons-material';
import { api } from './api.js';
import { useToast } from './App.jsx';
import {
  isValidName,
  makeNameSuggestions,
  nameWeight,
  randomChineseName,
  randomEnglishName,
} from './names.js';

export default function NameAccountForm({
  index,
  account = null,
  initialName = '',
  mode = 'name',
  onDone,
  onSkip,
  compact = false,
}) {
  const notify = useToast();
  const rename = mode === 'rename';
  const [name, setName] = useState(initialName);
  const [suggestions, setSuggestions] = useState(() => makeNameSuggestions());
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');

  const valid = isValidName(name);
  const weight = name.trim() ? nameWeight(name).toFixed(1) : '0.0';

  const refreshSuggestions = () => setSuggestions(makeNameSuggestions());

  const submit = async () => {
    if (busy) return;
    const trimmed = name.trim();
    if (!isValidName(trimmed)) {
      setError('游戏名长度需为 3–12 个字符（中文按 1.5 个字符计算）');
      return;
    }
    setBusy(true);
    setError('');
    try {
      const result = await api('account/set-name', {
        index,
        name: trimmed,
        uid: account?.uid || '',
        email: account?.email || '',
      });
      notify(`${rename ? '游戏名已修改' : '游戏名已设置'}：${result.name}`, 'success');
      onDone?.(result);
    } catch (failure) {
      setError(failure.message || '设置游戏名失败');
    } finally {
      setBusy(false);
    }
  };

  return (
    <Stack
      component="form"
      onSubmit={event => {
        event.preventDefault();
        submit();
      }}
      spacing={compact ? 1.6 : 2}
    >
      <Alert severity={rename ? 'warning' : 'info'}>
        {rename
          ? '网易限制：游戏名修改后 30 天内无法再次修改。若仍在限制期，保存时会显示网易返回的错误。'
          : '该账号还没有游戏名。设置后网易才会返回完整资料；游戏名需为 3–12 个字符，中文按 1.5 个字符计算。设置后网易限制 30 天内不能再次修改，请确认后再保存。'}
      </Alert>
      <TextField
        label={rename ? '新的游戏名' : '游戏名'}
        value={name}
        onChange={event => {
          setName(event.target.value);
          setError('');
        }}
        fullWidth
        autoComplete="off"
        helperText={`${weight} / 12 个字符；可直接手动输入，也可使用下方随机候选`}
        error={!!name.trim() && !valid}
      />
      <Stack direction="row" spacing={1} flexWrap="wrap" useFlexGap>
        <Button
          type="button"
          size="small"
          variant="outlined"
          startIcon={<Casino />}
          onClick={() => setName(randomChineseName())}
          disabled={busy}
        >
          随机中文名
        </Button>
        <Button
          type="button"
          size="small"
          variant="outlined"
          startIcon={<Casino />}
          onClick={() => setName(randomEnglishName())}
          disabled={busy}
        >
          随机英文+数字
        </Button>
        <Button
          type="button"
          size="small"
          startIcon={<Refresh />}
          onClick={refreshSuggestions}
          disabled={busy}
        >
          换一批
        </Button>
      </Stack>
      <Box>
        <Typography variant="caption" color="text.secondary">中文候选</Typography>
        <Stack direction="row" spacing={0.7} flexWrap="wrap" useFlexGap sx={{ mt: 0.6 }}>
          {suggestions.chinese.map(candidate => (
            <Chip
              key={candidate}
              label={candidate}
              size="small"
              clickable
              onClick={() => setName(candidate)}
              disabled={busy}
            />
          ))}
        </Stack>
      </Box>
      <Box>
        <Typography variant="caption" color="text.secondary">英文+数字候选</Typography>
        <Stack direction="row" spacing={0.7} flexWrap="wrap" useFlexGap sx={{ mt: 0.6 }}>
          {suggestions.english.map(candidate => (
            <Chip
              key={candidate}
              label={candidate}
              size="small"
              clickable
              onClick={() => setName(candidate)}
              disabled={busy}
            />
          ))}
        </Stack>
      </Box>
      {error && <Alert severity="error">{error}</Alert>}
      <Stack direction="row" spacing={1} justifyContent="flex-end" flexWrap="wrap" useFlexGap>
        {onSkip && (
          <Button type="button" onClick={onSkip} disabled={busy} color="inherit">
            稍后设置
          </Button>
        )}
        <Button type="submit" variant="contained" disabled={busy || !valid}>
          {busy ? '保存中…' : rename ? '保存新游戏名' : '保存游戏名'}
        </Button>
      </Stack>
    </Stack>
  );
}

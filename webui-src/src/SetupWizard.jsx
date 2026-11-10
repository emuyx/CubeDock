import React, { useState } from 'react';
import {
  Alert,
  Box,
  Button,
  Card,
  CardContent,
  CircularProgress,
  MenuItem,
  Stack,
  Step,
  StepLabel,
  Stepper,
  TextField,
  Typography,
} from '@mui/material';
import { ArrowBack, PlayArrow, SkipNext } from '@mui/icons-material';
import { api } from './api.js';
import { useToast } from './App.jsx';
import { useConfigForm } from './configForm.jsx';
import AccountLogin from './AccountLogin.jsx';
import DiagnosticsPanel from './DiagnosticsPanel.jsx';
import { FieldHint } from './HelpTip.jsx';

const STEPS = ['连接设置', '添加账号', '运行预检', '开始房间'];
const grid = { display: 'grid', gap: 2, gridTemplateColumns: { xs: '1fr', sm: '1fr 1fr' } };

export default function SetupWizard({ status, onChanged, onDone }) {
  const notify = useToast();
  const { form, update, save, busy, error } = useConfigForm();
  const [step, setStep] = useState(0);
  const [starting, setStarting] = useState(false);
  const [startError, setStartError] = useState('');
  const accounts = status.accounts || [];
  const running = !!status.running;

  if (!form) {
    return (
      <Box sx={{ py: 8, display: 'grid', placeItems: 'center' }}>
        <CircularProgress />
      </Box>
    );
  }

  const transfer = form.connection_mode === 'transfer';
  const next = () => setStep(current => Math.min(current + 1, STEPS.length - 1));
  const back = () => setStep(current => Math.max(current - 1, 0));

  const saveAndNext = async () => {
    if (await save()) next();
  };

  const start = async () => {
    setStarting(true);
    setStartError('');
    try {
      await api('start', {});
      notify('房间已启动', 'success');
      onChanged?.();
      onDone?.();
    } catch (failure) {
      setStartError(failure.message || '启动房间失败');
    } finally {
      setStarting(false);
    }
  };

  return (
    <Card>
      <CardContent sx={{ p: { xs: 2.4, sm: 3.4 } }}>
        <Stack spacing={2.6}>
          <Stack direction="row" justifyContent="space-between" alignItems="flex-start" spacing={2}>
            <Box>
              <Typography variant="overline" color="primary" fontWeight={800}>
                首次设置
              </Typography>
              <Typography variant="h4">几步完成 CubeDock 配置</Typography>
              <Typography color="text.secondary" sx={{ mt: 0.8 }}>
                先确认服务器地址和连接方式，再添加网易账号，最后运行预检并开房。
              </Typography>
            </Box>
            <Button color="inherit" startIcon={<SkipNext />} onClick={onDone}>
              跳过向导
            </Button>
          </Stack>

          <Stepper activeStep={step} alternativeLabel sx={{ display: { xs: 'none', sm: 'flex' } }}>
            {STEPS.map(label => (
              <Step key={label}>
                <StepLabel>{label}</StepLabel>
              </Step>
            ))}
          </Stepper>

          {error && <Alert severity="error">{error}</Alert>}

          {step === 0 && (
            <Stack spacing={2.2}>
              <Alert severity="info">
                代理模式：玩家流量经 CubeDock 转发到服务器。转服模式：玩家先进入本地中转房间，
                点击按钮后再进入服务器。两种方式都要求服务器支持网易协议。
              </Alert>
              <Box sx={grid}>
                <TextField
                  select
                  label="连接模式"
                  value={form.connection_mode || 'proxy'}
                  onChange={event => update('connection_mode', event.target.value)}
                  disabled={running || busy}
                  fullWidth
                >
                  <MenuItem value="proxy">代理模式（推荐）</MenuItem>
                  <MenuItem value="transfer">转服模式（先进中转房间）</MenuItem>
                </TextField>
                <TextField
                  label="服务器地址"
                  value={form.upstream || ''}
                  onChange={event => update('upstream', event.target.value)}
                  disabled={running || busy}
                  fullWidth
                  required
                  helperText={
                    <FieldHint help="服务器必须支持网易协议（RakNet v8）。代理和转服模式都一样；标准 Bedrock 服务器不能直接用。">
                      IP:端口，例如 127.0.0.1:19132
                    </FieldHint>
                  }
                />
              </Box>
              {transfer && (
                <TextField
                  label="中转房间监听地址"
                  value={form.limbo_upstream || ''}
                  onChange={event => update('limbo_upstream', event.target.value)}
                  disabled={running || busy}
                  fullWidth
                  helperText={
                    <FieldHint help="仅转服模式使用。玩家会先进入这个本地中转房间，一般保持 127.0.0.1:19133。">
                      例如 127.0.0.1:19133
                    </FieldHint>
                  }
                />
              )}
              <Box sx={grid}>
                <TextField
                  label="房间名称"
                  value={form.room_name || ''}
                  onChange={event => update('room_name', event.target.value)}
                  disabled={running || busy}
                  fullWidth
                  inputProps={{ maxLength: 40 }}
                />
                <TextField
                  label="房间密码（可选）"
                  type="password"
                  value={form.room_password || ''}
                  onChange={event => update('room_password', event.target.value)}
                  disabled={running || busy}
                  fullWidth
                  autoComplete="off"
                />
              </Box>
              <Stack direction="row" justifyContent="flex-end">
                <Button variant="contained" onClick={saveAndNext} disabled={running || busy}>
                  {busy ? '保存中…' : '保存并继续'}
                </Button>
              </Stack>
            </Stack>
          )}

          {step === 1 && (
            <Stack spacing={2.2}>
              <Alert severity={accounts.length ? 'success' : 'info'}>
                {accounts.length
                  ? `已添加 ${accounts.length} 个账号，可以继续预检。`
                  : '添加第一个网易账号。每个账号对应一个本地联机房间。'}
              </Alert>
              <AccountLogin compact onAdded={() => onChanged?.()} />
              <Stack direction="row" justifyContent="space-between">
                <Button startIcon={<ArrowBack />} onClick={back}>
                  上一步
                </Button>
                <Button variant="contained" onClick={next} disabled={accounts.length === 0}>
                  继续
                </Button>
              </Stack>
            </Stack>
          )}

          {step === 2 && (
            <Stack spacing={2.2}>
              <Typography color="text.secondary">
                预检不会登录网易账号。失败项可以继续，但开房前建议先处理。
              </Typography>
              <DiagnosticsPanel status={status} active />
              <Stack direction="row" justifyContent="space-between">
                <Button startIcon={<ArrowBack />} onClick={back}>
                  上一步
                </Button>
                <Button variant="contained" onClick={next}>
                  继续
                </Button>
              </Stack>
            </Stack>
          )}

          {step === 3 && (
            <Stack spacing={2.2}>
              {startError && <Alert severity="error">{startError}</Alert>}
              <Alert severity={running ? 'success' : 'info'}>
                {running
                  ? '房间已在运行。可以关闭向导，回到控制台查看房间与玩家。'
                  : `将启动 ${accounts.length} 个房间账号。启动后可在控制台查看房间号和玩家。`}
              </Alert>
              <Stack spacing={0.6}>
                <Typography variant="body2" color="text.secondary">
                  连接方式：{transfer ? '转服模式（先进中转房间）' : '代理模式'}
                </Typography>
                <Typography variant="body2" color="text.secondary">
                  服务器：{form.upstream || '未填写'}
                </Typography>
                <Typography variant="body2" color="text.secondary">
                  账号：{accounts.length} 个
                </Typography>
              </Stack>
              <Stack direction="row" justifyContent="space-between">
                <Button startIcon={<ArrowBack />} onClick={back}>
                  上一步
                </Button>
                <Stack direction="row" spacing={1}>
                  <Button onClick={onDone}>稍后开始</Button>
                  <Button
                    variant="contained"
                    startIcon={starting ? <CircularProgress size={16} color="inherit" /> : <PlayArrow />}
                    onClick={start}
                    disabled={starting || running || accounts.length === 0}
                  >
                    {running ? '房间运行中' : starting ? '启动中…' : '启动房间'}
                  </Button>
                </Stack>
              </Stack>
            </Stack>
          )}
        </Stack>
      </CardContent>
    </Card>
  );
}

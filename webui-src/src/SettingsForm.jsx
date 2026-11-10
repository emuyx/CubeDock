import React, { useRef, useState } from 'react';
import {
  Alert,
  Box,
  Button,
  Card,
  CardContent,
  CircularProgress,
  FormControlLabel,
  Stack,
  Switch,
  Tab,
  Tabs,
  TextField,
  Typography,
} from '@mui/material';
import { Download, Save, UploadFile } from '@mui/icons-material';
import { getBackup, restoreConfig } from './api.js';
import { useConfigForm } from './configForm.jsx';
import { downloadText, timestamp } from './util.js';
import { useToast } from './App.jsx';
import HelpTip, { FieldHint } from './HelpTip.jsx';

export default function SettingsForm({ status, onChanged }) {
  const notify = useToast();
  const { form, update, save, busy, error, discard } = useConfigForm();
  const running = !!status.running;
  const [tab, setTab] = useState(0);
  const [includePassword, setIncludePassword] = useState(false);
  const [restoring, setRestoring] = useState(false);
  const [backupError, setBackupError] = useState('');
  const fileInput = useRef(null);

  const backup = async () => {
    setBackupError('');
    try {
      const payload = await getBackup(includePassword);
      downloadText(`cubedock-backup-${timestamp()}.json`, JSON.stringify(payload, null, 2));
      notify('配置备份已下载', 'success');
    } catch (failure) {
      setBackupError(failure.message || '导出备份失败');
    }
  };

  const restore = async event => {
    const file = event.target.files?.[0];
    event.target.value = '';
    if (!file) return;
    if (!window.confirm('恢复配置会覆盖当前设置，且不会恢复网易登录凭证。确认继续？')) return;
    setRestoring(true);
    setBackupError('');
    try {
      const parsed = JSON.parse(await file.text());
      if (!parsed?.config) throw new Error('这个文件不是 CubeDock 的配置备份');
      const result = await restoreConfig(parsed.config);
      discard?.();
      onChanged?.();
      notify(
        result.warnings?.length ? `配置已恢复；${result.warnings.join('；')}` : '配置已恢复',
        result.warnings?.length ? 'warning' : 'success',
      );
    } catch (failure) {
      setBackupError(failure.message || '恢复配置失败');
    } finally {
      setRestoring(false);
    }
  };

  if (!form) {
    return (
      <Box sx={{ py: 8, display: 'grid', placeItems: 'center' }}>
        <CircularProgress />
      </Box>
    );
  }

  const grid = { display: 'grid', gap: 2, gridTemplateColumns: { xs: '1fr', md: '1fr 1fr' } };

  return (
    <Card component="form" onSubmit={save}>
      <CardContent sx={{ p: { xs: 2.4, sm: 3.2 } }}>
        <Stack spacing={2.4}>
          <Box>
            <Typography variant="h6">设置</Typography>
            <Typography variant="body2" color="text.secondary">
              服务器和房间常用项在「房间」，好友与自动开房在「行为」，兼容性参数在「高级」。
            </Typography>
          </Box>
          {running && <Alert severity="info">房间运行中，设置暂时不可修改。</Alert>}
          {error && <Alert severity="error">{error}</Alert>}

          <Tabs
            value={tab}
            onChange={(_, value) => setTab(value)}
            variant="scrollable"
            allowScrollButtonsMobile
          >
            <Tab label="房间" />
            <Tab label="行为" />
            <Tab label="高级" />
            <Tab label="备份" />
          </Tabs>

          {tab === 0 && (
            <Stack spacing={2.4}>
              {(status.accounts || []).some(account => account.platform === 'interop') && (
                <Alert severity="info">使用互通版账号即可，不需要选择网易客户端版本。</Alert>
              )}
              <Box sx={grid}>
                <TextField
                  label="服务器地址"
                  value={form.upstream || ''}
                  onChange={event => update('upstream', event.target.value)}
                  disabled={running}
                  required
                  fullWidth
                  helperText={
                    <FieldHint help="服务器必须支持网易协议（RakNet v8）。代理和转服模式都一样；标准 Bedrock 服务器不能直接用。">
                      IP:端口，例如 127.0.0.1:19132
                    </FieldHint>
                  }
                />
                <TextField
                  label="房间名称"
                  value={form.room_name || ''}
                  onChange={event => update('room_name', event.target.value)}
                  disabled={running}
                  required
                  fullWidth
                  inputProps={{ maxLength: 40 }}
                  helperText={<FieldHint help="玩家在网易联机列表里看到的房间名。">1–40 个字</FieldHint>}
                />
              </Box>
              <Box sx={grid}>
                <TextField
                  label="房间密码（可选）"
                  type="password"
                  value={form.room_password || ''}
                  onChange={event => update('room_password', event.target.value)}
                  disabled={running}
                  fullWidth
                  autoComplete="off"
                  helperText={<FieldHint help="留空表示无密码。密码最多 64 个字符。">留空表示无密码</FieldHint>}
                />
                <TextField
                  label="房间总人数（含 CubeDock）"
                  type="number"
                  value={form.max_players ?? 10}
                  onChange={event => update('max_players', Number(event.target.value))}
                  disabled={running}
                  fullWidth
                  inputProps={{ min: 2, max: 10 }}
                  helperText={
                    <FieldHint help="网易房间最多 10 人，CubeDock 自己占 1 个名额，所以最多 9 位玩家。">
                      2–10 人
                    </FieldHint>
                  }
                />
              </Box>
            </Stack>
          )}

          {tab === 1 && (
            <Stack spacing={2}>
              <Box
                sx={{
                  display: 'grid',
                  gap: 1.2,
                  gridTemplateColumns: { xs: '1fr', sm: '1fr 1fr' },
                }}
              >
                <Stack direction="row" spacing={0.2} alignItems="center">
                  <FormControlLabel
                    control={<Switch checked={!!form.friends_only} onChange={event => update('friends_only', event.target.checked)} disabled={running} />}
                    label="仅好友可见"
                  />
                  <HelpTip title="开启后只有你的网易好友能看到并进入房间。" />
                </Stack>
                <Stack direction="row" spacing={0.2} alignItems="center">
                  <FormControlLabel
                    control={<Switch checked={!!form.auto_accept_friends} onChange={event => update('auto_accept_friends', event.target.checked)} disabled={running} />}
                    label="自动同意好友申请"
                  />
                  <HelpTip title="开启「仅好友可见」时会自动跳过，避免陌生人通过好友申请进入房间。" />
                </Stack>
                <Stack direction="row" spacing={0.2} alignItems="center">
                  <FormControlLabel
                    control={<Switch checked={!!form.allow_pvp} onChange={event => update('allow_pvp', event.target.checked)} disabled={running} />}
                    label="允许 PvP"
                  />
                  <HelpTip title="只影响网易房间的 PvP 标记，实际规则仍由服务器决定。" />
                </Stack>
                <Stack direction="row" spacing={0.2} alignItems="center">
                  <FormControlLabel
                    control={<Switch checked={!!form.auto_start} onChange={event => update('auto_start', event.target.checked)} disabled={running} />}
                    label="打开程序时自动开房"
                  />
                  <HelpTip title="适合长期挂机的机器；如果服务器或账号还没准备好，建议保持关闭。" />
                </Stack>
              </Box>
              <Typography variant="caption" color="text.secondary">
                需要排查问题时，可以在「控制台」页打开详细日志，立即生效，不用停止房间。
              </Typography>
              <Alert severity="info" variant="outlined">
                CubeDock 不会把网易协议转换成标准 Bedrock。无论代理还是转服，服务器都必须支持网易协议。
              </Alert>
            </Stack>
          )}

          {tab === 2 && (
            <Stack spacing={2}>
              <Alert severity="warning" variant="outlined">
                这些是兼容性参数。除非确认网易客户端或服务器版本变化，否则保持默认。
              </Alert>
              <Box sx={grid}>
                <TextField
                  label="房间版本"
                  value={form.room_game_version || ''}
                  onChange={event => update('room_game_version', event.target.value)}
                  disabled={running}
                  required
                  fullWidth
                  helperText={
                    <FieldHint help="一般保持 1.21.130.0。只有网易更新客户端、确认版本变化后才需要改。">
                      当前为 1.21.130.0
                    </FieldHint>
                  }
                />
                <TextField
                  label="房间协议版本"
                  type="number"
                  value={form.lobby_protocol_id ?? 43}
                  onChange={event => update('lobby_protocol_id', Number(event.target.value))}
                  disabled={running}
                  fullWidth
                  inputProps={{ min: 1, max: 255 }}
                  helperText={
                    <FieldHint help="默认 43。只有确认客户端或服务器版本变化时才需要改。">
                      默认 43
                    </FieldHint>
                  }
                />
                <TextField
                  label="掉线重试间隔（秒）"
                  type="number"
                  value={form.retry_seconds ?? 10}
                  onChange={event => update('retry_seconds', Number(event.target.value))}
                  disabled={running}
                  fullWidth
                  inputProps={{ min: 5, max: 300 }}
                  helperText={<FieldHint help="房间掉线后等待多久重连，默认 10 秒。">5–300 秒</FieldHint>}
                />
              </Box>
            </Stack>
          )}

          {tab === 3 && (
            <Stack spacing={2}>
              <Box>
                <Typography variant="h6">配置备份与恢复</Typography>
                <Typography variant="body2" color="text.secondary" sx={{ mt: 0.4 }}>
                  备份包含配置和账号信息（邮箱、游戏名、UID、到期时间），不包含网易登录凭证；
                  恢复后需要重新登录或导入账号。恢复会覆盖当前设置，房间运行中不可恢复。
                </Typography>
              </Box>
              {backupError && <Alert severity="error">{backupError}</Alert>}
              <Stack
                direction={{ xs: 'column', sm: 'row' }}
                spacing={1}
                flexWrap="wrap"
                useFlexGap
                alignItems={{ xs: 'stretch', sm: 'center' }}
              >
                <FormControlLabel
                  control={
                    <Switch
                      checked={includePassword}
                      onChange={event => setIncludePassword(event.target.checked)}
                    />
                  }
                  label="备份包含房间密码"
                />
                <Button variant="outlined" startIcon={<Download />} onClick={backup}>
                  下载配置备份
                </Button>
                <Button
                  variant="outlined"
                  startIcon={<UploadFile />}
                  onClick={() => fileInput.current?.click()}
                  disabled={running || restoring}
                >
                  {restoring ? '恢复中…' : '从备份恢复'}
                </Button>
                <input
                  ref={fileInput}
                  type="file"
                  accept="application/json,.json"
                  hidden
                  onChange={restore}
                />
              </Stack>
            </Stack>
          )}

          {tab !== 3 && (
            <Stack direction="row" justifyContent="flex-end">
              <Button type="submit" variant="contained" startIcon={<Save />} disabled={running || busy}>
                {busy ? '保存中…' : '保存设置'}
              </Button>
            </Stack>
          )}
        </Stack>
      </CardContent>
    </Card>
  );
}

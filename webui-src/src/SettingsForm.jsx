import React, { useRef, useState } from 'react';
import {
  Alert,
  Box,
  Button,
  CircularProgress,
  Divider,
  FormControlLabel,
  Switch,
  Tab,
  Tabs,
  TextField,
} from '@mui/material';
import Typography from './Typography.jsx';
import Stack from './Stack.jsx';
import { Download, Save, UploadFile } from '@mui/icons-material';
import { getBackup, restoreConfig } from './api.js';
import { useConfigForm } from './configForm.jsx';
import { downloadText, timestamp } from './util.js';
import { useConfirm, useToast } from './App.jsx';
import { FieldHint } from './HelpTip.jsx';

const grid = { display: 'grid', gap: 2, gridTemplateColumns: { xs: '1fr', md: '1fr 1fr' } };

export default function SettingsForm({ status, onChanged }) {
  const notify = useToast();
  const confirm = useConfirm();
  const { form, update, save, busy, error, externalChanged, reload, discard } = useConfigForm();
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
    const confirmed = await confirm({
      title: '恢复配置',
      message: '恢复配置会覆盖当前方案和全局设置，且不会恢复网易登录凭证或账号的方案选择。确认继续？',
      confirmLabel: '恢复配置',
      severity: 'warning',
    });
    if (!confirmed) return;
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

  return (
    <Box component="form" onSubmit={save}>
      <Stack spacing={2.4}>
        <Box>
          <Typography variant="h5">全局设置</Typography>
          <Typography variant="body2" color="text.secondary" sx={{ mt: 0.3 }}>
            服务器、房间、资源包和提示页面在「配置方案」中按方案编辑。
          </Typography>
        </Box>
        {running && <Alert severity="info">有房间运行中，全局设置暂时不可修改。</Alert>}
        {externalChanged && (
          <Alert
            severity="warning"
            action={
              <Button color="inherit" size="small" onClick={reload}>
                加载最新
              </Button>
            }
          >
            服务端配置已在别处更新；当前修改尚未保存。
          </Alert>
        )}
        {error && <Alert severity="error">{error}</Alert>}

        <Tabs value={tab} onChange={(_, value) => setTab(value)} variant="scrollable" allowScrollButtonsMobile>
          <Tab label="常规" />
          <Tab label="备份" />
        </Tabs>
        <Divider />

        {tab === 0 && (
          <Stack spacing={2.4}>
            <Typography variant="caption" color="text.secondary">
              当前共有 {(form.profiles || []).length} 个配置方案 · 房间/服务器设置请到「配置方案」页编辑
            </Typography>
            <Box sx={grid}>
              <TextField
                label="limbo 程序路径（可选）"
                value={form.limbo_path || ''}
                onChange={event => update('limbo_path', event.target.value)}
                disabled={running}
                fullWidth
                helperText={
                  <FieldHint help="所有转服方案共用这一个辅助程序。留空时使用主程序同目录的 limbo（Windows 为 limbo.exe）。">
                    留空时自动查找
                  </FieldHint>
                }
              />
              <TextField
                label="控制台端口"
                value={form.web_port ?? 18321}
                disabled
                fullWidth
                helperText="关闭程序后在 config.json 修改"
              />
            </Box>
            <Box>
              <FormControlLabel
                control={<Switch checked={!!form.auto_start} onChange={event => update('auto_start', event.target.checked)} disabled={running} />}
                label="打开程序时自动启动全部房间"
              />
              <Typography variant="caption" color="text.secondary" display="block" sx={{ ml: 6 }}>
                每个账号使用自己选择的配置方案；账号或服务器未准备好时建议关闭。
              </Typography>
            </Box>
            <Box>
              <FormControlLabel
                control={
                  <Switch
                    checked={!!form.keep_alive}
                    onChange={event => update('keep_alive', event.target.checked)}
                    disabled={running}
                  />
                }
                label="空闲账号保活"
              />
              <TextField
                label="保活间隔（分钟）"
                type="number"
                value={form.keep_alive_minutes ?? 30}
                onChange={event => update('keep_alive_minutes', Number(event.target.value))}
                disabled={running || !form.keep_alive}
                inputProps={{ min: 5, max: 1440 }}
                sx={{ maxWidth: 220, mt: 1, ml: { xs: 0, sm: 6 } }}
                helperText="未开房账号会定期刷新资料/头像；不会登录已过期账号。"
              />
            </Box>
            <Typography variant="caption" color="text.secondary">
              Debug 详细日志在「控制台」页即时开关。
            </Typography>
            <Divider />
            <Box sx={{ display: 'flex', justifyContent: 'flex-end' }}>
              <Button
                type="submit"
                variant="contained"
                startIcon={<Save />}
                disabled={running || busy}
                sx={{ width: { xs: '100%', sm: 'auto' } }}
              >
                {busy ? '保存中…' : '保存设置'}
              </Button>
            </Box>
          </Stack>
        )}

        {tab === 1 && (
          <Stack spacing={2}>
            <Box>
              <Typography variant="h6">配置备份与恢复</Typography>
              <Typography variant="body2" color="text.secondary" sx={{ mt: 0.4 }}>
                备份包含全部配置方案、全局设置和账号信息（邮箱、游戏名、UID、到期时间），不包含网易登录凭证；
                恢复后需要重新登录或导入账号，账号的方案选择保持不变。
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
              <Button variant="contained" startIcon={<Download />} onClick={backup}>
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
      </Stack>
    </Box>
  );
}

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
import { useConfirm, useToast } from './App.jsx';
import { FieldHint } from './HelpTip.jsx';

const grid = { display: 'grid', gap: 2, gridTemplateColumns: { xs: '1fr', md: '1fr 1fr' } };

export default function SettingsForm({ status, onChanged }) {
  const notify = useToast();
  const confirm = useConfirm();
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
    <Card component="form" onSubmit={save}>
      <CardContent sx={{ p: { xs: 2.4, sm: 3.2 } }}>
        <Stack spacing={2.4}>
          <Box>
            <Typography variant="h6">设置</Typography>
            <Typography variant="body2" color="text.secondary">
              这里只保留全局设置；服务器、房间、资源包和提示页面在「配置方案」中按方案编辑。
            </Typography>
          </Box>
          {running && <Alert severity="info">有房间运行中，全局设置暂时不可修改。</Alert>}
          {error && <Alert severity="error">{error}</Alert>}

          <Tabs value={tab} onChange={(_, value) => setTab(value)} variant="scrollable" allowScrollButtonsMobile>
            <Tab label="常规" />
            <Tab label="备份" />
          </Tabs>

          {tab === 0 && (
            <Stack spacing={2.4}>
              <Alert severity="info" variant="outlined">
                当前共有 {(form.profiles || []).length} 个配置方案。房间/服务器相关设置请到「配置方案」页编辑。
              </Alert>
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
              <Stack direction="row" spacing={0.2} alignItems="center">
                <FormControlLabel
                  control={<Switch checked={!!form.auto_start} onChange={event => update('auto_start', event.target.checked)} disabled={running} />}
                  label="打开程序时自动启动全部房间"
                />
                <FieldHint help="每个账号使用自己选择的配置方案。账号或服务器没准备好时建议保持关闭。" />
              </Stack>
              <Alert severity="info" variant="outlined">
                Debug 详细日志在「控制台」页即时开关，不在这里修改。
              </Alert>
              <Stack direction="row" justifyContent="flex-end">
                <Button type="submit" variant="contained" startIcon={<Save />} disabled={running || busy}>
                  {busy ? '保存中…' : '保存设置'}
                </Button>
              </Stack>
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
        </Stack>
      </CardContent>
    </Card>
  );
}

import React, { useEffect, useState } from 'react';
import {
  Alert,
  Box,
  Button,
  Card,
  CardContent,
  CircularProgress,
  Divider,
  FormControlLabel,
  MenuItem,
  Stack,
  Switch,
  TextField,
  Typography,
} from '@mui/material';
import { CloudDownload, Refresh, Save } from '@mui/icons-material';
import { api } from './api.js';
import { useConfigForm } from './configForm.jsx';

function formatBytes(size) {
  if (!size) return '0 B';
  if (size < 1024) return `${size} B`;
  if (size < 1024 * 1024) return `${(size / 1024).toFixed(1)} KB`;
  return `${(size / 1024 / 1024).toFixed(1)} MB`;
}

function splitLimbo(value) {
  const text = value || '127.0.0.1:19133';
  const index = text.lastIndexOf(':');
  if (index < 0) {
    return ['127.0.0.1', '19133'];
  }
  return [text.slice(0, index) || '127.0.0.1', text.slice(index + 1) || '19133'];
}

export default function TransferPanel({ status, active }) {
  const { form, update, save, busy, error } = useConfigForm();
  const running = !!status.running;
  const transfer = form?.connection_mode === 'transfer';
  const [packs, setPacks] = useState(null);
  const [packsBusy, setPacksBusy] = useState(false);
  const [packsRefreshing, setPacksRefreshing] = useState(false);
  const [packsError, setPacksError] = useState('');
  const [packsNotice, setPacksNotice] = useState('');

  const loadPacks = async () => {
    setPacksBusy(true);
    setPacksError('');
    try {
      setPacks(await api('packs', undefined, 'GET'));
    } catch (failure) {
      setPacksError(failure.message || '读取资源包列表失败');
    } finally {
      setPacksBusy(false);
    }
  };

  const refreshFromBackend = async () => {
    setPacksRefreshing(true);
    setPacksError('');
    setPacksNotice('');
    try {
      const result = await api('packs/refresh', {});
      setPacks(result);
      const pulled = result.pulled || 0;
      const skipped = result.skipped || 0;
      const removed = result.removed?.length || 0;
      let message = pulled > 0 ? `已从后端服拉取 ${pulled} 个资源包` : '本地资源包已是最新';
      if (skipped > 0) message += `，跳过 ${skipped} 个`;
      if (removed) message += `，删除 ${removed} 个重复包`;
      setPacksNotice(`${message}。`);
      if (result.warnings?.length) {
        setPacksError(result.warnings.join('；'));
      }
    } catch (failure) {
      setPacksError(failure.message || '从后端服拉取资源包失败');
    } finally {
      setPacksRefreshing(false);
    }
  };

  useEffect(() => {
    if (active && transfer) {
      loadPacks();
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [active, transfer]);

  if (!form) {
    return (
      <Box sx={{ py: 8, display: 'grid', placeItems: 'center' }}>
        <CircularProgress />
      </Box>
    );
  }

  const grid = { display: 'grid', gap: 2, gridTemplateColumns: { xs: '1fr', md: '1fr 1fr' } };
  const [limboHost, limboPort] = splitLimbo(form.limbo_upstream);
  const updateLimbo = (host, port) => update('limbo_upstream', `${host}:${port}`);

  return (
    <Card component="form" onSubmit={save}>
      <CardContent sx={{ p: { xs: 2.4, sm: 3.2 } }}>
        <Stack spacing={2.4}>
          <Box>
            <Typography variant="h6">转服与资源包</Typography>
            <Typography variant="body2" color="text.secondary">
              连接模式、limbo 监听端口、转服表单和资源包拉取。limbo 只在房间运行期间启动，
              停止房间后自动退出。
            </Typography>
          </Box>
          {running && <Alert severity="info">房间运行中，转服设置暂时不可修改。</Alert>}
          {error && <Alert severity="error">{error}</Alert>}

          <Box sx={grid}>
            <TextField
              select
              label="连接模式"
              value={form.connection_mode || 'proxy'}
              onChange={event => update('connection_mode', event.target.value)}
              disabled={running}
              fullWidth
              helperText={
                transfer
                  ? '玩家先进入 limbo，点击表单按钮后直连目标服务器。'
                  : '玩家流量经 CubeDock 中继到上游服务器；limbo 不会启动。'
              }
            >
              <MenuItem value="proxy">代理模式（中继）</MenuItem>
              <MenuItem value="transfer">转服模式（limbo + Transfer）</MenuItem>
            </TextField>
            <TextField
              label="Limbo 监听主机"
              value={limboHost}
              onChange={event => updateLimbo(event.target.value, limboPort)}
              disabled={running || !transfer}
              fullWidth
              helperText="通常保持 127.0.0.1"
            />
            <TextField
              label="Limbo 监听端口"
              type="number"
              value={limboPort}
              onChange={event => updateLimbo(limboHost, event.target.value)}
              disabled={running || !transfer}
              fullWidth
              inputProps={{ min: 1, max: 65535 }}
              helperText="默认 19133；limbo 只在房间运行期间监听"
            />
            <TextField
              label="Limbo 资源包目录"
              value={form.limbo_packs_dir || ''}
              onChange={event => update('limbo_packs_dir', event.target.value)}
              disabled={running || !transfer}
              fullWidth
              sx={{ gridColumn: { md: '1 / -1' } }}
              helperText=".mcpack/.zip 或包文件夹；留空时使用 limbo 同级自动创建的 packs/"
            />
          </Box>

          <Box>
            <FormControlLabel
              control={<Switch checked={!!form.limbo_auto_pull_packs} onChange={event => update('limbo_auto_pull_packs', event.target.checked)} disabled={running || !transfer} />}
              label="开房时自动从后端服拉取资源包"
            />
            <Typography variant="caption" color="text.secondary" display="block">
              开始房间前用假网易机器人登录「本地 Bedrock 地址」下载未加密资源包；
              下载完成后立即断开，失败不会阻止开房。
            </Typography>
          </Box>

          {transfer ? (
            <>
              <Divider />
              <Box>
                <Typography variant="subtitle2" color="text.secondary">
                  转服表单（SimpleForm）
                </Typography>
                <Typography variant="caption" color="text.secondary">
                  玩家进入 limbo 后看到的标题、正文和按钮文字；修改后需重新启动房间。
                </Typography>
              </Box>
              <Box sx={grid}>
                <TextField
                  label="表单标题"
                  value={form.limbo_form_title || ''}
                  onChange={event => update('limbo_form_title', event.target.value)}
                  disabled={running}
                  fullWidth
                  helperText="默认 CubeDock Limbo"
                />
                <TextField
                  label="按钮文字"
                  value={form.limbo_form_button || ''}
                  onChange={event => update('limbo_form_button', event.target.value)}
                  disabled={running}
                  fullWidth
                  helperText="默认 进入服务器；表单固定包含「断开连接」按钮"
                />
                <TextField
                  label="表单内容"
                  value={form.limbo_form_content || ''}
                  onChange={event => update('limbo_form_content', event.target.value)}
                  disabled={running}
                  fullWidth
                  multiline
                  minRows={3}
                  sx={{ gridColumn: { md: '1 / -1' } }}
                  helperText="支持换行；默认 点击下方按钮进入服务器。"
                />
              </Box>
              <Typography variant="caption" color="text.secondary">
                玩家误关表单时 limbo 会在 1 秒后自动重新打开；选择「断开连接」会发送正常的
                Bedrock 断开包并退出 limbo。
              </Typography>

              <Divider />
              <Box>
                <Stack
                  direction={{ xs: 'column', sm: 'row' }}
                  spacing={1}
                  alignItems={{ xs: 'stretch', sm: 'flex-start' }}
                  justifyContent="space-between"
                >
                  <Box>
                    <Typography variant="subtitle2" color="text.secondary">
                      Limbo 资源包列表
                    </Typography>
                    <Typography variant="caption" color="text.secondary" sx={{ wordBreak: 'break-all' }}>
                      目录：{packs?.dir || '尚未读取'}
                    </Typography>
                  </Box>
                  <Stack direction="row" spacing={1} flexWrap="wrap" useFlexGap>
                    <Button
                      type="button"
                      size="small"
                      variant="contained"
                      startIcon={<CloudDownload />}
                      onClick={refreshFromBackend}
                      disabled={packsRefreshing || packsBusy}
                    >
                      {packsRefreshing ? '拉取中…' : '从后端服拉取'}
                    </Button>
                    <Button
                      type="button"
                      size="small"
                      variant="outlined"
                      startIcon={<Refresh />}
                      onClick={loadPacks}
                      disabled={packsBusy || packsRefreshing}
                    >
                      {packsBusy ? '刷新中…' : '刷新列表'}
                    </Button>
                  </Stack>
                </Stack>
                {packsError && <Alert severity="error" sx={{ mt: 1 }}>{packsError}</Alert>}
                {packsNotice && <Alert severity="success" sx={{ mt: 1 }}>{packsNotice}</Alert>}
                {packs && packs.packs?.length === 0 && (
                  <Typography variant="body2" color="text.secondary" sx={{ mt: 1 }}>
                    目录中没有资源包。
                  </Typography>
                )}
                {packs && packs.packs?.length > 0 && (
                  <Stack spacing={1} sx={{ mt: 1.2 }}>
                    {packs.packs.map(pack => (
                      <Box
                        key={pack.file}
                        sx={{ border: '1px solid', borderColor: 'divider', borderRadius: 1, p: 1 }}
                      >
                        <Typography variant="body2">{pack.name}</Typography>
                        <Typography variant="caption" color="text.secondary">
                          {pack.file} · {pack.uuid || '无 UUID'} · v{pack.version || '—'} · {formatBytes(pack.size)}
                        </Typography>
                        {pack.error && (
                          <Typography variant="caption" color="error" display="block">
                            {pack.error}
                          </Typography>
                        )}
                      </Box>
                    ))}
                  </Stack>
                )}
                <Typography variant="caption" color="text.secondary" display="block" sx={{ mt: 1 }}>
                  拉取会以假网易机器人登录「本地 Bedrock 地址」，只下载未加密包；本地已有的
                  UUID+版本会跳过，完全相同的重复包会自动删除。资源包在 limbo 启动时加载，
                  拉取/刷新后请重新启动房间生效。
                </Typography>
              </Box>
            </>
          ) : (
            <Alert severity="info">
              当前为代理模式，limbo 不会启动。切换到转服模式后才会在开房时启动 limbo。
            </Alert>
          )}

          <Stack direction="row" justifyContent="flex-end">
            <Button type="submit" variant="contained" startIcon={<Save />} disabled={running || busy}>
              {busy ? '保存中…' : '保存转服设置'}
            </Button>
          </Stack>
        </Stack>
      </CardContent>
    </Card>
  );
}

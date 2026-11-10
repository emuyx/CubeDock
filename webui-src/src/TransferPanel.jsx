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
  Tab,
  Tabs,
  TextField,
  Typography,
} from '@mui/material';
import { CloudDownload, Refresh, Save } from '@mui/icons-material';
import { api } from './api.js';
import { useConfigForm } from './configForm.jsx';
import HelpTip, { FieldHint } from './HelpTip.jsx';

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
  const [tab, setTab] = useState(0);
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

  const refreshPacks = async () => {
    setPacksRefreshing(true);
    setPacksError('');
    setPacksNotice('');
    try {
      const result = await api('packs/refresh', {});
      setPacks(result);
      const pulled = result.pulled || 0;
      const skipped = result.skipped || 0;
      const removed = result.removed?.length || 0;
      let message = pulled > 0 ? `已从服务器拉取 ${pulled} 个资源包` : '本地资源包已是最新';
      if (skipped > 0) message += `，跳过 ${skipped} 个`;
      if (removed) message += `，删除 ${removed} 个重复包`;
      setPacksNotice(`${message}。`);
      if (result.warnings?.length) {
        setPacksError(result.warnings.join('；'));
      }
    } catch (failure) {
      setPacksError(failure.message || '从服务器拉取资源包失败');
    } finally {
      setPacksRefreshing(false);
    }
  };

  useEffect(() => {
    if (active && transfer && tab === 1) {
      loadPacks();
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [active, transfer, tab]);

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
            <Typography variant="h6">连接方式与资源包</Typography>
            <Typography variant="body2" color="text.secondary">
              选择玩家如何进入服务器，以及中转房间和资源包设置。两种方式都要求服务器支持网易协议。
            </Typography>
          </Box>
          {running && <Alert severity="info">房间运行中，连接设置暂时不能修改。</Alert>}
          {error && <Alert severity="error">{error}</Alert>}

          <Tabs
            value={tab}
            onChange={(_, value) => setTab(value)}
            variant="scrollable"
            allowScrollButtonsMobile
          >
            <Tab label="连接方式" />
            <Tab label="资源包" />
            <Tab label="提示页面" />
          </Tabs>

          {tab === 0 && (
            <Stack spacing={2.4}>
              <TextField
                select
                label="连接方式"
                value={form.connection_mode || 'proxy'}
                onChange={event => update('connection_mode', event.target.value)}
                disabled={running}
                fullWidth
                helperText={
                  <FieldHint help="两种方式都要求服务器支持网易协议。CubeDock 不会把网易协议转换成标准 Bedrock。">
                    {transfer
                      ? '玩家先进入本地中转房间，点击按钮后再进入你的服务器。'
                      : '玩家流量经 CubeDock 转发到服务器，不需要中转房间。'}
                  </FieldHint>
                }
              >
                <MenuItem value="proxy">代理模式（推荐）</MenuItem>
                <MenuItem value="transfer">转服模式（先进中转房间）</MenuItem>
              </TextField>
              {transfer ? (
                <Box sx={grid}>
                  <TextField
                    label="中转房间监听地址"
                    value={limboHost}
                    onChange={event => updateLimbo(event.target.value, limboPort)}
                    disabled={running}
                    fullWidth
                    helperText={<FieldHint help="玩家会先进入这个本地中转房间；一般保持 127.0.0.1。">一般保持 127.0.0.1</FieldHint>}
                  />
                  <TextField
                    label="中转房间端口"
                    type="number"
                    value={limboPort}
                    onChange={event => updateLimbo(limboHost, event.target.value)}
                    disabled={running}
                    fullWidth
                    inputProps={{ min: 1, max: 65535 }}
                    helperText={<FieldHint help="只在房间运行期间监听。">默认 19133</FieldHint>}
                  />
                </Box>
              ) : (
                <Alert severity="info">
                  代理模式不需要中转房间。玩家会直接经 CubeDock 进入服务器。
                </Alert>
              )}
            </Stack>
          )}

          {tab === 1 && (
            transfer ? (
              <Stack spacing={2.4}>
                <TextField
                  label="资源包目录"
                  value={form.limbo_packs_dir || ''}
                  onChange={event => update('limbo_packs_dir', event.target.value)}
                  disabled={running}
                  fullWidth
                  helperText={
                    <FieldHint help="放入 .mcpack/.zip 或包文件夹。留空时使用程序目录下自动创建的 packs/。">
                      留空时自动创建
                    </FieldHint>
                  }
                />
                <Box>
                  <Stack direction="row" spacing={0.2} alignItems="center">
                    <FormControlLabel
                      control={<Switch checked={!!form.limbo_auto_pull_packs} onChange={event => update('limbo_auto_pull_packs', event.target.checked)} disabled={running} />}
                      label="开房时自动从服务器拉取资源包"
                    />
                    <HelpTip title="关闭后只使用本地资源包；可以随时在下方手动拉取。自动拉取失败不会阻止开房。" />
                  </Stack>
                  <Typography variant="caption" color="text.secondary" display="block">
                    开房前会登录服务器下载未加密资源包，下载完立即断开；失败不会阻止开房。
                  </Typography>
                </Box>

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
                        本地资源包
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
                        onClick={refreshPacks}
                        disabled={packsRefreshing || packsBusy}
                      >
                        {packsRefreshing ? '拉取中…' : '从服务器拉取'}
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
                    拉取时会用一个临时网易账号登录服务器，只下载未加密资源包；本地已有的版本会跳过，
                    完全相同的重复包会自动删除。资源包在房间启动时加载，拉取后请重新启动房间。
                  </Typography>
                </Box>
              </Stack>
            ) : (
              <Alert severity="info">代理模式不使用本地资源包。切换到转服模式后再配置。</Alert>
            )
          )}

          {tab === 2 && (
            transfer ? (
              <Stack spacing={2.4}>
                <Box>
                  <Typography variant="subtitle2" color="text.secondary">
                    中转房间提示页面
                  </Typography>
                  <Typography variant="caption" color="text.secondary">
                    玩家进入中转房间后看到的标题、正文和按钮文字；修改后需要重新启动房间。
                  </Typography>
                </Box>
                <Box sx={grid}>
                  <TextField
                    label="表单标题"
                    value={form.limbo_form_title || ''}
                    onChange={event => update('limbo_form_title', event.target.value)}
                    disabled={running}
                    fullWidth
                    helperText="默认 CubeDock 中转房间"
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
                  玩家误关提示页面时会在 1 秒后自动重新打开；点击「断开连接」会正常退出中转房间。
                </Typography>
              </Stack>
            ) : (
              <Alert severity="info">提示页面只在中转房间使用。代理模式不需要配置。</Alert>
            )
          )}

          <Stack direction="row" justifyContent="flex-end">
            <Button type="submit" variant="contained" startIcon={<Save />} disabled={running || busy}>
              {busy ? '保存中…' : '保存连接设置'}
            </Button>
          </Stack>
        </Stack>
      </CardContent>
    </Card>
  );
}

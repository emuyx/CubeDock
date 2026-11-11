import React, { useEffect, useState } from 'react';
import {
  Alert,
  Box,
  Button,
  Card,
  CardContent,
  Chip,
  CircularProgress,
  Divider,
  FormControlLabel,
  IconButton,
  List,
  ListItemButton,
  ListItemText,
  MenuItem,
  Stack,
  Switch,
  Tab,
  Tabs,
  TextField,
  Tooltip,
  Typography,
} from '@mui/material';
import {
  Add,
  CloudDownload,
  ContentCopy,
  DeleteOutlined,
  Refresh,
  Save,
  Tune,
} from '@mui/icons-material';
import { getPacks, refreshPacks } from './api.js';
import { useConfigForm } from './configForm.jsx';
import { useConfirm, useToast } from './App.jsx';
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
  if (index < 0) return ['127.0.0.1', '19133'];
  return [text.slice(0, index) || '127.0.0.1', text.slice(index + 1) || '19133'];
}

const grid = { display: 'grid', gap: 2, gridTemplateColumns: { xs: '1fr', md: '1fr 1fr' } };
const ROOM_TAGS = ['生存', '创造', '冒险', '模拟', '动作', '建筑', '竞技', '休闲'];

export default function ProfilesPanel({ status, active }) {
  const notify = useToast();
  const confirm = useConfirm();
  const {
    form,
    updateProfile,
    addProfile,
    duplicateProfile,
    removeProfile,
    save,
    busy,
    error,
    dirty,
    discard,
  } = useConfigForm();
  const profiles = form?.profiles || [];
  const accounts = status.accounts || [];
  const running = !!status.running;
  const [selectedId, setSelectedId] = useState(null);
  const [tab, setTab] = useState(0);
  const [packs, setPacks] = useState(null);
  const [packsBusy, setPacksBusy] = useState(false);
  const [packsRefreshing, setPacksRefreshing] = useState(false);
  const [packsError, setPacksError] = useState('');
  const [packsNotice, setPacksNotice] = useState('');

  useEffect(() => {
    if (!profiles.length) return;
    if (!profiles.some(profile => profile.id === selectedId)) {
      setSelectedId(profiles[0].id);
    }
  }, [profiles, selectedId]);

  const profile = profiles.find(item => item.id === selectedId) || profiles[0] || null;
  const profileId = profile?.id || '';
  const transfer = profile?.connection_mode === 'transfer';
  const fallbackProfile = status.config?.profiles?.[0]?.id || 'default';
  const assignedCount = id =>
    accounts.filter(account => (account.profile_id || fallbackProfile) === id).length;
  const selectedAccounts = profile ? assignedCount(profile.id) : 0;

  useEffect(() => {
    setPacks(null);
    setPacksError('');
    setPacksNotice('');
  }, [profileId]);

  const loadPacks = async () => {
    if (!profile || dirty) return;
    setPacksBusy(true);
    setPacksError('');
    try {
      setPacks(await getPacks(profile.id));
    } catch (failure) {
      setPacksError(failure.message || '读取资源包列表失败');
    } finally {
      setPacksBusy(false);
    }
  };

  const pullPacks = async () => {
    if (!profile || dirty) return;
    setPacksRefreshing(true);
    setPacksError('');
    setPacksNotice('');
    try {
      const result = await refreshPacks(profile.id);
      setPacks(result);
      const pulled = result.pulled || 0;
      const skipped = result.skipped || 0;
      const removed = result.removed?.length || 0;
      let message = pulled > 0 ? `已从服务器拉取 ${pulled} 个资源包` : '本地资源包已是最新';
      if (skipped > 0) message += `，跳过 ${skipped} 个`;
      if (removed) message += `，删除 ${removed} 个重复包`;
      setPacksNotice(`${message}。`);
      if (result.warnings?.length) setPacksError(result.warnings.join('；'));
    } catch (failure) {
      setPacksError(failure.message || '从服务器拉取资源包失败');
    } finally {
      setPacksRefreshing(false);
    }
  };

  useEffect(() => {
    if (active && tab === 2 && profile && transfer && !dirty) {
      loadPacks();
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [active, tab, profileId, transfer, dirty]);

  const add = () => {
    const next = addProfile();
    if (next) setSelectedId(next.id);
    setTab(0);
  };

  const duplicate = () => {
    if (!profile) return;
    const next = duplicateProfile(profile.id);
    if (next) setSelectedId(next.id);
  };

  const remove = async () => {
    if (!profile) return;
    const confirmed = await confirm({
      title: '删除配置方案',
      message: `删除「${profile.name}」？仍有 ${selectedAccounts} 个账号使用它时不能删除。`,
      confirmLabel: '删除方案',
      severity: 'error',
    });
    if (confirmed) removeProfile(profile.id);
  };

  if (!form) {
    return (
      <Box sx={{ py: 8, display: 'grid', placeItems: 'center' }}>
        <CircularProgress />
      </Box>
    );
  }

  const [limboHost, limboPort] = splitLimbo(profile?.limbo_upstream);
  const updateLimbo = (host, port) =>
    profile && updateProfile(profile.id, 'limbo_upstream', `${host}:${port}`);
  const roomTags = profile?.room_tags || [];
  const toggleTag = tag => {
    if (!profile) return;
    if (roomTags.includes(tag)) {
      updateProfile(profile.id, 'room_tags', roomTags.filter(value => value !== tag));
      return;
    }
    if (roomTags.length >= 3) {
      notify('房间标签最多选择 3 个', 'warning');
      return;
    }
    updateProfile(profile.id, 'room_tags', [...roomTags, tag]);
  };

  return (
    <Stack spacing={2.2}>
      <Card>
        <CardContent sx={{ p: { xs: 2.2, sm: 2.8 } }}>
          <Stack
            direction={{ xs: 'column', sm: 'row' }}
            spacing={2}
            justifyContent="space-between"
            alignItems={{ xs: 'stretch', sm: 'center' }}
          >
            <Box>
              <Stack direction="row" spacing={1} alignItems="center">
                <Tune color="primary" />
                <Typography variant="h5">配置方案</Typography>
                <Chip size="small" variant="outlined" label={`${profiles.length} 个`} />
              </Stack>
              <Typography variant="body2" color="text.secondary" sx={{ mt: 0.5 }}>
                每个网易账号选择一个方案；同一方案可被多个账号共享。方案包含服务器、连接方式、房间设置、资源包和提示页面。
              </Typography>
            </Box>
            <Stack direction={{ xs: 'column', sm: 'row' }} spacing={1} flexWrap="wrap" useFlexGap>
              <Button
                variant="outlined"
                startIcon={<Add />}
                onClick={add}
                disabled={running || profiles.length >= 32}
                sx={{ whiteSpace: 'nowrap', width: { xs: '100%', sm: 'auto' } }}
              >
                添加方案
              </Button>
              <Button
                variant="outlined"
                startIcon={<ContentCopy />}
                onClick={duplicate}
                disabled={!profile || running}
                sx={{ whiteSpace: 'nowrap', width: { xs: '100%', sm: 'auto' } }}
              >
                复制方案
              </Button>
              <Button
                variant="outlined"
                color="error"
                startIcon={<DeleteOutlined />}
                onClick={remove}
                disabled={!profile || running || profiles.length <= 1 || selectedAccounts > 0}
                sx={{ whiteSpace: 'nowrap', width: { xs: '100%', sm: 'auto' } }}
              >
                删除方案
              </Button>
            </Stack>
          </Stack>
          {running && <Alert severity="info" sx={{ mt: 1.6 }}>有房间运行中，停止全部房间后才能修改配置方案。</Alert>}
          {error && <Alert severity="error" sx={{ mt: 1.6 }}>{error}</Alert>}
          {dirty && <Alert severity="info" variant="outlined" sx={{ mt: 1.6 }}>方案有未保存的修改。</Alert>}
        </CardContent>
      </Card>

      <Box sx={{ display: 'grid', gap: 2, gridTemplateColumns: { xs: '1fr', lg: '280px minmax(0, 1fr)' } }}>
        <Card sx={{ alignSelf: 'start' }}>
          <CardContent sx={{ p: 1.2, '&:last-child': { pb: 1.2 } }}>
            <Typography variant="overline" color="text.secondary" fontWeight={800} sx={{ px: 1 }}>
              方案列表
            </Typography>
            <List disablePadding sx={{ mt: 0.5 }}>
              {profiles.map(item => (
                <ListItemButton
                  key={item.id}
                  selected={item.id === profileId}
                  onClick={() => {
                    setSelectedId(item.id);
                    setTab(0);
                  }}
                  sx={{ borderRadius: 2, mb: 0.5 }}
                >
                  <ListItemText
                    primary={item.name}
                    secondary={`${item.connection_mode === 'transfer' ? '转服' : '代理'} · ${assignedCount(item.id)} 个账号`}
                    primaryTypographyProps={{ fontWeight: 750 }}
                  />
                </ListItemButton>
              ))}
            </List>
            {profiles.length === 0 && (
              <Typography variant="body2" color="text.secondary" sx={{ p: 1.5 }}>
                还没有配置方案，请添加一个。
              </Typography>
            )}
          </CardContent>
        </Card>

        {profile ? (
          <Card component="form" onSubmit={save}>
            <CardContent sx={{ p: { xs: 2.2, sm: 2.8 } }}>
              <Stack spacing={2.2}>
                <Stack direction="row" justifyContent="space-between" alignItems="center" spacing={1}>
                  <Box sx={{ minWidth: 0 }}>
                    <Typography variant="h6" noWrap>{profile.name}</Typography>
                    <Typography variant="caption" color="text.secondary">
                      {profile.connection_mode === 'transfer' ? '转服模式' : '代理模式'} · {profile.upstream || '未填写服务器'}
                      {selectedAccounts > 0 ? ` · ${selectedAccounts} 个账号使用` : ' · 暂无账号使用'}
                    </Typography>
                  </Box>
                  <Chip size="small" color={profile.connection_mode === 'transfer' ? 'secondary' : 'primary'} label={profile.connection_mode === 'transfer' ? '转服' : '代理'} />
                </Stack>

                <Tabs value={tab} onChange={(_, value) => setTab(value)} variant="scrollable" allowScrollButtonsMobile>
                  <Tab label="连接" />
                  <Tab label="房间" />
                  <Tab label="资源包" />
                  <Tab label="提示页面" />
                </Tabs>

                {tab === 0 && (
                  <Stack spacing={2.2}>
                    <Box sx={grid}>
                      <TextField
                        label="方案名称"
                        value={profile.name || ''}
                        onChange={event => updateProfile(profile.id, 'name', event.target.value)}
                        disabled={running}
                        required
                        fullWidth
                        inputProps={{ maxLength: 64 }}
                        helperText="用于在账号和仪表盘中识别"
                      />
                      <TextField
                        select
                        label="连接方式"
                        value={profile.connection_mode || 'proxy'}
                        onChange={event => updateProfile(profile.id, 'connection_mode', event.target.value)}
                        disabled={running}
                        fullWidth
                        helperText={<FieldHint help="两种方式都要求服务器支持网易协议。CubeDock 不会把网易协议转换成标准 Bedrock。">每个方案可以独立选择</FieldHint>}
                      >
                        <MenuItem value="proxy">代理模式（推荐）</MenuItem>
                        <MenuItem value="transfer">转服模式（先进中转房间）</MenuItem>
                      </TextField>
                    </Box>
                    <TextField
                      label="服务器地址"
                      value={profile.upstream || ''}
                      onChange={event => updateProfile(profile.id, 'upstream', event.target.value)}
                      disabled={running}
                      required
                      fullWidth
                      helperText={<FieldHint help="服务器必须支持网易协议（RakNet v8）。标准 Bedrock 服务器不能直接用。">IP:端口，例如 127.0.0.1:19132</FieldHint>}
                    />
                    {transfer && (
                      <Box sx={grid}>
                        <TextField
                          label="中转房间监听地址"
                          value={limboHost}
                          onChange={event => updateLimbo(event.target.value, limboPort)}
                          disabled={running}
                          fullWidth
                          helperText={<FieldHint help="每个转服方案使用独立端口，不能与其他方案重复。">一般保持 127.0.0.1</FieldHint>}
                        />
                        <TextField
                          label="中转房间端口"
                          type="number"
                          value={limboPort}
                          onChange={event => updateLimbo(limboHost, event.target.value)}
                          disabled={running}
                          fullWidth
                          inputProps={{ min: 1, max: 65535 }}
                          helperText="每个转服方案独立监听"
                        />
                      </Box>
                    )}
                    <TextField
                      label="房间协议版本"
                      type="number"
                      value={profile.lobby_protocol_id ?? 43}
                      onChange={event => updateProfile(profile.id, 'lobby_protocol_id', Number(event.target.value))}
                      disabled={running}
                      sx={{ maxWidth: 260 }}
                      inputProps={{ min: 1, max: 255 }}
                      helperText={<FieldHint help="默认 43。只有确认客户端或服务器版本变化时才需要改。">默认 43</FieldHint>}
                    />
                  </Stack>
                )}

                {tab === 1 && (
                  <Stack spacing={2.2}>
                    <Box sx={grid}>
                      <TextField
                        label="房间名称"
                        value={profile.room_name || ''}
                        onChange={event => updateProfile(profile.id, 'room_name', event.target.value)}
                        disabled={running}
                        required
                        fullWidth
                        inputProps={{ maxLength: 40 }}
                        helperText={<FieldHint help="玩家在网易联机列表里看到的房间名。">1–40 个字</FieldHint>}
                      />
                      <TextField
                        label="房间密码（可选）"
                        type="password"
                        value={profile.room_password || ''}
                        onChange={event => updateProfile(profile.id, 'room_password', event.target.value)}
                        disabled={running}
                        fullWidth
                        autoComplete="off"
                        helperText={<FieldHint help="留空表示无密码。密码最多 64 个字符。">留空表示无密码</FieldHint>}
                      />
                    </Box>
                    <Box sx={grid}>
                      <TextField
                        label="房间总人数（含 CubeDock）"
                        type="number"
                        value={profile.max_players ?? 10}
                        onChange={event => updateProfile(profile.id, 'max_players', Number(event.target.value))}
                        disabled={running}
                        fullWidth
                        inputProps={{ min: 2, max: 10 }}
                        helperText={<FieldHint help="网易房间最多 10 人，CubeDock 自己占 1 个名额，所以最多 9 位玩家。">2–10 人</FieldHint>}
                      />
                      <TextField
                        label="房间版本"
                        value={profile.room_game_version || ''}
                        onChange={event => updateProfile(profile.id, 'room_game_version', event.target.value)}
                        disabled={running}
                        required
                        fullWidth
                        helperText={<FieldHint help="一般保持 1.21.130.0。只有网易更新客户端、确认版本变化后才需要改。">当前为 1.21.130.0</FieldHint>}
                      />
                    </Box>
                    <Box sx={grid}>
                      <TextField
                        label="掉线重试间隔（秒）"
                        type="number"
                        value={profile.retry_seconds ?? 10}
                        onChange={event => updateProfile(profile.id, 'retry_seconds', Number(event.target.value))}
                        disabled={running}
                        fullWidth
                        inputProps={{ min: 5, max: 300 }}
                        helperText={<FieldHint help="房间掉线后等待多久重连，默认 10 秒。">5–300 秒</FieldHint>}
                      />
                      <TextField
                        label="准入等级"
                        type="number"
                        value={profile.min_level ?? 0}
                        onChange={event => updateProfile(profile.id, 'min_level', Number(event.target.value))}
                        disabled={running}
                        fullWidth
                        inputProps={{ min: 0, max: 1000 }}
                        helperText={<FieldHint help="进入房间所需的最低网易等级；0 表示不限制。">0 表示不限制</FieldHint>}
                      />
                    </Box>
                    <Box>
                      <Typography variant="subtitle2" color="text.secondary" sx={{ mb: 0.8 }}>
                        房间标签（最多 3 个）
                      </Typography>
                      <Stack direction="row" spacing={0.8} flexWrap="wrap" useFlexGap>
                        {ROOM_TAGS.map((label, tag) => {
                          const selected = roomTags.includes(tag);
                          return (
                            <Chip
                              key={label}
                              label={label}
                              clickable
                              color={selected ? 'primary' : 'default'}
                              variant={selected ? 'filled' : 'outlined'}
                              onClick={() => toggleTag(tag)}
                              disabled={running || (!selected && roomTags.length >= 3)}
                            />
                          );
                        })}
                      </Stack>
                    </Box>
                    <Box sx={{ display: 'grid', gap: 1, gridTemplateColumns: { xs: '1fr', sm: '1fr 1fr' } }}>
                      <Stack direction="row" spacing={0.2} alignItems="center">
                        <FormControlLabel
                          control={<Switch checked={!!profile.friends_only} onChange={event => updateProfile(profile.id, 'friends_only', event.target.checked)} disabled={running} />}
                          label="仅好友可见"
                        />
                        <HelpTip title="开启后只有你的网易好友能看到并进入房间。" />
                      </Stack>
                      <Stack direction="row" spacing={0.2} alignItems="center">
                        <FormControlLabel
                          control={<Switch checked={!!profile.auto_accept_friends} onChange={event => updateProfile(profile.id, 'auto_accept_friends', event.target.checked)} disabled={running} />}
                          label="自动同意好友申请"
                        />
                        <HelpTip title="开启「仅好友可见」时会自动跳过，避免陌生人通过好友申请进入房间。" />
                      </Stack>
                      <Stack direction="row" spacing={0.2} alignItems="center">
                        <FormControlLabel
                          control={<Switch checked={!!profile.allow_pvp} onChange={event => updateProfile(profile.id, 'allow_pvp', event.target.checked)} disabled={running} />}
                          label="允许 PvP"
                        />
                        <HelpTip title="只影响网易房间的 PvP 标记，实际规则仍由服务器决定。" />
                      </Stack>
                    </Box>
                  </Stack>
                )}

                {tab === 2 && (
                  transfer ? (
                    <Stack spacing={2.2}>
                      <TextField
                        label="资源包目录"
                        value={profile.limbo_packs_dir || ''}
                        onChange={event => updateProfile(profile.id, 'limbo_packs_dir', event.target.value)}
                        disabled={running}
                        fullWidth
                        helperText={<FieldHint help="留空时使用数据目录下 packs/<方案ID>/，避免不同服务器的资源包混在一起。">留空时按方案自动创建</FieldHint>}
                      />
                      <Stack direction="row" spacing={0.2} alignItems="center">
                        <FormControlLabel
                          control={<Switch checked={!!profile.limbo_auto_pull_packs} onChange={event => updateProfile(profile.id, 'limbo_auto_pull_packs', event.target.checked)} disabled={running} />}
                          label="开房时自动从服务器拉取资源包"
                        />
                        <HelpTip title="关闭后只使用本地资源包；可以随时在下方手动拉取。自动拉取失败不会阻止开房。" />
                      </Stack>
                      <Divider />
                      <Stack direction="row" justifyContent="space-between" alignItems="flex-start" spacing={1.5} flexWrap="wrap" useFlexGap>
                        <Box sx={{ minWidth: 0 }}>
                          <Typography variant="subtitle2" color="text.secondary">本地资源包</Typography>
                          <Typography variant="caption" color="text.secondary" sx={{ wordBreak: 'break-all' }}>
                            目录：{packs?.dir || (profile.limbo_packs_dir || '保存后自动生成')}
                          </Typography>
                        </Box>
                        <Stack direction="row" spacing={1} flexWrap="wrap" useFlexGap>
                          <Button
                            type="button"
                            size="small"
                            variant="contained"
                            startIcon={<CloudDownload />}
                            onClick={pullPacks}
                            disabled={dirty || packsRefreshing || packsBusy}
                          >
                            {packsRefreshing ? '拉取中…' : '从服务器拉取'}
                          </Button>
                          <Button
                            type="button"
                            size="small"
                            variant="outlined"
                            startIcon={<Refresh />}
                            onClick={loadPacks}
                            disabled={dirty || packsBusy || packsRefreshing}
                          >
                            {packsBusy ? '刷新中…' : '刷新列表'}
                          </Button>
                        </Stack>
                      </Stack>
                      {dirty && <Alert severity="info" variant="outlined">请先保存方案，再拉取或刷新资源包。</Alert>}
                      {packsError && <Alert severity="error">{packsError}</Alert>}
                      {packsNotice && <Alert severity="success">{packsNotice}</Alert>}
                      {packs && packs.packs?.length === 0 && (
                        <Typography variant="body2" color="text.secondary">目录中没有资源包。</Typography>
                      )}
                      {packs && packs.packs?.length > 0 && (
                        <Stack spacing={1}>
                          {packs.packs.map(pack => (
                            <Box key={pack.file} sx={{ border: '1px solid', borderColor: 'divider', borderRadius: 1, p: 1 }}>
                              <Typography variant="body2">{pack.name}</Typography>
                              <Typography variant="caption" color="text.secondary">
                                {pack.file} · {pack.uuid || '无 UUID'} · v{pack.version || '—'} · {formatBytes(pack.size)}
                              </Typography>
                              {pack.error && <Typography variant="caption" color="error" display="block">{pack.error}</Typography>}
                            </Box>
                          ))}
                        </Stack>
                      )}
                    </Stack>
                  ) : (
                    <Alert severity="info">代理模式不使用本地资源包。切换到转服模式后再配置。</Alert>
                  )
                )}

                {tab === 3 && (
                  transfer ? (
                    <Stack spacing={2.2}>
                      <Box sx={grid}>
                        <TextField
                          label="表单标题"
                          value={profile.limbo_form_title || ''}
                          onChange={event => updateProfile(profile.id, 'limbo_form_title', event.target.value)}
                          disabled={running}
                          fullWidth
                          helperText="默认 CubeDock Limbo"
                        />
                        <TextField
                          label="按钮文字"
                          value={profile.limbo_form_button || ''}
                          onChange={event => updateProfile(profile.id, 'limbo_form_button', event.target.value)}
                          disabled={running}
                          fullWidth
                          helperText="表单固定包含「断开连接」按钮"
                        />
                      </Box>
                      <TextField
                        label="表单内容"
                        value={profile.limbo_form_content || ''}
                        onChange={event => updateProfile(profile.id, 'limbo_form_content', event.target.value)}
                        disabled={running}
                        fullWidth
                        multiline
                        minRows={3}
                        helperText="支持换行；默认 点击下方按钮进入服务器。"
                      />
                      <Typography variant="caption" color="text.secondary">
                        玩家误关提示页面时会在 1 秒后自动重新打开；点击「断开连接」会正常退出中转房间。
                      </Typography>
                    </Stack>
                  ) : (
                    <Alert severity="info">提示页面只在中转房间使用。代理模式不需要配置。</Alert>
                  )
                )}

                <Stack direction="row" justifyContent="flex-end" spacing={1}>
                  {dirty && (
                    <Button type="button" color="inherit" onClick={discard} disabled={busy}>
                      取消修改
                    </Button>
                  )}
                  <Button type="submit" variant="contained" startIcon={<Save />} disabled={running || busy}>
                    {busy ? '保存中…' : '保存配置方案'}
                  </Button>
                </Stack>
              </Stack>
            </CardContent>
          </Card>
        ) : (
          <Card>
            <CardContent sx={{ py: 8, textAlign: 'center' }}>
              <Typography color="text.secondary">还没有配置方案，点击「添加方案」开始。</Typography>
            </CardContent>
          </Card>
        )}
      </Box>
    </Stack>
  );
}

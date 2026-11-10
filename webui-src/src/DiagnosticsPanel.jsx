import React, { useCallback, useEffect, useState } from 'react';
import {
  Alert,
  Box,
  Button,
  Card,
  CardContent,
  Chip,
  CircularProgress,
  Divider,
  Stack,
  Tab,
  Tabs,
  Typography,
} from '@mui/material';
import {
  CheckCircleOutlined,
  ContentCopy,
  Download,
  ErrorOutlineOutlined,
  MonitorHeart,
  Refresh,
  RemoveCircleOutlineOutlined,
  SettingsSuggest,
  WarningAmber,
} from '@mui/icons-material';
import { getDiagnostics } from './api.js';
import { copyText, downloadText, timestamp } from './util.js';
import { useToast } from './App.jsx';

const STATUS = {
  ok: { label: '通过', color: 'success', Icon: CheckCircleOutlined },
  warn: { label: '注意', color: 'warning', Icon: WarningAmber },
  fail: { label: '失败', color: 'error', Icon: ErrorOutlineOutlined },
  skip: { label: '跳过', color: 'default', Icon: RemoveCircleOutlineOutlined },
};

function statusMeta(status) {
  return STATUS[status] || STATUS.skip;
}

function runtimeRows(runtime) {
  if (!runtime) return [];
  return [
    ['版本', `${runtime.version || 'dev'} (${runtime.commit || 'unknown'})`],
    ['运行环境', `${runtime.os || '—'} / ${runtime.arch || '—'} · ${runtime.go || '—'}`],
    ['数据目录', runtime.data_dir || '—'],
    ['控制台端口', runtime.web_port || '—'],
  ];
}

function InfoRow({ label, value }) {
  return (
    <Stack direction="row" spacing={1} sx={{ minWidth: 0 }}>
      <Typography variant="caption" color="text.secondary" sx={{ flexShrink: 0 }}>
        {label}
      </Typography>
      <Typography variant="caption" fontWeight={700} sx={{ wordBreak: 'break-all' }}>
        {value}
      </Typography>
    </Stack>
  );
}

function CheckRow({ check }) {
  const meta = statusMeta(check.status);
  const StatusIcon = meta.Icon;
  return (
    <Box
      sx={{
        display: 'flex',
        gap: 1.2,
        alignItems: 'flex-start',
        p: 1.2,
        borderRadius: 2,
        bgcolor: 'action.hover',
      }}
    >
      <StatusIcon color={meta.color} fontSize="small" sx={{ mt: 0.2 }} />
      <Box sx={{ minWidth: 0, flexGrow: 1 }}>
        <Stack direction="row" spacing={1} alignItems="center" flexWrap="wrap" useFlexGap>
          <Typography fontWeight={750}>{check.label}</Typography>
          <Chip size="small" variant="outlined" color={meta.color} label={meta.label} />
        </Stack>
        {check.detail && (
          <Typography variant="body2" color="text.secondary" sx={{ mt: 0.4, wordBreak: 'break-all' }}>
            {check.detail}
          </Typography>
        )}
        {check.hint && (
          <Typography variant="caption" color="warning.main" display="block" sx={{ mt: 0.4 }}>
            {check.hint}
          </Typography>
        )}
      </Box>
    </Box>
  );
}

function groupChecks(checks = []) {
  const groups = [];
  const index = new Map();
  for (const check of checks) {
    const category = check.category || '其他';
    if (!index.has(category)) {
      index.set(category, { category, checks: [] });
      groups.push(index.get(category));
    }
    index.get(category).checks.push(check);
  }
  return groups;
}

export default function DiagnosticsPanel({ status, active, onOpenSetup }) {
  const notify = useToast();
  const [data, setData] = useState(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [tab, setTab] = useState('全部');

  const load = useCallback(async () => {
    setLoading(true);
    setError('');
    try {
      setData(await getDiagnostics());
    } catch (failure) {
      setError(failure.message || '运行诊断失败');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    if (active) load();
  }, [active, load]);

  const buildReport = () => {
    if (!data) return '';
    const lines = [
      'CubeDock 诊断报告',
      `生成时间: ${data.generated_at}`,
      ...runtimeRows(data.runtime).map(([label, value]) => `${label}: ${value}`),
      '',
    ];
    for (const group of groupChecks(data.checks)) {
      lines.push(`[${group.category}]`);
      for (const check of group.checks) {
        const meta = statusMeta(check.status);
        let line = `- [${meta.label}] ${check.label}`;
        if (check.detail) line += `：${check.detail}`;
        if (check.hint) line += `（${check.hint}）`;
        lines.push(line);
      }
      lines.push('');
    }
    const events = (status?.events || []).slice(-50);
    if (events.length) {
      lines.push('最近事件:');
      for (const event of events) {
        lines.push(`${event.time} [${event.source || '系统'}] ${event.message}`);
      }
    }
    return lines.join('\n');
  };

  const copyReport = async () => {
    try {
      await copyText(buildReport());
      notify('诊断报告已复制', 'success');
    } catch (failure) {
      notify(failure.message || '复制失败', 'error');
    }
  };

  const downloadReport = () => {
    downloadText(`cubedock-diagnostics-${timestamp()}.txt`, buildReport());
    notify('诊断报告已下载', 'success');
  };

  const summary = data?.summary || {};
  const overall = summary.fail ? STATUS.fail : summary.warn ? STATUS.warn : STATUS.ok;
  const groups = data ? groupChecks(data.checks) : [];
  const categories = groups.map(group => group.category);
  const visibleGroups = tab === '全部' ? groups : groups.filter(group => group.category === tab);

  return (
    <Stack spacing={2.4}>
      <Card>
        <CardContent sx={{ p: { xs: 2.4, sm: 3.2 } }}>
          <Stack
            direction={{ xs: 'column', sm: 'row' }}
            spacing={2}
            justifyContent="space-between"
            alignItems={{ xs: 'stretch', sm: 'center' }}
          >
            <Box>
              <Stack direction="row" spacing={1} alignItems="center">
                <MonitorHeart color="primary" />
                <Typography variant="h6">运行诊断</Typography>
                {data && (
                  <Chip
                    size="small"
                    color={overall.color}
                    label={overall === STATUS.ok ? '全部通过' : overall.label}
                  />
                )}
              </Stack>
              <Typography variant="body2" color="text.secondary" sx={{ mt: 0.6 }}>
                只读检查服务器、中转房间、资源包、账号与本地环境；不会登录网易账号，也不会返回登录凭证。
              </Typography>
            </Box>
            <Stack direction="row" spacing={1} flexWrap="wrap" useFlexGap>
              <Button
                variant="contained"
                startIcon={loading ? <CircularProgress size={16} color="inherit" /> : <Refresh />}
                onClick={load}
                disabled={loading}
              >
                {loading ? '检测中…' : '重新检测'}
              </Button>
              <Button variant="outlined" startIcon={<ContentCopy />} onClick={copyReport} disabled={!data}>
                复制报告
              </Button>
              <Button variant="outlined" startIcon={<Download />} onClick={downloadReport} disabled={!data}>
                下载报告
              </Button>
            </Stack>
          </Stack>

          {data && (
            <>
              <Divider sx={{ my: 2 }} />
              <Stack direction="row" spacing={1} flexWrap="wrap" useFlexGap>
                <Chip size="small" color="success" variant="outlined" label={`通过 ${summary.ok || 0}`} />
                <Chip size="small" color="warning" variant="outlined" label={`注意 ${summary.warn || 0}`} />
                <Chip size="small" color="error" variant="outlined" label={`失败 ${summary.fail || 0}`} />
                <Chip size="small" variant="outlined" label={`跳过 ${summary.skip || 0}`} />
              </Stack>
              <Stack spacing={0.4} sx={{ mt: 1.6 }}>
                {runtimeRows(data.runtime).map(([label, value]) => (
                  <InfoRow key={label} label={label} value={value} />
                ))}
              </Stack>
              {categories.length > 0 && (
                <Tabs
                  value={tab}
                  onChange={(_, value) => setTab(value)}
                  variant="scrollable"
                  allowScrollButtonsMobile
                  sx={{ mt: 2 }}
                >
                  <Tab label="全部" value="全部" />
                  {categories.map(category => (
                    <Tab key={category} label={category} value={category} />
                  ))}
                </Tabs>
              )}
            </>
          )}
        </CardContent>
      </Card>

      {error && (
        <Alert
          severity="error"
          action={
            <Button color="inherit" size="small" onClick={load}>
              重试
            </Button>
          }
        >
          {error}
        </Alert>
      )}

      {!data && loading && (
        <Box sx={{ py: 8, display: 'grid', placeItems: 'center' }}>
          <CircularProgress />
        </Box>
      )}

      {data &&
        visibleGroups.map(group => (
          <Card key={group.category}>
            <CardContent sx={{ p: { xs: 2, sm: 2.6 } }}>
              <Typography variant="subtitle1" fontWeight={800}>
                {group.category}
              </Typography>
              <Divider sx={{ my: 1.2 }} />
              <Stack spacing={1}>
                {group.checks.map(check => (
                  <CheckRow key={check.id} check={check} />
                ))}
              </Stack>
            </CardContent>
          </Card>
        ))}

      {onOpenSetup && (
        <Card variant="outlined">
          <CardContent sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', gap: 2 }}>
            <Typography variant="body2" color="text.secondary">
              新用户？可以重新打开首次设置向导，按步骤配置服务器、添加账号并预检。
            </Typography>
            <Button startIcon={<SettingsSuggest />} onClick={onOpenSetup}>
              重新打开首次设置
            </Button>
          </CardContent>
        </Card>
      )}
    </Stack>
  );
}

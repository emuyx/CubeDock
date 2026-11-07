import React, { useEffect, useMemo, useRef, useState } from 'react';
import {
  Box,
  Button,
  Chip,
  CircularProgress,
  FormControlLabel,
  IconButton,
  InputAdornment,
  Stack,
  Switch,
  TextField,
  Typography,
} from '@mui/material';
import { Download, Search, Send } from '@mui/icons-material';
import { useTheme } from '@mui/material/styles';
import { downloadText, eventLevel, timestamp } from './util.js';
import { api, runConsole } from './api.js';
import { useToast } from './App.jsx';

const maxTranscriptLines = 200;
const maxHistoryLines = 100;
const controlLabelSx = {
  mr: 0,
  '& .MuiFormControlLabel-label': { fontSize: { xs: 13, sm: 14 } },
};

function clockNow() {
  return new Date().toLocaleTimeString('zh-CN', { hour12: false });
}

export default function LogsPanel({ events = [], debugEnabled = false, onDebugChanged, active = true }) {
  const theme = useTheme();
  const notify = useToast();
  const [query, setQuery] = useState('');
  const [showDebug, setShowDebug] = useState(debugEnabled);
  const [debugBusy, setDebugBusy] = useState(false);
  const [autoScroll, setAutoScroll] = useState(true);
  const [transcript, setTranscript] = useState([]);
  const [command, setCommand] = useState('');
  const [history, setHistory] = useState([]);
  const [busy, setBusy] = useState(false);
  const terminal = useRef(null);
  const input = useRef(null);
  const historyPos = useRef(-1);
  const draft = useRef('');
  const lineSeq = useRef(0);

  const merged = useMemo(() => {
    const server = events.map((event, index) => ({
      ...event,
      key: `server-${event.time}-${index}`,
      time: event.time || '',
    }));
    const local = transcript.map(line => ({ ...line, key: line.id }));
    return [...server, ...local].sort((left, right) => left.time.localeCompare(right.time));
  }, [events, transcript]);

  const filtered = useMemo(() => {
    const needle = query.trim().toLowerCase();
    return merged.filter(event => {
      const isDebug = event.level === 'debug' || (event.message || '').startsWith('[DEBUG]');
      if (!showDebug && isDebug) return false;
      if (!needle) return true;
      return (
        (event.message || '').toLowerCase().includes(needle) ||
        (event.source || '').toLowerCase().includes(needle) ||
        event.time.includes(needle)
      );
    });
  }, [merged, query, showDebug]);

  const lastVisibleKey = filtered.length ? filtered[filtered.length - 1].key : '';

  useEffect(() => {
    if (autoScroll && terminal.current) {
      terminal.current.scrollTop = terminal.current.scrollHeight;
    }
  }, [lastVisibleKey, autoScroll]);

  useEffect(() => {
    if (debugEnabled) setShowDebug(true);
  }, [debugEnabled]);

  useEffect(() => {
    if (active) input.current?.focus();
  }, [active]);

  const appendLines = lines => {
    setTranscript(current => {
      const next = [...current, ...lines];
      return next.length > maxTranscriptLines ? next.slice(next.length - maxTranscriptLines) : next;
    });
  };

  const localLine = (message, level = 'info') => {
    lineSeq.current += 1;
    return {
      id: `console-${lineSeq.current}`,
      time: clockNow(),
      source: '控制台',
      level,
      message,
    };
  };

  const submit = async () => {
    const line = command.trim();
    if (!line || busy) return;
    appendLines([localLine(`❯ ${line}`)]);
    setHistory(current => {
      const next = [...current, line];
      return next.length > maxHistoryLines ? next.slice(next.length - maxHistoryLines) : next;
    });
    historyPos.current = -1;
    draft.current = '';
    setCommand('');
    setBusy(true);
    try {
      const result = await runConsole(line);
      const output = result?.output?.length ? result.output : ['（无输出）'];
      appendLines(output.map(text => localLine(text)));
    } catch (failure) {
      appendLines([localLine(`错误：${failure.message || '命令执行失败'}`, 'error')]);
    } finally {
      setBusy(false);
      input.current?.focus();
    }
  };

  const keyDown = event => {
    if (event.key === 'Enter') {
      if (event.nativeEvent?.isComposing) return;
      event.preventDefault();
      submit();
      return;
    }
    if (event.key === 'ArrowUp') {
      if (!history.length) return;
      event.preventDefault();
      if (historyPos.current === -1) {
        draft.current = command;
        historyPos.current = history.length - 1;
      } else {
        historyPos.current = Math.max(0, historyPos.current - 1);
      }
      setCommand(history[historyPos.current]);
      return;
    }
    if (event.key === 'ArrowDown') {
      if (historyPos.current === -1) return;
      event.preventDefault();
      if (historyPos.current < history.length - 1) {
        historyPos.current += 1;
        setCommand(history[historyPos.current]);
      } else {
        historyPos.current = -1;
        setCommand(draft.current);
      }
    }
  };

  const changeCommand = event => {
    setCommand(event.target.value);
    if (historyPos.current !== -1) {
      historyPos.current = -1;
      draft.current = event.target.value;
    }
  };

  const toggleDebug = async enabled => {
    setDebugBusy(true);
    try {
      await api('debug', { enabled });
      notify(enabled ? 'Debug 详细日志已开启' : 'Debug 详细日志已关闭', 'success');
      onDebugChanged?.();
    } catch (failure) {
      notify(failure.message || '切换 Debug 失败', 'error');
    } finally {
      setDebugBusy(false);
    }
  };

  const download = () => {
    if (!merged.length) {
      notify('当前没有日志可导出', 'warning');
      return;
    }
    const text = merged
      .map(event => `${event.time} [${event.source || '系统'}] ${event.message}`)
      .join('\n');
    downloadText(`cubedock-logs-${timestamp()}.txt`, text);
    notify('日志已导出', 'success');
  };

  const color = level => {
    if (level === 'error') return '#fb7185';
    if (level === 'debug') return '#9db5a8';
    if (level === 'warning') return '#fbbf24';
    return '#e6efe9';
  };

  const sourceColor = source => {
    if (source === '控制台') return '#34d399';
    if (source?.startsWith('房间')) return '#fbbf24';
    if (source === '网易登录') return '#34d399';
    if (source === '账号') return '#60a5fa';
    if (source === '游戏') return '#fbbf24';
    if (source === '管理员') return '#fb7185';
    return '#fcd34d';
  };

  return (
    <Stack spacing={2}>
      <Stack spacing={1.2}>
        <TextField
          size="small"
          placeholder="搜索日志"
          value={query}
          onChange={event => setQuery(event.target.value)}
          fullWidth
          slotProps={{
            input: {
              startAdornment: (
                <InputAdornment position="start">
                  <Search fontSize="small" />
                </InputAdornment>
              ),
            },
          }}
        />
        <Box
          sx={{
            display: 'grid',
            gap: { xs: 0.8, sm: 1.5 },
            gridTemplateColumns: { xs: '1fr 1fr', sm: 'repeat(4, auto)' },
            alignItems: 'center',
            justifyItems: 'start',
          }}
        >
          <FormControlLabel
            sx={controlLabelSx}
            control={<Switch checked={debugEnabled} disabled={debugBusy} onChange={event => toggleDebug(event.target.checked)} />}
            label="Debug 详细日志"
          />
          <FormControlLabel
            sx={controlLabelSx}
            control={<Switch checked={showDebug} onChange={event => setShowDebug(event.target.checked)} />}
            label="显示 Debug 行"
          />
          <FormControlLabel
            sx={controlLabelSx}
            control={<Switch checked={autoScroll} onChange={event => setAutoScroll(event.target.checked)} />}
            label="自动滚动"
          />
          <Button variant="outlined" startIcon={<Download />} onClick={download}>
            导出日志
          </Button>
        </Box>
      </Stack>

      <Stack direction="row" spacing={1} alignItems="center" flexWrap="wrap" useFlexGap>
        <Chip size="small" label={`最近 ${merged.length} 条`} />
        <Chip size="small" variant="outlined" label={`当前显示 ${filtered.length} 条`} />
        <Typography variant="caption" color="text.secondary">
          日志文件：logs/cubedock.log
        </Typography>
      </Stack>
      <Typography variant="caption" color="text.secondary">
        “Debug 详细日志”立即生效；“显示 Debug 行”只控制当前视图。玩家加入/离开默认显示。
      </Typography>

      <Box
        sx={{
          bgcolor: theme.palette.mode === 'dark' ? '#060c09' : '#0d1b16',
          color: '#e6efe9',
          borderRadius: 3,
          border: '1px solid',
          borderColor: 'divider',
          overflow: 'hidden',
        }}
      >
        <Box
          ref={terminal}
          sx={{
            p: 2,
            minHeight: 360,
            maxHeight: { xs: '55vh', md: 540 },
            overflow: 'auto',
            fontFamily: 'ui-monospace, SFMono-Regular, Menlo, Consolas, monospace',
            fontSize: 12.5,
            lineHeight: 1.75,
            whiteSpace: 'pre-wrap',
            wordBreak: 'break-word',
          }}
        >
          {filtered.length === 0 ? (
            <Box sx={{ py: 8, textAlign: 'center', color: '#9db5a8' }}>暂无匹配日志</Box>
          ) : (
            filtered.map((event, index) => {
              const level = event.level === 'error' ? 'error' : eventLevel(event.message);
              const source = event.source || '系统';
              const echo = source === '控制台' && (event.message || '').startsWith('❯');
              return (
                <Box
                  key={event.key || `${event.time}-${index}`}
                  sx={{ display: 'flex', gap: 1.1, alignItems: 'flex-start' }}
                >
                  <Box component="span" sx={{ color: '#7b8f84', flex: '0 0 auto' }}>
                    {event.time}
                  </Box>
                  <Box
                    component="span"
                    sx={{
                      color: sourceColor(source),
                      flex: '0 0 auto',
                      minWidth: 64,
                      fontWeight: 700,
                    }}
                  >
                    [{source}]
                  </Box>
                  <Box component="span" sx={{ color: echo ? '#fcd34d' : color(level) }}>
                    {event.message}
                  </Box>
                </Box>
              );
            })
          )}
        </Box>
        <Box
          sx={{
            display: 'flex',
            alignItems: 'center',
            gap: 1,
            px: 1.4,
            py: 1,
            borderTop: '1px solid',
            borderColor: 'divider',
          }}
        >
          <Box component="span" sx={{ color: '#34d399', fontFamily: 'monospace', fontWeight: 800 }}>
            ❯
          </Box>
          <TextField
            size="small"
            fullWidth
            value={command}
            onChange={changeCommand}
            onKeyDown={keyDown}
            disabled={busy}
            placeholder="输入命令，help 查看帮助"
            autoComplete="off"
            variant="standard"
            slotProps={{
              htmlInput: { ref: input },
              input: {
                disableUnderline: true,
                sx: { fontFamily: 'ui-monospace, SFMono-Regular, Menlo, Consolas, monospace', fontSize: 13 },
              },
            }}
          />
          {busy && <CircularProgress size={16} />}
          <IconButton color="primary" onClick={submit} disabled={busy || !command.trim()} size="small">
            <Send fontSize="small" />
          </IconButton>
        </Box>
      </Box>
      <Typography variant="caption" color="text.secondary">
        输入 help 查看命令 · ↑/↓ 历史 · Enter 执行。命令输出仅保留在当前页面会话中。
      </Typography>
    </Stack>
  );
}

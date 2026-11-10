import React, { useState } from 'react';
import {
  Box,
  Button,
  Chip,
  IconButton,
  List,
  ListItemButton,
  ListItemIcon,
  ListItemText,
  Stack,
  Tooltip,
  Typography,
} from '@mui/material';
import {
  ContentCopy,
  Help,
  Home,
  MonitorHeart,
  PeopleAlt,
  RoomPreferences,
  Settings as SettingsIcon,
  SwapHoriz,
  Terminal,
} from '@mui/icons-material';
import { copyText, phaseColor, phaseLabel } from './util.js';
import { useToast } from './App.jsx';
import PlayersDialog from './PlayersDialog.jsx';

export const PAGES = [
  { key: 'home', label: '主页', icon: Home },
  { key: 'accounts', label: '账号与房间', icon: RoomPreferences },
  { key: 'transfer', label: '连接与资源包', icon: SwapHoriz },
  { key: 'console', label: '控制台', icon: Terminal },
  { key: 'diagnostics', label: '诊断', icon: MonitorHeart },
  { key: 'faq', label: '常见问题', icon: Help },
  { key: 'settings', label: '设置', icon: SettingsIcon },
];

export default function SidebarNav({ page, onSelect, status }) {
  const notify = useToast();
  const [playersRoom, setPlayersRoom] = useState(null);
  const accounts = status?.accounts || [];
  const rooms = status?.rooms || [];

  const copyRoom = async roomId => {
    try {
      await copyText(String(roomId));
      notify('房间号已复制', 'success');
    } catch (failure) {
      notify(failure.message || '复制失败', 'error');
    }
  };

  return (
    <Box sx={{ p: 1.5 }}>
      <Typography variant="overline" color="text.secondary" fontWeight={800} sx={{ px: 1.2 }}>
        控制台导航
      </Typography>
      <List disablePadding sx={{ mt: 0.6 }}>
        {PAGES.map(({ key, label, icon: Icon }) => (
          <ListItemButton
            key={key}
            selected={page === key}
            onClick={() => onSelect(key)}
            sx={{ borderRadius: 2, mb: 0.5 }}
          >
            <ListItemIcon sx={{ minWidth: 38 }}>
              <Icon fontSize="small" />
            </ListItemIcon>
            <ListItemText primary={label} primaryTypographyProps={{ fontWeight: 700 }} />
          </ListItemButton>
        ))}
      </List>

      <Stack
        direction="row"
        alignItems="center"
        justifyContent="space-between"
        sx={{ px: 1.2, mt: 2 }}
      >
        <Typography variant="overline" color="text.secondary" fontWeight={800}>
          我的房间
        </Typography>
        {accounts.length > 0 && (
          <Chip size="small" variant="outlined" label={accounts.length} sx={{ height: 20 }} />
        )}
      </Stack>

      <Stack
        spacing={0.7}
        sx={{ mt: 0.6, maxHeight: { xs: '42vh', md: '38vh' }, overflowY: 'auto', pr: 0.3 }}
      >
        {accounts.length === 0 && (
          <Box sx={{ p: 1.2, border: 1, borderColor: 'divider', borderRadius: 2 }}>
            <Typography variant="caption" color="text.secondary">
              还没有账号。
            </Typography>
            <Button size="small" sx={{ mt: 0.4, px: 0 }} onClick={() => onSelect('accounts')}>
              去添加
            </Button>
          </Box>
        )}
        {accounts.map(account => {
          const room = rooms[account.index];
          const phase = room?.phase || 'stopped';
          const name = account.name || account.label || account.email || `房间 ${account.index + 1}`;
          return (
            <Box
              key={account.index}
              onClick={() => setPlayersRoom(account.index)}
              sx={{
                p: 1,
                borderRadius: 2,
                border: 1,
                borderColor: 'divider',
                cursor: 'pointer',
                '&:hover': { bgcolor: 'action.hover' },
              }}
            >
              <Stack direction="row" justifyContent="space-between" alignItems="center" spacing={0.5}>
                <Typography variant="body2" fontWeight={700} noWrap sx={{ minWidth: 0 }}>
                  {name}
                </Typography>
                <Chip
                  size="small"
                  color={phaseColor(phase)}
                  label={phaseLabel(phase)}
                  sx={{ height: 20, flexShrink: 0 }}
                />
              </Stack>
              <Stack direction="row" justifyContent="space-between" alignItems="center" sx={{ mt: 0.2 }}>
                <Typography variant="caption" color="text.secondary" noWrap sx={{ minWidth: 0 }}>
                  {room?.room_id ? `${room.room_id} · ${room.players ?? 0} 人` : '未开房'}
                </Typography>
                <Stack direction="row" spacing={0} sx={{ flexShrink: 0 }}>
                  <Tooltip title="管理玩家">
                    <IconButton
                      size="small"
                      aria-label="管理玩家"
                      onClick={event => {
                        event.stopPropagation();
                        setPlayersRoom(account.index);
                      }}
                    >
                      <PeopleAlt sx={{ fontSize: 16 }} />
                    </IconButton>
                  </Tooltip>
                  <Tooltip title="复制房间号">
                    <span>
                      <IconButton
                        size="small"
                        aria-label="复制房间号"
                        disabled={!room?.room_id}
                        onClick={event => {
                          event.stopPropagation();
                          copyRoom(room.room_id);
                        }}
                      >
                        <ContentCopy sx={{ fontSize: 15 }} />
                      </IconButton>
                    </span>
                  </Tooltip>
                </Stack>
              </Stack>
            </Box>
          );
        })}
      </Stack>

      {accounts.length > 0 && (
        <Button fullWidth size="small" sx={{ mt: 1 }} onClick={() => onSelect('accounts')}>
          管理全部账号
        </Button>
      )}

      <Box sx={{ mt: 1.5, p: 1.2, borderRadius: 2, bgcolor: 'action.hover' }}>
        <Stack direction="row" justifyContent="space-between">
          <Typography variant="caption" color="text.secondary">
            账号
          </Typography>
          <Typography variant="caption" fontWeight={800}>
            {accounts.length}
          </Typography>
        </Stack>
        <Stack direction="row" justifyContent="space-between" sx={{ mt: 0.3 }}>
          <Typography variant="caption" color="text.secondary">
            在线玩家
          </Typography>
          <Typography variant="caption" fontWeight={800}>
            {status?.players ?? 0}
          </Typography>
        </Stack>
      </Box>

      <PlayersDialog
        open={playersRoom !== null}
        roomIndex={playersRoom}
        status={status}
        onClose={() => setPlayersRoom(null)}
      />
    </Box>
  );
}

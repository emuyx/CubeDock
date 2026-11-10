import React from 'react';
import {
  Box,
  List,
  ListItemButton,
  ListItemIcon,
  ListItemText,
  Stack,
  Typography,
} from '@mui/material';
import {
  Help,
  Home,
  MonitorHeart,
  RoomPreferences,
  Settings as SettingsIcon,
  SwapHoriz,
  Terminal,
} from '@mui/icons-material';

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
  const accounts = status?.accounts?.length ?? 0;
  const online = status?.players ?? 0;

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
      <Box sx={{ mt: 2, p: 1.3, borderRadius: 2, bgcolor: 'action.hover' }}>
        <Stack direction="row" justifyContent="space-between">
          <Typography variant="caption" color="text.secondary">
            账号
          </Typography>
          <Typography variant="caption" fontWeight={800}>
            {accounts}
          </Typography>
        </Stack>
        <Stack direction="row" justifyContent="space-between" sx={{ mt: 0.4 }}>
          <Typography variant="caption" color="text.secondary">
            在线玩家
          </Typography>
          <Typography variant="caption" fontWeight={800}>
            {online}
          </Typography>
        </Stack>
      </Box>
    </Box>
  );
}


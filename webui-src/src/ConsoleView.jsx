import React, { useCallback, useEffect, useState } from 'react';
import {
  Alert,
  AppBar,
  Box,
  Button,
  Card,
  CardContent,
  Chip,
  CircularProgress,
  Container,
  Divider,
  Drawer,
  IconButton,
  Menu,
  MenuItem,
  Stack,
  Toolbar,
  Tooltip,
  Typography,
} from '@mui/material';
import {
  AccountCircle,
  DarkMode,
  Key,
  LightMode,
  Logout,
  Menu as MenuIcon,
  Refresh,
  WarningAmber,
} from '@mui/icons-material';
import { getStatus, logout } from './api.js';
import { phaseColor, phaseLabel } from './util.js';
import AccountLogin from './AccountLogin.jsx';
import AccountsPanel from './AccountsPanel.jsx';
import ChangePasswordDialog from './ChangePasswordDialog.jsx';
import LogsPanel from './LogsPanel.jsx';
import Overview from './Overview.jsx';
import SettingsForm from './SettingsForm.jsx';
import SidebarNav from './SidebarNav.jsx';
import TransferPanel from './TransferPanel.jsx';
import { ConfigFormProvider } from './configForm.jsx';

function Brand() {
  return (
    <Stack direction="row" spacing={1.2} alignItems="center">
      <Box
        sx={{
          width: 36,
          height: 36,
          borderRadius: 2,
          display: 'grid',
          placeItems: 'center',
          background: 'linear-gradient(135deg,#8b5cf6,#2dd4bf)',
          color: '#fff',
          fontWeight: 900,
          fontSize: 18,
          boxShadow: '0 6px 18px rgba(139,92,246,.35)',
        }}
      >
        C
      </Box>
      <Box sx={{ display: { xs: 'none', sm: 'block' } }}>
        <Typography fontWeight={850} lineHeight={1.1}>
          Cube<span style={{ color: '#8b5cf6' }}>Dock</span>
        </Typography>
        <Typography variant="caption" color="text.secondary" letterSpacing=".12em">
          方块码头 · CONTROL PANEL
        </Typography>
      </Box>
    </Stack>
  );
}

export default function ConsoleView({ mode, toggleMode }) {
  const [status, setStatus] = useState(null);
  const [error, setError] = useState('');
  const [page, setPage] = useState('home');
  const [navOpen, setNavOpen] = useState(false);
  const [menuAnchor, setMenuAnchor] = useState(null);
  const [passwordOpen, setPasswordOpen] = useState(false);
  const [refreshing, setRefreshing] = useState(false);

  const load = useCallback(async () => {
    try {
      const next = await getStatus();
      setStatus(next);
      setError('');
    } catch (failure) {
      if (failure.message !== '登录已过期，请重新登录') setError(failure.message || '读取状态失败');
    }
  }, []);

  useEffect(() => {
    let active = true;
    let timer;
    const tick = async () => {
      await load();
      if (active) timer = setTimeout(tick, 2000);
    };
    tick();
    return () => {
      active = false;
      clearTimeout(timer);
    };
  }, [load]);

  const refresh = async () => {
    setRefreshing(true);
    await load();
    setRefreshing(false);
  };

  const logoutAction = async () => {
    setMenuAnchor(null);
    await logout();
    window.location.replace('/login');
  };

  if (!status) {
    return (
      <Box sx={{ minHeight: '100vh' }}>
        <AppBar position="sticky" color="transparent" elevation={0}>
          <Toolbar>
            <Brand />
            <Box sx={{ flexGrow: 1 }} />
            {error && <Chip color="error" label="状态读取失败" />}
          </Toolbar>
        </AppBar>
        <Container maxWidth="sm" sx={{ py: 10 }}>
          {error ? (
            <Card>
              <CardContent>
                <Stack spacing={2} alignItems="center">
                  <WarningAmber color="error" sx={{ fontSize: 44 }} />
                  <Typography variant="h6">无法连接控制台</Typography>
                  <Typography color="text.secondary" textAlign="center">
                    {error}
                  </Typography>
                  <Button variant="contained" onClick={refresh} disabled={refreshing}>
                    重试
                  </Button>
                </Stack>
              </CardContent>
            </Card>
          ) : (
            <Stack spacing={2} alignItems="center">
              <CircularProgress />
              <Typography color="text.secondary">正在读取控制台状态…</Typography>
            </Stack>
          )}
        </Container>
      </Box>
    );
  }

  const accounts = status.accounts || [];
  const isSetup = accounts.length === 0;

  return (
    <Box sx={{ minHeight: '100vh' }}>
      <AppBar position="sticky" color="transparent" elevation={0}>
        <Toolbar sx={{ gap: 1 }}>
          {!isSetup && (
            <IconButton
              onClick={() => setNavOpen(true)}
              aria-label="打开导航"
              sx={{ display: { xs: 'inline-flex', md: 'none' }, ml: -0.5 }}
            >
              <MenuIcon />
            </IconButton>
          )}
          <Brand />
          <Box sx={{ flexGrow: 1 }} />
          <Chip
            size="small"
            color={phaseColor(status.phase)}
            label={phaseLabel(status.phase)}
            sx={{ mr: { xs: 0, sm: 0.5 } }}
          />
          <Chip
            size="small"
            variant="outlined"
            color={status.upstream_online ? 'success' : 'warning'}
            label={status.upstream_online ? `上游在线${status.upstream_version ? ` · ${status.upstream_version}` : ''}` : '上游离线'}
            sx={{ display: { xs: 'none', md: 'inline-flex' } }}
          />
          <Tooltip title="刷新">
            <IconButton
              onClick={refresh}
              disabled={refreshing}
              sx={{ display: { xs: 'none', sm: 'inline-flex' } }}
            >
              <Refresh />
            </IconButton>
          </Tooltip>
          <Tooltip title={mode === 'dark' ? '切换浅色主题' : '切换深色主题'}>
            <IconButton onClick={toggleMode}>{mode === 'dark' ? <LightMode /> : <DarkMode />}</IconButton>
          </Tooltip>
          <Tooltip title="管理员">
            <IconButton onClick={event => setMenuAnchor(event.currentTarget)}>
              <AccountCircle />
            </IconButton>
          </Tooltip>
          <Menu anchorEl={menuAnchor} open={!!menuAnchor} onClose={() => setMenuAnchor(null)}>
            <MenuItem
              onClick={() => {
                setMenuAnchor(null);
                setPasswordOpen(true);
              }}
            >
              <Key fontSize="small" sx={{ mr: 1.2 }} />
              修改管理密码
            </MenuItem>
            <MenuItem onClick={logoutAction}>
              <Logout fontSize="small" sx={{ mr: 1.2 }} />
              退出登录
            </MenuItem>
          </Menu>
        </Toolbar>
      </AppBar>

      <Box sx={{ display: 'flex', alignItems: 'flex-start' }}>
        {!isSetup && (
          <Box
            component="aside"
            sx={{
              display: { xs: 'none', md: 'block' },
              width: 244,
              flexShrink: 0,
              position: 'sticky',
              top: 64,
              maxHeight: 'calc(100vh - 64px)',
              minHeight: 'calc(100vh - 64px)',
              overflowY: 'auto',
              borderRight: 1,
              borderColor: 'divider',
            }}
          >
            <SidebarNav page={page} onSelect={setPage} status={status} />
          </Box>
        )}

        <Box sx={{ flexGrow: 1, minWidth: 0 }}>
          <Container maxWidth={isSetup ? 'sm' : 'xl'} sx={{ py: { xs: 2.5, sm: 4 } }}>
            {error && (
              <Alert severity="warning" sx={{ mb: 2 }} onClose={() => setError('')}>
                {error}
              </Alert>
            )}

            {isSetup ? (
              <Stack spacing={2.4}>
                <Card>
                  <CardContent sx={{ p: { xs: 2.4, sm: 3.4 } }}>
                    <Stack spacing={2.4}>
                      <Box>
                        <Typography variant="overline" color="primary" fontWeight={800}>
                          第一步
                        </Typography>
                        <Typography variant="h4" sx={{ mt: 0.4 }}>
                          先登录一个网易账号
                        </Typography>
                        <Typography color="text.secondary" sx={{ mt: 1 }}>
                          登录成功并添加为房间账号后，才会显示开房界面。每个网易账号对应一个本地联机房间。
                        </Typography>
                      </Box>
                      <AccountLogin onAdded={refresh} />
                    </Stack>
                  </CardContent>
                </Card>

                <Divider />
                <Typography variant="caption" color="text.secondary" textAlign="center">
                  管理员密码可在右上角菜单修改；网易登录密码只用于换取会话，不会保存。
                </Typography>
              </Stack>
            ) : (
              <ConfigFormProvider status={status} onChanged={refresh}>
                <Stack spacing={2.6}>
                  <Box hidden={page !== 'home'}>
                    <Overview status={status} onChanged={refresh} />
                  </Box>
                  <Box hidden={page !== 'accounts'}>
                    <AccountsPanel status={status} onChanged={refresh} />
                  </Box>
                  <Box hidden={page !== 'transfer'}>
                    <TransferPanel status={status} active={page === 'transfer'} />
                  </Box>
                  <Box hidden={page !== 'console'}>
                    <LogsPanel
                      events={status.events || []}
                      debugEnabled={!!status.config?.debug}
                      onDebugChanged={refresh}
                      active={page === 'console'}
                    />
                  </Box>
                  <Box hidden={page !== 'settings'}>
                    <SettingsForm status={status} />
                  </Box>
                </Stack>
              </ConfigFormProvider>
            )}
          </Container>

          <Box component="footer" sx={{ py: 3, textAlign: 'center' }}>
            <Typography variant="caption" color="text.secondary">
              CubeDock 方块码头 · 网易本地联机 → 网易协议上游
            </Typography>
          </Box>
        </Box>
      </Box>

      {!isSetup && (
        <Drawer
          open={navOpen}
          onClose={() => setNavOpen(false)}
          slotProps={{ paper: { sx: { width: 268 } } }}
        >
          <Toolbar>
            <Brand />
          </Toolbar>
          <SidebarNav
            page={page}
            onSelect={key => {
              setPage(key);
              setNavOpen(false);
            }}
            status={status}
          />
        </Drawer>
      )}

      <ChangePasswordDialog open={passwordOpen} onClose={() => setPasswordOpen(false)} />
    </Box>
  );
}


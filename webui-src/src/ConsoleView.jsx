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
import AccountDashboard from './AccountDashboard.jsx';
import AccountsPanel from './AccountsPanel.jsx';
import ChangePasswordDialog from './ChangePasswordDialog.jsx';
import DiagnosticsPanel from './DiagnosticsPanel.jsx';
import FaqPanel from './FaqPanel.jsx';
import LogsPanel from './LogsPanel.jsx';
import Overview from './Overview.jsx';
import ProfilesPanel from './ProfilesPanel.jsx';
import SettingsForm from './SettingsForm.jsx';
import SetupWizard from './SetupWizard.jsx';
import SidebarNav from './SidebarNav.jsx';
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
          background: 'linear-gradient(135deg,#f59e0b,#34d399)',
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
          Cube<span style={{ color: '#f59e0b' }}>Dock</span>
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
  const [accountIndex, setAccountIndex] = useState(null);
  const [navOpen, setNavOpen] = useState(false);
  const [menuAnchor, setMenuAnchor] = useState(null);
  const [passwordOpen, setPasswordOpen] = useState(false);
  const [refreshing, setRefreshing] = useState(false);
  const [setupActive, setSetupActive] = useState(false);

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

  useEffect(() => {
    if (!status) return;
    const count = status.accounts?.length ?? 0;
    if (count === 0 && localStorage.getItem('cubedock-setup-done') !== '1') {
      setSetupActive(true);
    }
  }, [status]);

  const refresh = async () => {
    setRefreshing(true);
    await load();
    setRefreshing(false);
  };

  const logoutAction = async () => {
    setMenuAnchor(null);
    await logout();
    if (window.__CUBEDOCK_DEMO__) {
      window.location.reload();
      return;
    }
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

  const isSetup = setupActive;
  const openSetup = () => setSetupActive(true);
  const finishSetup = () => {
    localStorage.setItem('cubedock-setup-done', '1');
    setSetupActive(false);
  };

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
            label={status.upstream_online ? `服务器已连接${status.upstream_version ? ` · ${status.upstream_version}` : ''}` : '服务器未连接'}
            sx={{ display: { xs: 'none', md: 'inline-flex' } }}
          />
          {window.__CUBEDOCK_DEMO__ && (
            <Tooltip title="演示模式：数据只保存在当前页面，不会连接真实服务">
              <Chip size="small" color="warning" variant="outlined" label="演示模式" />
            </Tooltip>
          )}
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
            <SidebarNav
              page={accountIndex !== null ? 'account' : page}
              onSelect={key => {
                setAccountIndex(null);
                setPage(key);
              }}
              onOpenAccount={index => setAccountIndex(index)}
              status={status}
            />
          </Box>
        )}

        <Box sx={{ flexGrow: 1, minWidth: 0 }}>
          <Container maxWidth={isSetup ? 'md' : 'xl'} sx={{ py: { xs: 2.5, sm: 4 } }}>
            {error && (
              <Alert severity="warning" sx={{ mb: 2 }} onClose={() => setError('')}>
                {error}
              </Alert>
            )}

            {isSetup ? (
              <ConfigFormProvider status={status} onChanged={refresh}>
                <SetupWizard status={status} onChanged={refresh} onDone={finishSetup} />
              </ConfigFormProvider>
            ) : (
              <ConfigFormProvider status={status} onChanged={refresh}>
                {accountIndex !== null && status.accounts?.[accountIndex] ? (
                  <AccountDashboard
                    status={status}
                    accountIndex={accountIndex}
                    onChanged={refresh}
                    onBack={() => setAccountIndex(null)}
                    onOpenProfiles={() => {
                      setAccountIndex(null);
                      setPage('profiles');
                    }}
                  />
                ) : (
                  <Stack spacing={2.6}>
                    <Box hidden={page !== 'home'}>
                      <Overview
                        status={status}
                        onChanged={refresh}
                        onOpenSetup={openSetup}
                        onOpenAccount={index => setAccountIndex(index)}
                      />
                    </Box>
                    <Box hidden={page !== 'accounts'}>
                      <AccountsPanel
                        status={status}
                        onChanged={refresh}
                        onOpenDashboard={index => setAccountIndex(index)}
                      />
                    </Box>
                    <Box hidden={page !== 'profiles'}>
                      <ProfilesPanel status={status} active={page === 'profiles'} />
                    </Box>
                    <Box hidden={page !== 'console'}>
                      <LogsPanel
                        events={status.events || []}
                        debugEnabled={!!status.config?.debug}
                        onDebugChanged={refresh}
                        active={page === 'console'}
                      />
                    </Box>
                    <Box hidden={page !== 'diagnostics'}>
                      <DiagnosticsPanel status={status} active={page === 'diagnostics'} onOpenSetup={openSetup} />
                    </Box>
                    <Box hidden={page !== 'faq'}>
                      <FaqPanel />
                    </Box>
                    <Box hidden={page !== 'settings'}>
                      <SettingsForm status={status} onChanged={refresh} />
                    </Box>
                  </Stack>
                )}
              </ConfigFormProvider>
            )}
          </Container>

          <Box component="footer" sx={{ py: 3, textAlign: 'center' }}>
            <Typography variant="caption" color="text.secondary">
              CubeDock 方块码头 · 网易本地联机 → 你的服务器
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
            page={accountIndex !== null ? 'account' : page}
            onSelect={key => {
              setAccountIndex(null);
              setPage(key);
              setNavOpen(false);
            }}
            onOpenAccount={index => {
              setAccountIndex(index);
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


import React, { createContext, useCallback, useContext, useEffect, useMemo, useState } from 'react';
import { Alert, CssBaseline, Snackbar, ThemeProvider } from '@mui/material';
import { buildTheme } from './theme.js';
import { getStored, setStored } from './util.js';
import LoginView from './LoginView.jsx';
import ConsoleView from './ConsoleView.jsx';
import ConfirmDialog from './ConfirmDialog.jsx';

const ToastContext = createContext(() => {});
export const useToast = () => useContext(ToastContext);

const ConfirmContext = createContext(async () => false);
export const useConfirm = () => useContext(ConfirmContext);

export default function App() {
  const [mode, setMode] = useState(() => getStored('cubedock-theme', 'dark'));
  const [background, setBackground] = useState(() => getStored('cubedock-background', '1') !== '0');
  const [toast, setToast] = useState({ open: false, message: '', severity: 'info' });
  const [confirmRequest, setConfirmRequest] = useState(null);

  useEffect(() => {
    setStored('cubedock-theme', mode);
  }, [mode]);

  useEffect(() => {
    setStored('cubedock-background', background ? '1' : '0');
  }, [background]);

  const theme = useMemo(() => buildTheme(mode, background), [mode, background]);
  const notify = (message, severity = 'info') => setToast({ open: true, message, severity });
  const closeToast = () => setToast(current => ({ ...current, open: false }));
  const toggleMode = () => setMode(current => (current === 'dark' ? 'light' : 'dark'));
  const toggleBackground = () => setBackground(current => !current);
  const isLogin = window.location.pathname === '/login';

  const confirm = useCallback(
    options =>
      new Promise(resolve => {
        setConfirmRequest({ ...options, resolve });
      }),
    [],
  );

  const resolveConfirm = useCallback(result => {
    setConfirmRequest(current => {
      current?.resolve(result);
      return null;
    });
  }, []);

  return (
    <ThemeProvider theme={theme}>
      <CssBaseline />
      <ToastContext.Provider value={notify}>
        <ConfirmContext.Provider value={confirm}>
          {isLogin ? (
            <LoginView
              mode={mode}
              toggleMode={toggleMode}
              background={background}
              toggleBackground={toggleBackground}
            />
          ) : (
            <ConsoleView
              mode={mode}
              toggleMode={toggleMode}
              background={background}
              toggleBackground={toggleBackground}
            />
          )}
          <Snackbar
            open={toast.open}
            autoHideDuration={4500}
            onClose={closeToast}
            anchorOrigin={{ vertical: 'bottom', horizontal: 'right' }}
          >
            <Alert severity={toast.severity} variant="filled" onClose={closeToast} sx={{ width: '100%' }}>
              {toast.message}
            </Alert>
          </Snackbar>
          <ConfirmDialog
            open={confirmRequest !== null}
            title={confirmRequest?.title || '确认操作'}
            message={confirmRequest?.message || ''}
            confirmLabel={confirmRequest?.confirmLabel}
            cancelLabel={confirmRequest?.cancelLabel}
            severity={confirmRequest?.severity || 'warning'}
            onConfirm={() => resolveConfirm(true)}
            onCancel={() => resolveConfirm(false)}
          />
        </ConfirmContext.Provider>
      </ToastContext.Provider>
    </ThemeProvider>
  );
}


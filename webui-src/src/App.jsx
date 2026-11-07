import React, { createContext, useContext, useEffect, useMemo, useState } from 'react';
import { Alert, CssBaseline, Snackbar, ThemeProvider } from '@mui/material';
import { buildTheme } from './theme.js';
import LoginView from './LoginView.jsx';
import ConsoleView from './ConsoleView.jsx';

const ToastContext = createContext(() => {});
export const useToast = () => useContext(ToastContext);

export default function App() {
  const [mode, setMode] = useState(() => localStorage.getItem('cubedock-theme') || 'dark');
  const [toast, setToast] = useState({ open: false, message: '', severity: 'info' });

  useEffect(() => {
    localStorage.setItem('cubedock-theme', mode);
  }, [mode]);

  const theme = useMemo(() => buildTheme(mode), [mode]);
  const notify = (message, severity = 'info') => setToast({ open: true, message, severity });
  const closeToast = () => setToast(current => ({ ...current, open: false }));
  const toggleMode = () => setMode(current => (current === 'dark' ? 'light' : 'dark'));
  const isLogin = window.location.pathname === '/login';

  return (
    <ThemeProvider theme={theme}>
      <CssBaseline />
      <ToastContext.Provider value={notify}>
        {isLogin ? (
          <LoginView mode={mode} toggleMode={toggleMode} />
        ) : (
          <ConsoleView mode={mode} toggleMode={toggleMode} />
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
      </ToastContext.Provider>
    </ThemeProvider>
  );
}


import { createTheme, alpha } from '@mui/material/styles';

export function buildTheme(mode) {
  const dark = mode === 'dark';
  const primary = '#8b5cf6';
  const secondary = '#2dd4bf';
  const border = dark ? alpha('#ffffff', 0.09) : alpha('#3b0764', 0.1);

  return createTheme({
    palette: {
      mode,
      primary: { main: primary, light: '#a78bfa', dark: '#6d28d9', contrastText: '#ffffff' },
      secondary: { main: secondary, contrastText: '#042f2e' },
      background: dark
        ? { default: '#0b0714', paper: '#151022' }
        : { default: '#f6f3fc', paper: '#ffffff' },
      success: { main: '#4ade80' },
      warning: { main: '#fbbf24' },
      error: { main: '#fb7185' },
      info: { main: '#60a5fa' },
      divider: border,
      text: dark
        ? { primary: '#f1ecff', secondary: '#a99bc8' }
        : { primary: '#21152f', secondary: '#6b5b86' },
    },
    shape: { borderRadius: 16 },
    typography: {
      fontFamily: '"Inter", "Noto Sans SC", "Microsoft YaHei", system-ui, -apple-system, sans-serif',
      h1: { fontWeight: 850, letterSpacing: '-0.035em' },
      h2: { fontWeight: 850, letterSpacing: '-0.03em' },
      h3: { fontWeight: 800, letterSpacing: '-0.02em' },
      h4: { fontWeight: 800, letterSpacing: '-0.02em' },
      h5: { fontWeight: 750 },
      h6: { fontWeight: 750 },
      button: { textTransform: 'none', fontWeight: 700 },
      caption: { letterSpacing: '0.02em' },
    },
    components: {
      MuiCssBaseline: {
        styleOverrides: {
          html: { height: '100%' },
          body: {
            minHeight: '100%',
            backgroundImage: dark
              ? 'radial-gradient(1100px 520px at 8% -10%, rgba(139,92,246,.20), transparent 62%), radial-gradient(900px 420px at 100% 0%, rgba(45,212,191,.10), transparent 58%)'
              : 'radial-gradient(1100px 520px at 8% -10%, rgba(139,92,246,.13), transparent 62%), radial-gradient(900px 420px at 100% 0%, rgba(45,212,191,.08), transparent 58%)',
            backgroundAttachment: 'fixed',
          },
          '#root': { minHeight: '100vh' },
          '::selection': { background: alpha(primary, 0.35) },
        },
      },
      MuiCard: {
        styleOverrides: {
          root: {
            border: `1px solid ${border}`,
            backgroundImage: 'none',
          },
        },
      },
      MuiPaper: {
        styleOverrides: {
          root: { backgroundImage: 'none' },
        },
      },
      MuiButton: { defaultProps: { disableElevation: true } },
      MuiAppBar: {
        styleOverrides: {
          root: {
            backgroundImage: 'none',
            borderBottom: `1px solid ${border}`,
            backdropFilter: 'blur(18px)',
            backgroundColor: dark ? alpha('#0b0714', 0.78) : alpha('#ffffff', 0.8),
          },
        },
      },
      MuiChip: { styleOverrides: { root: { fontWeight: 700 } } },
      MuiTooltip: { defaultProps: { arrow: true } },
      MuiTableCell: { styleOverrides: { head: { fontWeight: 750, whiteSpace: 'nowrap' } } },
    },
  });
}


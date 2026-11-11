import { createTheme, alpha } from '@mui/material/styles';

const palettes = {
  dark: {
    primary: '#f59e0b',
    primaryLight: '#fbbf24',
    primaryDark: '#b45309',
    primaryContrast: '#1a1205',
    secondary: '#34d399',
    secondaryContrast: '#042f22',
    background: '#0a1410',
    paper: '#12201a',
    text: '#eaf6ef',
    textSecondary: '#9db5a8',
    border: alpha('#ffffff', 0.09),
    appBar: alpha('#0a1410', 0.78),
    scrollbar: alpha('#9db5a8', 0.26),
    scrollbarHover: alpha('#9db5a8', 0.46),
    menuShadow: '0 18px 48px rgba(0,0,0,.55)',
    glow: 'radial-gradient(1100px 520px at 8% -10%, rgba(245,158,11,.18), transparent 62%), radial-gradient(900px 420px at 100% 0%, rgba(52,211,153,.12), transparent 58%)',
  },
  light: {
    primary: '#b45309',
    primaryLight: '#d97706',
    primaryDark: '#78350f',
    primaryContrast: '#ffffff',
    secondary: '#047857',
    secondaryContrast: '#ffffff',
    background: '#f7f4ea',
    paper: '#fffdf6',
    text: '#1c2a23',
    textSecondary: '#5c7267',
    border: alpha('#14532d', 0.12),
    appBar: alpha('#fffdf6', 0.82),
    scrollbar: alpha('#5c7267', 0.24),
    scrollbarHover: alpha('#5c7267', 0.42),
    menuShadow: '0 18px 48px rgba(28,42,35,.16)',
    glow: 'radial-gradient(1100px 520px at 8% -10%, rgba(180,83,9,.12), transparent 62%), radial-gradient(900px 420px at 100% 0%, rgba(4,120,87,.08), transparent 58%)',
  },
};

export function buildTheme(mode) {
  const dark = mode === 'dark';
  const c = dark ? palettes.dark : palettes.light;

  return createTheme({
    palette: {
      mode,
      primary: {
        main: c.primary,
        light: c.primaryLight,
        dark: c.primaryDark,
        contrastText: c.primaryContrast,
      },
      secondary: { main: c.secondary, contrastText: c.secondaryContrast },
      background: { default: c.background, paper: c.paper },
      success: { main: '#4ade80' },
      warning: { main: '#fbbf24' },
      error: { main: '#fb7185' },
      info: { main: '#38bdf8' },
      divider: c.border,
      text: { primary: c.text, secondary: c.textSecondary },
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
            backgroundImage: c.glow,
            backgroundAttachment: 'fixed',
          },
          '#root': { minHeight: '100vh' },
          '::selection': { background: alpha(c.primary, 0.35) },
          '*': { scrollbarWidth: 'thin', scrollbarColor: `${c.scrollbar} transparent` },
          '*::-webkit-scrollbar': { width: 10, height: 10 },
          '*::-webkit-scrollbar-track': { background: 'transparent' },
          '*::-webkit-scrollbar-thumb': {
            backgroundColor: c.scrollbar,
            borderRadius: 10,
            border: '2px solid transparent',
            backgroundClip: 'padding-box',
          },
          '*::-webkit-scrollbar-thumb:hover': { backgroundColor: c.scrollbarHover },
          '*::-webkit-scrollbar-corner': { background: 'transparent' },
        },
      },
      MuiCard: {
        styleOverrides: {
          root: {
            border: `1px solid ${c.border}`,
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
            borderBottom: `1px solid ${c.border}`,
            backdropFilter: 'blur(18px)',
            backgroundColor: c.appBar,
          },
        },
      },
      MuiChip: { styleOverrides: { root: { fontWeight: 700 } } },
      MuiTooltip: { defaultProps: { arrow: true } },
      MuiTableCell: { styleOverrides: { head: { fontWeight: 750, whiteSpace: 'nowrap' } } },
      MuiMenu: {
        styleOverrides: {
          paper: {
            marginTop: 4,
            borderRadius: 12,
            border: `1px solid ${c.border}`,
            backgroundColor: c.paper,
            boxShadow: c.menuShadow,
            backgroundImage: 'none',
          },
          list: { padding: 6 },
        },
      },
      MuiMenuItem: {
        styleOverrides: {
          root: {
            borderRadius: 8,
            margin: '1px 4px',
            '&:hover': { backgroundColor: alpha(c.primary, 0.12) },
            '&.Mui-selected': { backgroundColor: alpha(c.primary, 0.2) },
            '&.Mui-selected:hover': { backgroundColor: alpha(c.primary, 0.26) },
          },
        },
      },
      MuiSelect: {
        styleOverrides: {
          icon: { color: c.textSecondary },
          select: { '&:focus': { backgroundColor: 'transparent' } },
        },
      },
    },
  });
}

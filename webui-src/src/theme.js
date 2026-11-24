import { createTheme, alpha } from '@mui/material/styles';
import backgroundImage from './assets/background.webp';

const softFont = '"Noto Sans SC", "Microsoft YaHei UI", "Microsoft YaHei", system-ui, sans-serif';

const palettes = {
  dark: {
    primary: '#f2a0bc',
    primaryLight: '#ffc2d6',
    primaryDark: '#c87494',
    primaryContrast: '#2b1621',
    secondary: '#8fe3d2',
    secondaryLight: '#b8f5e9',
    secondaryDark: '#62b8a7',
    secondaryContrast: '#102924',
    background: '#141625',
    paper: '#1d2033',
    text: '#f4f1fb',
    textSecondary: '#b3b0c8',
    border: alpha('#eee2ff', 0.12),
    appBar: alpha('#141625', 0.8),
    scrollbar: alpha('#c9bfe2', 0.24),
    scrollbarHover: alpha('#c9bfe2', 0.44),
    menuShadow: '0 18px 48px rgba(8, 8, 24, .42)',
    glow:
      'radial-gradient(1100px 540px at 8% -10%, rgba(242,160,188,.16), transparent 62%), ' +
      'radial-gradient(900px 460px at 100% 0%, rgba(174,169,255,.14), transparent 60%), ' +
      'radial-gradient(900px 520px at 50% 112%, rgba(143,227,210,.10), transparent 62%)',
    success: '#9ce8c8',
    successContrast: '#123027',
    warning: '#ffd79a',
    warningContrast: '#3b2a12',
    error: '#ff9aae',
    errorContrast: '#3b1520',
    info: '#a8c8ff',
    infoContrast: '#152744',
  },
  light: {
    primary: '#c95d86',
    primaryLight: '#e58bae',
    primaryDark: '#9e3e63',
    primaryContrast: '#fff8fc',
    secondary: '#46a38f',
    secondaryLight: '#7fd0bd',
    secondaryDark: '#2d7d6c',
    secondaryContrast: '#ffffff',
    background: '#fff8fc',
    paper: '#ffffff',
    text: '#3a2b3a',
    textSecondary: '#7c6b7f',
    border: alpha('#a86a8c', 0.16),
    appBar: alpha('#fff8fc', 0.86),
    scrollbar: alpha('#9b7b91', 0.22),
    scrollbarHover: alpha('#9b7b91', 0.4),
    menuShadow: '0 18px 48px rgba(94, 55, 80, .16)',
    glow:
      'radial-gradient(1100px 540px at 8% -10%, rgba(255,183,208,.2), transparent 64%), ' +
      'radial-gradient(900px 460px at 100% 0%, rgba(198,180,255,.16), transparent 62%), ' +
      'radial-gradient(900px 520px at 50% 112%, rgba(155,232,216,.12), transparent 64%)',
    success: '#3f9d86',
    successContrast: '#ffffff',
    warning: '#cf8f32',
    warningContrast: '#ffffff',
    error: '#d95670',
    errorContrast: '#ffffff',
    info: '#4a8dd6',
    infoContrast: '#ffffff',
  },
};

export function buildTheme(mode, showBackground = false) {
  const dark = mode === 'dark';
  const c = dark ? palettes.dark : palettes.light;
  const softBorder = `1px solid ${c.border}`;
  // Keep the starfield readable: a soft overlay sits between it and the UI,
  // and cards stay slightly translucent so the backdrop is still visible.
  const backdropOverlay = dark
    ? 'linear-gradient(180deg, rgba(20,22,37,.72), rgba(20,22,37,.62) 42%, rgba(20,22,37,.86))'
    : 'linear-gradient(180deg, rgba(255,248,252,.82), rgba(255,248,252,.74) 42%, rgba(255,248,252,.9))';

  return createTheme({
    palette: {
      mode,
      primary: {
        main: c.primary,
        light: c.primaryLight,
        dark: c.primaryDark,
        contrastText: c.primaryContrast,
      },
      secondary: {
        main: c.secondary,
        light: c.secondaryLight,
        dark: c.secondaryDark,
        contrastText: c.secondaryContrast,
      },
      background: { default: c.background, paper: c.paper },
      success: { main: c.success, contrastText: c.successContrast },
      warning: { main: c.warning, contrastText: c.warningContrast },
      error: { main: c.error, contrastText: c.errorContrast },
      info: { main: c.info, contrastText: c.infoContrast },
      divider: c.border,
      text: { primary: c.text, secondary: c.textSecondary },
    },
    shape: { borderRadius: 20 },
    typography: {
      fontFamily: softFont,
      h1: { fontWeight: 800, letterSpacing: '-0.015em' },
      h2: { fontWeight: 800, letterSpacing: '-0.012em' },
      h3: { fontWeight: 800, letterSpacing: '-0.01em' },
      h4: { fontWeight: 800, letterSpacing: '-0.01em' },
      h5: { fontWeight: 750 },
      h6: { fontWeight: 750 },
      button: { textTransform: 'none', fontWeight: 750, letterSpacing: '0.005em' },
      caption: { letterSpacing: '0.01em' },
    },
    components: {
      MuiCssBaseline: {
        styleOverrides: {
          html: { height: '100%' },
          body: {
            minHeight: '100%',
            backgroundImage: showBackground ? `${backdropOverlay}, url("${backgroundImage}")` : c.glow,
            backgroundSize: showBackground ? 'cover, cover' : undefined,
            backgroundPosition: showBackground ? 'center, center 32%' : undefined,
            backgroundRepeat: showBackground ? 'no-repeat' : undefined,
            backgroundAttachment: 'fixed',
          },
          '#root': { minHeight: '100vh' },
          '::selection': { background: alpha(c.primary, 0.32) },
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
            border: softBorder,
            borderRadius: 22,
            backgroundColor: showBackground ? alpha(c.paper, 0.88) : c.paper,
            backdropFilter: showBackground ? 'blur(12px)' : undefined,
            backgroundImage: dark
              ? 'linear-gradient(180deg, rgba(255,255,255,.025), rgba(255,255,255,0))'
              : 'linear-gradient(180deg, rgba(255,255,255,.9), rgba(255,248,252,.75))',
            transition: 'border-color .2s ease, box-shadow .2s ease',
          },
        },
      },
      MuiPaper: {
        styleOverrides: {
          root: { backgroundImage: 'none' },
        },
      },
      MuiButton: {
        defaultProps: { disableElevation: true },
        styleOverrides: {
          root: {
            borderRadius: 999,
            paddingInline: 18,
            fontWeight: 750,
            '&.Mui-focusVisible': {
              outline: `2px solid ${alpha(c.primary, 0.55)}`,
              outlineOffset: 2,
            },
          },
          containedPrimary: {
            boxShadow: `0 8px 22px ${alpha(c.primary, dark ? 0.22 : 0.24)}`,
          },
          outlined: {
            borderColor: alpha(c.primary, 0.45),
            '&:hover': { borderColor: c.primary, backgroundColor: alpha(c.primary, 0.08) },
          },
        },
      },
      MuiIconButton: {
        styleOverrides: {
          root: { borderRadius: 999 },
        },
      },
      MuiChip: {
        styleOverrides: {
          root: {
            borderRadius: 999,
            fontWeight: 700,
            borderColor: alpha(c.primary, 0.24),
          },
          label: { paddingInline: 1.2 },
        },
      },
      MuiToggleButton: {
        styleOverrides: {
          root: { borderRadius: 999, textTransform: 'none', fontWeight: 700 },
        },
      },
      MuiAppBar: {
        styleOverrides: {
          root: {
            backgroundImage: 'none',
            borderBottom: softBorder,
            backdropFilter: 'blur(18px)',
            backgroundColor: c.appBar,
          },
        },
      },
      MuiMenu: {
        styleOverrides: {
          paper: {
            marginTop: 6,
            borderRadius: 18,
            border: softBorder,
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
            borderRadius: 12,
            margin: '1px 4px',
            '&:hover': { backgroundColor: alpha(c.primary, 0.12) },
            '&.Mui-selected': { backgroundColor: alpha(c.primary, 0.18) },
            '&.Mui-selected:hover': { backgroundColor: alpha(c.primary, 0.24) },
          },
        },
      },
      MuiDialog: {
        styleOverrides: {
          paper: {
            borderRadius: 24,
            border: softBorder,
            backgroundImage: 'none',
            '&.MuiDialog-paperFullScreen': { borderRadius: 0, border: 0 },
          },
        },
      },
      MuiAlert: {
        styleOverrides: {
          root: { borderRadius: 14 },
        },
      },
      MuiOutlinedInput: {
        styleOverrides: {
          root: {
            borderRadius: 14,
            '& .MuiOutlinedInput-notchedOutline': { borderColor: alpha(c.textSecondary, 0.24) },
            '&:hover .MuiOutlinedInput-notchedOutline': { borderColor: alpha(c.primary, 0.55) },
            '&.Mui-focused .MuiOutlinedInput-notchedOutline': { borderColor: c.primary },
          },
        },
      },
      MuiSelect: {
        styleOverrides: {
          icon: { color: c.textSecondary },
          select: { '&:focus': { backgroundColor: 'transparent' } },
        },
      },
      MuiTabs: {
        styleOverrides: {
          indicator: { height: 3, borderRadius: 3 },
        },
      },
      MuiTab: {
        styleOverrides: {
          root: { textTransform: 'none', fontWeight: 700, minHeight: 42 },
        },
      },
      MuiTooltip: {
        defaultProps: { arrow: true },
        styleOverrides: {
          tooltip: {
            borderRadius: 10,
            backgroundColor: dark ? '#2a2740' : '#4a3444',
            fontWeight: 600,
          },
        },
      },
      MuiTableCell: {
        styleOverrides: {
          head: { fontWeight: 750, whiteSpace: 'nowrap' },
        },
      },
    },
  });
}

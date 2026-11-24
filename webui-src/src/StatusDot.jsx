import React from 'react';
import { Box } from '@mui/material';
import Typography from './Typography.jsx';
import Stack from './Stack.jsx';
import { phaseColor, phaseLabel } from './util.js';

const DOT_COLORS = {
  success: 'success.main',
  warning: 'warning.main',
  info: 'info.main',
  error: 'error.main',
  default: 'text.disabled',
};

// StatusDot is a compact alternative to a Chip: a single colored dot carries
// the state, and the label stays plain text next to it.
export function StatusDot({ color = 'default', size = 8, sx }) {
  return (
    <Box
      component="span"
      sx={{
        display: 'inline-block',
        width: size,
        height: size,
        borderRadius: '50%',
        bgcolor: DOT_COLORS[color] || DOT_COLORS.default,
        flexShrink: 0,
        ...sx,
      }}
    />
  );
}

export default function PhaseDot({ phase, label, sx }) {
  return (
    <Stack direction="row" spacing={0.7} alignItems="center" sx={{ minWidth: 0, ...sx }}>
      <StatusDot color={phaseColor(phase)} />
      <Typography variant="body2" color="text.secondary" noWrap>
        {label ?? phaseLabel(phase)}
      </Typography>
    </Stack>
  );
}

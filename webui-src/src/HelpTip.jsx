import React from 'react';
import { Box, IconButton, Stack, Tooltip, Typography } from '@mui/material';
import { Help } from '@mui/icons-material';

// HelpTip renders the small question mark used next to settings. Keep the
// text short and practical; long explanations belong in the FAQ.
export default function HelpTip({ title, placement = 'top' }) {
  if (!title) return null;
  return (
    <Tooltip
      arrow
      placement={placement}
      title={
        <Box sx={{ maxWidth: 280, py: 0.2 }}>
          <Typography variant="caption" component="div" sx={{ lineHeight: 1.5 }}>
            {title}
          </Typography>
        </Box>
      }
    >
      <IconButton size="small" aria-label="查看说明" sx={{ p: 0.25, color: 'text.secondary' }}>
        <Help sx={{ fontSize: 15 }} />
      </IconButton>
    </Tooltip>
  );
}

// FieldHint combines a normal helper line with the question-mark tooltip.
export function FieldHint({ children, help, placement }) {
  return (
    <Stack
      component="span"
      direction="row"
      spacing={0.4}
      alignItems="center"
      flexWrap="wrap"
      useFlexGap
    >
      {children ? <span>{children}</span> : null}
      {help ? <HelpTip title={help} placement={placement} /> : null}
    </Stack>
  );
}

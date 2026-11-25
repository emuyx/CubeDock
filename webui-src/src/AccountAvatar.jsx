import React from 'react';
import Avatar from '@mui/material/Avatar';

export default function AccountAvatar({ account, size = 32, sx }) {
  const label = account?.name || account?.label || account?.email || '?';
  const initial = [...String(label).trim()][0]?.toUpperCase() || '?';
  return (
    <Avatar
      src={account?.avatar_url || undefined}
      alt={label}
      sx={{
        width: size,
        height: size,
        fontSize: size * 0.42,
        bgcolor: 'primary.main',
        color: 'primary.contrastText',
        flexShrink: 0,
        ...sx,
      }}
    >
      {initial}
    </Avatar>
  );
}

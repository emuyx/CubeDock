import React from 'react';
import {
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  useMediaQuery,
  useTheme,
} from '@mui/material';
import NameAccountForm from './NameAccountForm.jsx';

export default function NameAccountDialog({
  open,
  index,
  initialName = '',
  rename = false,
  onClose,
  onNamed,
}) {
  const theme = useTheme();
  const phone = useMediaQuery(theme.breakpoints.down('sm'));
  return (
    <Dialog open={open} onClose={onClose} fullWidth maxWidth="sm" fullScreen={phone}>
      <DialogTitle>{rename ? '修改游戏名' : '设置游戏名'}</DialogTitle>
      <DialogContent>
        {index !== null && index !== undefined && (
          <NameAccountForm
            key={`${index}-${initialName}`}
            index={index}
            initialName={initialName}
            mode={rename ? 'rename' : 'name'}
            compact
            onDone={result => {
              onNamed?.(result);
            }}
          />
        )}
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose}>关闭</Button>
      </DialogActions>
    </Dialog>
  );
}

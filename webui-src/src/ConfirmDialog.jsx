import React from 'react';
import {
  Alert,
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
} from '@mui/material';

export default function ConfirmDialog({
  open,
  title,
  message,
  confirmLabel = '确认',
  cancelLabel = '取消',
  severity = 'warning',
  onConfirm,
  onCancel,
}) {
  return (
    <Dialog
      open={open}
      onClose={onCancel}
      maxWidth="xs"
      fullWidth
      // The global confirm must stay above nested dialogs such as the player
      // panel; MUI uses the same base z-index for every modal.
      sx={{ zIndex: theme => theme.zIndex.modal + 100 }}
    >
      <DialogTitle>{title}</DialogTitle>
      <DialogContent>
        <Alert severity={severity} variant="outlined">
          {message}
        </Alert>
      </DialogContent>
      <DialogActions>
        <Button onClick={onCancel}>{cancelLabel}</Button>
        <Button
          variant="contained"
          color={severity === 'error' ? 'error' : 'warning'}
          onClick={onConfirm}
          autoFocus
        >
          {confirmLabel}
        </Button>
      </DialogActions>
    </Dialog>
  );
}

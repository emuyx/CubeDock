import React, { useEffect, useMemo, useState } from 'react';
import {
  Alert,
  Box,
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Stack,
  Typography,
} from '@mui/material';
import { ContentCopy, Download, QrCode2 } from '@mui/icons-material';
import QRCode from 'qrcode';
import { copyText } from './util.js';
import { useToast } from './App.jsx';

function roomPayload(roomId, ownerUid) {
  const room = Number(roomId);
  const owner = Number(ownerUid);
  if (!Number.isSafeInteger(room) || room <= 0 || !Number.isSafeInteger(owner) || owner <= 0) {
    return null;
  }
  return JSON.stringify({
    room_id: room,
    from: 'room',
    sharing_place: 'qrcode',
    owner_id: owner,
  });
}

export default function ShareRoomDialog({
  open,
  onClose,
  roomId,
  ownerUid,
  roomName = '',
  password = '',
}) {
  const notify = useToast();
  const [qr, setQr] = useState('');
  const [error, setError] = useState('');
  const payload = useMemo(() => roomPayload(roomId, ownerUid), [roomId, ownerUid]);

  useEffect(() => {
    let active = true;
    setQr('');
    setError('');
    if (!open || !payload) return undefined;
    QRCode.toDataURL(payload, {
      errorCorrectionLevel: 'M',
      margin: 1,
      width: 512,
      color: { dark: '#141625', light: '#ffffff' },
    })
      .then(value => {
        if (active) setQr(value);
      })
      .catch(failure => {
        if (active) setError(failure.message || '生成二维码失败');
      });
    return () => {
      active = false;
    };
  }, [open, payload]);

  const inviteText = [
    `网易版 Minecraft「本地联机」房号：${roomId}`,
    `房间名：${roomName || '未命名房间'}`,
    '打开网易版 Minecraft → 本地联机 → 搜索房间号，',
    '或用联机大厅右上角「扫一扫」扫描二维码。',
    password ? `房间密码：${password}` : '如果房间设置了密码，请向房主询问。',
  ].join('\n');

  const copyInvite = async () => {
    try {
      await copyText(inviteText);
      notify('邀请文案已复制', 'success');
    } catch (failure) {
      notify(failure.message || '复制失败', 'error');
    }
  };

  const download = () => {
    if (!qr) return;
    const anchor = document.createElement('a');
    anchor.href = qr;
    anchor.download = `cubedock-room-${roomId}-qrcode.png`;
    document.body.appendChild(anchor);
    anchor.click();
    anchor.remove();
  };

  return (
    <Dialog open={open} onClose={onClose} fullWidth maxWidth="xs">
      <DialogTitle sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
        <QrCode2 color="primary" />
        房间二维码
      </DialogTitle>
      <DialogContent>
        <Stack spacing={2} alignItems="center">
          <Typography variant="body2" color="text.secondary" textAlign="center">
            网易版 Minecraft 打开「联机大厅」，点右上角「扫一扫」即可进入。
          </Typography>
          {error && <Alert severity="error">{error}</Alert>}
          {!payload && (
            <Alert severity="warning">
              房间还没有房间号，或账号缺少房主 UID，暂时不能生成二维码。
            </Alert>
          )}
          {payload && !qr && !error && (
            <Box sx={{ width: 260, height: 260, display: 'grid', placeItems: 'center' }}>
              <Typography variant="body2" color="text.secondary">
                正在生成…
              </Typography>
            </Box>
          )}
          {qr && (
            <Box
              component="img"
              src={qr}
              alt="房间二维码"
              sx={{
                width: 260,
                maxWidth: '100%',
                borderRadius: 2,
                bgcolor: '#fff',
                p: 1,
              }}
            />
          )}
          <Typography variant="caption" color="text.secondary" textAlign="center">
            房号 <strong>{roomId}</strong> · {roomName || '未命名房间'}
          </Typography>
        </Stack>
      </DialogContent>
      <DialogActions sx={{ px: 3, pb: 2.2 }}>
        <Button onClick={onClose} color="inherit">
          关闭
        </Button>
        <Button startIcon={<ContentCopy />} onClick={copyInvite} disabled={!roomId}>
          复制邀请
        </Button>
        <Button variant="contained" startIcon={<Download />} onClick={download} disabled={!qr}>
          保存二维码
        </Button>
      </DialogActions>
    </Dialog>
  );
}

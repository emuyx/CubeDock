export const phaseLabels = {
  stopped: '已停止',
  starting: '正在启动',
  checking_upstream: '检查服务器',
  authenticating: '正在鉴权',
  creating: '正在创建房间',
  online: '房间在线',
  stopping: '正在停止',
  retrying: '等待重连',
  upstream_offline: '后端离线',
};

export function phaseLabel(phase) {
  return phaseLabels[phase] || phase || '未知';
}

// isProxyRoom resolves the connection mode for one account's room instead of
// assuming a single global mode.
export function isProxyRoom(status, index) {
  const room = status?.rooms?.[index];
  const account = status?.accounts?.[index];
  return (room?.connection_mode || account?.connection_mode || 'proxy') === 'proxy';
}

export function phaseColor(phase) {
  switch (phase) {
    case 'online':
      return 'success';
    case 'retrying':
    case 'upstream_offline':
      return 'warning';
    case 'starting':
    case 'stopping':
    case 'checking_upstream':
    case 'authenticating':
    case 'creating':
      return 'info';
    default:
      return 'default';
  }
}

export function formatDuration(iso) {
  if (!iso) return '—';
  const started = Date.parse(iso);
  if (!Number.isFinite(started)) return '—';
  const seconds = Math.max(0, Math.floor((Date.now() - started) / 1000));
  const hours = Math.floor(seconds / 3600);
  const minutes = Math.floor((seconds % 3600) / 60);
  if (hours > 0) return `${hours} 时 ${minutes} 分`;
  if (minutes > 0) return `${minutes} 分 ${seconds % 60} 秒`;
  return `${seconds} 秒`;
}

export function eventLevel(message) {
  if (message.startsWith('[DEBUG]')) return 'debug';
  if (/失败|错误|断开|离线|超时|异常/.test(message)) return 'warning';
  return 'info';
}

export async function copyText(text) {
  if (navigator.clipboard?.writeText) {
    try {
      await navigator.clipboard.writeText(text);
      return;
    } catch {
      // LAN HTTP often blocks Clipboard API; fall back below.
    }
  }
  const field = document.createElement('textarea');
  field.value = text;
  field.readOnly = true;
  field.style.position = 'fixed';
  field.style.opacity = '0';
  document.body.appendChild(field);
  field.focus();
  field.select();
  try {
    if (!document.execCommand('copy')) throw new Error('浏览器不允许复制');
  } finally {
    field.remove();
  }
}

export function downloadText(filename, text) {
  const blob = new Blob([text], { type: 'text/plain;charset=utf-8' });
  const url = URL.createObjectURL(blob);
  const anchor = document.createElement('a');
  anchor.href = url;
  anchor.download = filename;
  document.body.appendChild(anchor);
  anchor.click();
  anchor.remove();
  setTimeout(() => URL.revokeObjectURL(url), 2000);
}

export function timestamp() {
  const now = new Date();
  const pad = value => String(value).padStart(2, '0');
  return `${now.getFullYear()}${pad(now.getMonth() + 1)}${pad(now.getDate())}-${pad(now.getHours())}${pad(now.getMinutes())}${pad(now.getSeconds())}`;
}


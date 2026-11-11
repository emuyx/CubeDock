const jsonHeaders = { 'Content-Type': 'application/json' };

export async function api(path, data, method = 'POST') {
  const init = { method, credentials: 'same-origin', headers: {} };
  if (data !== undefined) {
    init.headers = jsonHeaders;
    init.body = JSON.stringify(data);
  }
  const response = await fetch(`/api/${path}`, init);
  if (response.status === 401) {
    if (window.location.pathname !== '/login') {
      window.location.replace('/login');
    }
    throw new Error('登录已过期，请重新登录');
  }
  const text = await response.text();
  if (!response.ok) {
    throw new Error(text || `HTTP ${response.status}`);
  }
  if (!text) return {};
  try {
    return JSON.parse(text);
  } catch {
    return text;
  }
}

export const getStatus = () => api('status', undefined, 'GET');
export const getDiagnostics = () => api('diagnostics', undefined, 'GET');
export const getBackup = includePassword =>
  api(`backup?include_password=${includePassword ? 'true' : 'false'}`, undefined, 'GET');
export const restoreConfig = config => api('restore', { config });

export const runConsole = command => api('console', { command });

export async function login(username, password) {
  const response = await fetch('/api/login', {
    method: 'POST',
    credentials: 'same-origin',
    headers: jsonHeaders,
    body: JSON.stringify({ username, password }),
  });
  if (!response.ok) {
    throw new Error((await response.text()) || '登录失败');
  }
  return response.json();
}

export async function logout() {
  try {
    await api('logout', {});
  } catch {
    // The session is gone either way.
  }
}

export async function getSetupStatus() {
  const response = await fetch('/api/setup/status', { credentials: 'same-origin' });
  if (!response.ok) throw new Error((await response.text()) || '读取初始化状态失败');
  return response.json();
}

export async function completeSetup(username, password, confirm) {
  const response = await fetch('/api/setup', {
    method: 'POST',
    credentials: 'same-origin',
    headers: jsonHeaders,
    body: JSON.stringify({ username, password, confirm }),
  });
  if (!response.ok) throw new Error((await response.text()) || '创建管理员账户失败');
  return response.json();
}

// In-memory API used by the public demo build. It mirrors the shapes of the
// real handlers so the demo reuses the exact same Material UI console.

const nowISO = () => new Date().toISOString();
const clock = () => new Date().toTimeString().slice(0, 8);

const future = new Date(Date.now() + 6 * 3600 * 1000).toISOString();

const config = {
  connection_mode: 'proxy',
  limbo_upstream: '127.0.0.1:19133',
  limbo_path: '',
  limbo_packs_dir: '演示/packs',
  limbo_auto_pull_packs: true,
  limbo_form_title: 'CubeDock 中转房间',
  limbo_form_content: '点击下方按钮进入服务器。',
  limbo_form_button: '进入服务器',
  lobby_protocol_id: 43,
  upstream: '127.0.0.1:19132',
  room_name: '周末联机',
  room_game_version: '1.21.130.0',
  room_password: '',
  max_players: 10,
  friends_only: false,
  auto_accept_friends: true,
  allow_pvp: true,
  auto_start: false,
  web_port: 18321,
  retry_seconds: 10,
  debug: false,
};

const state = {
  config: { ...config },
  accounts: [
    { email: 'demo@example.com', label: '主号', name: '方块小助手', uid: '10001', level: 12, platform: 'interop', expires_at: future },
    { email: 'alt@example.com', label: '副号', name: '矿工阿明', uid: '10002', level: 8, platform: 'interop', expires_at: future },
    { email: 'afk@example.com', label: '挂机号', name: '夜猫子', uid: '10003', level: 21, platform: 'interop', expires_at: future },
  ],
  running: false,
  rooms: [],
  players: [],
  events: [
    { time: '09:12:01', source: '系统', level: 'info', message: '演示数据已载入；所有操作只在当前页面生效。' },
    { time: '09:12:03', source: '系统', level: 'info', message: '已连接服务器 127.0.0.1:19132（1.21.130.0）。' },
    { time: '09:12:05', source: '房间 1', level: 'info', message: '房主机器人持续保活，等待玩家加入。' },
  ],
  packs: {
    dir: '演示/packs',
    packs: [
      { file: 'demo-ui_1.0.0.mcpack', name: '演示界面包', uuid: '11111111-2222-3333-4444-555555555555', version: '1.0.0', size: 1048576 },
      { file: 'demo-audio_1.0.0.mcpack', name: '演示音效包', uuid: '66666666-7777-8888-9999-000000000000', version: '1.0.0', size: 524288 },
    ],
  },
  playerSeq: 0,
};

const append = (source, message, level = 'info') => {
  state.events = [...state.events.slice(-299), { time: clock(), source, level, message }];
};

const accountView = () =>
  state.accounts.map((account, index) => ({ index, ...account }));

const playerView = () =>
  state.players.map(player => ({
    ...player,
    uptime_seconds:
      Math.max(0, Math.round((Date.now() - new Date(player.joined_at).getTime()) / 1000)) || player.uptime_seconds,
  }));

function makePlayer(name, roomIndex, packets) {
  state.playerSeq += 1;
  return {
    id: `p${state.playerSeq}`,
    name,
    // Match the real API: PlayerStatus.Room is 1-based.
    room: roomIndex + 1,
    room_id: state.rooms[roomIndex]?.room_id || 0,
    account: state.accounts[roomIndex]?.name || '演示账号',
    state: 'joined',
    started_at: nowISO(),
    joined_at: nowISO(),
    uptime_seconds: 42,
    up_packets: packets,
    down_packets: packets * 3,
  };
}

function roomView(room, index) {
  const count = state.players.filter(player => player.room === index + 1).length;
  return { ...room, players: count, account: state.accounts[index]?.name || `房间 ${index + 1}` };
}

function startRooms() {
  if (state.running) return;
  state.running = true;
  state.rooms = state.accounts.map((_, index) => ({
    phase: 'creating',
    room_id: 888888 + index,
    players: 0,
    account: state.accounts[index]?.name || `房间 ${index + 1}`,
    protocol_id: 43,
    connected_at: nowISO(),
    retry_at: '',
    attempts: 0,
    last_error: '',
  }));
  append('系统', '正在创建房间…');
  window.setTimeout(() => {
    if (!state.running) return;
    state.rooms = state.rooms.map(room => ({ ...room, phase: 'online' }));
    state.players = [
      makePlayer('史蒂夫', 0, 128),
      makePlayer('爱丽丝', 0, 96),
      makePlayer('矿工小王', Math.min(1, state.accounts.length - 1), 64),
    ];
    append('房间 1', '房间已上线，玩家可以加入了。');
  }, 900);
}

function stopRooms() {
  state.running = false;
  state.rooms = [];
  state.players = [];
  append('系统', '房间已停止。');
}

function status() {
  const rooms = state.rooms.map(roomView);
  const accounts = accountView();
  const first = rooms[0];
  return {
    phase: first ? first.phase : 'stopped',
    running: state.running,
    room_id: first?.room_id || 0,
    players: state.players.length,
    account: first?.account || accounts[0]?.name || '',
    protocol_id: 43,
    connected_at: first?.connected_at || '',
    retry_at: '',
    attempts: 0,
    last_error: '',
    upstream_online: true,
    upstream_version: '1.21.130.0',
    token_set: accounts.length > 0,
    token_count: accounts.length,
    rooms,
    accounts,
    config: { ...state.config },
    events: [...state.events],
  };
}

const diagnostics = () => {
  const checks = [
    { id: 'config', category: '配置', label: '配置有效', status: 'ok', detail: '代理模式，服务器 127.0.0.1:19132' },
    { id: 'data_dir', category: '环境', label: '数据目录可读写', status: 'ok', detail: '演示模式：不会写入磁盘' },
    { id: 'upstream', category: '网络', label: '服务器可达', status: 'ok', detail: '127.0.0.1:19132 · 1.21.130.0' },
    { id: 'limbo', category: '资源', label: '中转程序', status: 'skip', detail: '代理模式不需要中转程序' },
    { id: 'packs', category: '资源', label: '资源包目录', status: 'ok', detail: '演示/packs（2 个包）' },
    { id: 'accounts', category: '账号', label: '网易账号', status: 'ok', detail: `${state.accounts.length} 个账号` },
    { id: 'mode', category: '配置', label: '房间策略', status: 'ok', detail: '未发现冲突' },
    {
      id: 'locks',
      category: '账号',
      label: '账号占用',
      status: state.running ? 'skip' : 'ok',
      detail: state.running ? '房间运行中，跳过账号占用检查' : '账号占用检查通过',
    },
    { id: 'webui', category: '控制台', label: 'WebUI 资源', status: 'ok', detail: '控制台页面可用' },
    { id: 'admin', category: '控制台', label: '管理员密码', status: 'ok', detail: '演示模式' },
    { id: 'logs', category: '控制台', label: '日志目录', status: 'ok', detail: '演示日志（不会落盘）' },
  ];
  const summary = checks.reduce((out, check) => ({ ...out, [check.status]: (out[check.status] || 0) + 1 }), {});
  return {
    generated_at: nowISO(),
    runtime: {
      version: 'demo',
      commit: 'demo',
      go: '演示环境',
      os: 'browser',
      arch: 'wasm',
      data_dir: '演示模式（不会写入磁盘）',
      web_port: 18321,
    },
    summary,
    checks,
  };
};

function backup(includePassword) {
  return {
    format: 1,
    app: 'CubeDock',
    version: 'demo',
    exported_at: nowISO(),
    config: { ...state.config, room_password: includePassword ? state.config.room_password : '' },
    accounts: accountView(),
    note: '演示备份：不包含网易登录凭证。',
  };
}

function consoleCommand(command) {
  const text = String(command || '').trim();
  if (!text) throw new Error('请输入命令');
  const [name, ...rest] = text.split(/\s+/);
  const arg = rest.join(' ');
  if (name === 'help') {
    return ['可用命令：', 'list — 查看玩家', 'kick <ID|名称> — 断开玩家', 'transfer <ID|名称> <host:port> — 转服（实验）'];
  }
  if (name === 'list') {
    if (!state.players.length) return ['当前没有在线玩家。'];
    return state.players.map(
      player => `${player.id}  ${player.name}  房间 ${player.room}  在线  ↑ ${player.up_packets} / ↓ ${player.down_packets}`,
    );
  }
  if (name === 'kick') {
    const player = state.players.find(item => item.id === arg || item.name === arg);
    if (!arg || !player) throw new Error('未找到玩家');
    state.players = state.players.filter(item => item.id !== player.id);
    append('控制台', `管理员已断开玩家 ${player.name || player.id}`);
    return [`已断开 ${player.name || player.id}`];
  }
  if (name === 'transfer') {
    const [target, address] = arg.split(/\s+/);
    const player = state.players.find(item => item.id === target || item.name === target);
    if (!player || !address) throw new Error('用法：transfer <ID|名称> <host:port>');
    append('控制台', `已请求玩家 ${player.name} 转服到 ${address}`);
    return [`已请求 ${player.name} 转服到 ${address}`];
  }
  throw new Error('未知命令；输入 help 查看可用命令');
}

const delay = ms => new Promise(resolve => window.setTimeout(resolve, ms));

const json = (data, statusCode = 200) =>
  new Response(JSON.stringify(data), {
    status: statusCode,
    headers: { 'Content-Type': 'application/json; charset=utf-8' },
  });

const failure = message => new Response(message, { status: 400, headers: { 'Content-Type': 'text/plain; charset=utf-8' } });

function addDemoAccount({ email = '', name = '', label = '' } = {}) {
  const count = state.accounts.length + 1;
  const account = {
    email: email || `demo${count}@example.com`,
    label: label || '演示账号',
    name: name || `演示账号 ${count}`,
    uid: String(10000 + count),
    level: 5 + count,
    platform: 'interop',
    expires_at: future,
  };
  state.accounts.push(account);
  append('账号', `已添加演示账号 ${account.name}`);
  return account;
}

function upsertDemoAccount(index, fields = {}) {
  const target = Number(index);
  if (Number.isInteger(target) && state.accounts[target]) {
    const current = state.accounts[target];
    state.accounts[target] = {
      ...current,
      email: fields.email || current.email,
      label: fields.label || current.label,
      name: fields.name || current.name,
      expires_at: future,
    };
    append('账号', `已更新房间 ${target + 1} 的登录：${state.accounts[target].name}`);
    return { account: state.accounts[target], index: target, updated: true };
  }
  return { account: addDemoAccount(fields), index: state.accounts.length - 1, updated: false };
}

export function installDemoApi() {
  if (window.__CUBEDOCK_DEMO_API__) return;
  window.__CUBEDOCK_DEMO_API__ = true;
  const originalFetch = window.fetch.bind(window);

  window.fetch = async (input, init = {}) => {
    const url = typeof input === 'string' ? input : input.url;
    if (!url.includes('/api/')) return originalFetch(input, init);
    const parsed = new URL(url, window.location.origin);
    const path = parsed.pathname.split('/api/')[1] || '';
    const method = (init.method || 'GET').toUpperCase();
    let body = {};
    if (init.body) {
      try {
        body = JSON.parse(init.body);
      } catch {
        body = {};
      }
    }
    await delay(80);
    try {
      switch (`${method} ${path}`) {
        case 'GET status':
          return json(status());
        case 'GET players':
          return json(playerView());
        case 'GET diagnostics':
          return json(diagnostics());
        case 'GET backup':
          return json(backup(parsed.searchParams.get('include_password') === 'true'));
        case 'GET packs':
          return json(state.packs);
        case 'POST packs/refresh':
          if (!state.packs.packs.some(pack => pack.file.startsWith('demo-extra'))) {
            state.packs.packs.push({
              file: 'demo-extra_1.0.0.mcpack',
              name: '演示额外资源包',
              uuid: 'aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee',
              version: '1.0.0',
              size: 262144,
            });
          }
          return json({ ...state.packs, pulled: 1, skipped: 0, removed: [], warnings: [] });
        case 'POST config':
          state.config = { ...state.config, ...(body.config || {}) };
          append('设置', '设置已保存（演示模式）。');
          return json({ ok: true });
        case 'POST restore':
          state.config = { ...state.config, ...(body.config || {}) };
          append('设置', '已从演示备份恢复配置。');
          return json({ ok: true, warnings: [] });
        case 'POST start':
          startRooms();
          return json({ ok: true });
        case 'POST stop':
          stopRooms();
          return json({ ok: true });
        case 'POST debug':
          state.config.debug = !!body.enabled;
          return json({ ok: true });
        case 'POST console':
          return json({ OK: true, Output: consoleCommand(body.command) });
        case 'POST exit':
          append('管理员', '演示模式不会真正退出。');
          return json({ ok: true });
        case 'POST session/import': {
          if (state.running) return failure('请先停止房间，再添加账号');
          const saved = upsertDemoAccount(body.index, { label: body.label });
          return json({ ok: true, index: saved.index, name: saved.account.name, uid: saved.account.uid, level: saved.account.level, platform: 'interop', expires_at: future });
        }
        case 'POST session/interop-login': {
          if (state.running) return failure('请先停止房间，再添加账号');
          if (String(body.email || '').includes('verify') || body.password === '1351') {
            return json({
              status: 'need_verify',
              message: '演示：网易要求完成一次安全验证。',
              verify_url: 'https://example.com/verify',
            });
          }
          const saved = upsertDemoAccount(body.index, {
            email: body.email,
            name: String(body.email || '').split('@')[0],
            label: body.label,
          });
          return json({ ok: true, index: saved.index, name: saved.account.name, uid: saved.account.uid, level: saved.account.level, platform: 'interop', expires_at: future });
        }
        case 'POST account/remove':
          if (state.running) return failure('请先停止房间，再移除账号');
          state.accounts.splice(Number(body.index), 1);
          return json({ ok: true });
        case 'POST account/profile': {
          const account = state.accounts[Number(body.index)];
          if (!account) return failure('账号序号无效');
          return json({ ok: true, name: account.name, uid: account.uid, level: account.level });
        }
        case 'POST player/kick': {
          const player = state.players.find(item => item.id === body.target || item.name === body.target);
          if (!player) return failure('未找到玩家');
          state.players = state.players.filter(item => item.id !== player.id);
          append('控制台', `管理员已断开玩家 ${player.name || player.id}`);
          return json(player);
        }
        case 'POST player/transfer': {
          const player = state.players.find(item => item.id === body.target || item.name === body.target);
          if (!player) return failure('未找到玩家');
          if (!body.address) return failure('请填写目标地址');
          append('控制台', `已请求玩家 ${player.name} 转服到 ${body.address}`);
          return json(player);
        }
        case 'POST admin/password':
          return json({ ok: true });
        case 'POST login':
        case 'POST logout':
          return json({ ok: true });
        default:
          return new Response('演示模式未实现该接口', { status: 404, headers: { 'Content-Type': 'text/plain' } });
      }
    } catch (error) {
      return failure(error.message || '演示操作失败');
    }
  };
}

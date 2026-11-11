// In-memory API used by the public demo build. It mirrors the shapes of the
// real handlers so the demo reuses the exact same Material UI console.

const nowISO = () => new Date().toISOString();
const clock = () => new Date().toTimeString().slice(0, 8);
const future = new Date(Date.now() + 6 * 3600 * 1000).toISOString();

const profileDefaults = {
  lobby_protocol_id: 43,
  room_game_version: '1.21.130.0',
  room_password: '',
  room_tags: [],
  min_level: 0,
  max_players: 10,
  friends_only: false,
  auto_accept_friends: true,
  allow_pvp: true,
  retry_seconds: 10,
  limbo_auto_pull_packs: true,
  limbo_form_title: 'CubeDock Limbo',
  limbo_form_content: '点击下方按钮进入服务器。',
  limbo_form_button: '进入服务器',
};

const profiles = [
  {
    ...profileDefaults,
    id: 'default',
    name: '默认配置',
    connection_mode: 'proxy',
    upstream: '127.0.0.1:19132',
    limbo_upstream: '127.0.0.1:19133',
    limbo_packs_dir: '',
    room_name: '周末联机',
  },
  {
    ...profileDefaults,
    id: 'creative',
    name: '创造服',
    connection_mode: 'transfer',
    upstream: '127.0.0.1:19134',
    limbo_upstream: '127.0.0.1:19135',
    limbo_packs_dir: '演示/packs/creative',
    room_name: '创造服中转',
    auto_accept_friends: false,
  },
];

const state = {
  config: {
    profiles,
    limbo_path: '',
    auto_start: false,
    web_port: 18321,
    debug: false,
  },
  accounts: [
    { email: 'demo@example.com', label: '主号', name: '方块小助手', uid: '10001', level: 12, platform: 'interop', expires_at: future, profile_id: 'default' },
    { email: 'alt@example.com', label: '副号', name: '矿工阿明', uid: '10002', level: 8, platform: 'interop', expires_at: future, profile_id: 'default' },
    { email: 'creative@example.com', label: '创造号', name: '建筑大师', uid: '10003', level: 21, platform: 'interop', expires_at: future, profile_id: 'creative' },
  ],
  running: new Set(),
  startedAt: {},
  players: [],
  playerSeq: 0,
  friendRequests: [
    { uid: '9007199254740993', name: '想一起玩的小林', apply_time: '1710000000' },
    { uid: '9002', name: '建筑爱好者', apply_time: '1710000500' },
  ],
  events: [
    { time: '09:12:01', source: '系统', level: 'info', message: '演示数据已载入；所有操作只在当前页面生效。' },
    { time: '09:12:03', source: '系统', level: 'info', message: '两个账号使用「默认配置」（代理），一个账号使用「创造服」（转服）。' },
  ],
  packs: {
    default: {
      dir: '演示/packs/default',
      packs: [
        { file: 'demo-ui_1.0.0.mcpack', name: '演示界面包', uuid: '11111111-2222-3333-4444-555555555555', version: '1.0.0', size: 1048576 },
      ],
    },
    creative: {
      dir: '演示/packs/creative',
      packs: [
        { file: 'demo-audio_1.0.0.mcpack', name: '演示音效包', uuid: '66666666-7777-8888-9999-000000000000', version: '1.0.0', size: 524288 },
      ],
    },
  },
};

const append = (source, message, level = 'info', room = 0, profile = '') => {
  state.events = [...state.events.slice(-299), { time: clock(), source, level, message, room, profile }];
};

const profileOf = account =>
  state.config.profiles.find(profile => profile.id === (account.profile_id || 'default')) ||
  state.config.profiles[0];

const accountView = () =>
  state.accounts.map((account, index) => {
    const profile = profileOf(account);
    return {
      index,
      ...account,
      profile_id: profile?.id || '',
      profile_name: profile?.name || '',
      connection_mode: profile?.connection_mode || 'proxy',
    };
  });

const playerView = () =>
  state.players.map(player => ({
    ...player,
    uptime_seconds:
      Math.max(0, Math.round((Date.now() - new Date(player.joined_at).getTime()) / 1000)) ||
      player.uptime_seconds,
  }));

function makePlayer(name, roomIndex, packets) {
  state.playerSeq += 1;
  return {
    id: `p${state.playerSeq}`,
    name,
    room: roomIndex + 1,
    room_id: 600000 + roomIndex,
    room_player_id: 1000 + state.playerSeq,
    account: state.accounts[roomIndex]?.name || '演示账号',
    state: 'joined',
    started_at: nowISO(),
    joined_at: nowISO(),
    uptime_seconds: 42,
    up_packets: packets,
    down_packets: packets * 3,
  };
}

function roomsView() {
  return state.accounts.map((account, index) => {
    const profile = profileOf(account);
    const running = state.running.has(index);
    const players = state.players.filter(player => player.room === index + 1).length;
    return {
      phase: running ? 'online' : 'stopped',
      room_id: running ? 600000 + index : 0,
      players,
      account: account.name || account.label || account.email,
      profile_id: profile?.id || '',
      profile_name: profile?.name || '',
      connection_mode: profile?.connection_mode || 'proxy',
      protocol_id: running ? profile?.lobby_protocol_id || 43 : 0,
      upstream_online: running,
      upstream_version: running ? profile?.room_game_version || '' : '',
      connected_at: running ? state.startedAt[index] || nowISO() : '',
      retry_at: '',
      attempts: 0,
      last_error: '',
    };
  });
}

function startAccount(index) {
  if (index < 0 || index >= state.accounts.length) throw new Error('账号序号无效');
  if (state.running.has(index)) return;
  state.running.add(index);
  state.startedAt[index] = nowISO();
  const account = state.accounts[index];
  const profile = profileOf(account);
  append('系统', `房间 ${index + 1} 正在启动（${profile?.name || '默认配置'}）`, 'info', index + 1, profile?.name);
  if (profile?.connection_mode === 'proxy') {
    state.players = [
      ...state.players,
      makePlayer(`玩家${index + 1}-A`, index, 128),
      makePlayer(`玩家${index + 1}-B`, index, 96),
    ];
  }
  append(`房间 ${index + 1}`, '房间已上线，玩家可以加入了。', 'info', index + 1, profile?.name);
}

function stopAccount(index) {
  if (!state.running.has(index)) return;
  state.running.delete(index);
  const profile = profileOf(state.accounts[index]);
  state.players = state.players.filter(player => player.room !== index + 1);
  append(`房间 ${index + 1}`, '房间已停止。', 'info', index + 1, profile?.name);
}

function startAll() {
  state.accounts.forEach((_, index) => startAccount(index));
}

function stopAll() {
  [...state.running].forEach(index => stopAccount(index));
}

function status() {
  const rooms = roomsView();
  const accounts = accountView();
  const first = rooms.find(room => room.phase !== 'stopped');
  const running = rooms.some(room => room.phase !== 'stopped');
  return {
    phase: running ? first?.phase || 'online' : 'stopped',
    running,
    room_id: first?.room_id || 0,
    players: state.players.length,
    account: first?.account || accounts[0]?.name || '',
    protocol_id: first?.protocol_id || 0,
    connected_at: first?.connected_at || '',
    retry_at: '',
    attempts: 0,
    last_error: '',
    upstream_online: running,
    upstream_version: first?.upstream_version || '1.21.130.0',
    token_set: accounts.length > 0,
    token_count: accounts.length,
    rooms,
    accounts,
    config: JSON.parse(JSON.stringify(state.config)),
    events: [...state.events],
  };
}

const diagnostics = profileId => {
  const selected = profileId
    ? state.config.profiles.filter(profile => profile.id === profileId)
    : state.config.profiles;
  const checks = [
    { id: 'config', category: '配置', label: '配置有效', status: 'ok', detail: `${selected.length} 个配置方案：${selected.map(profile => profile.name).join('、')}` },
    { id: 'data_dir', category: '环境', label: '数据目录可读写', status: 'ok', detail: '演示模式：不会写入磁盘' },
    ...selected.map(profile => ({
      id: `upstream-${profile.id}`,
      category: '网络',
      label: `服务器可达 · ${profile.name}`,
      status: 'ok',
      detail: `${profile.upstream} · 演示数据`,
    })),
    { id: 'limbo', category: '资源', label: '中转程序', status: selected.some(profile => profile.connection_mode === 'transfer') ? 'ok' : 'skip', detail: '演示模式：不启动真实进程' },
    { id: 'packs', category: '资源', label: '资源包目录', status: selected.some(profile => profile.connection_mode === 'transfer') ? 'ok' : 'skip', detail: '演示资源包（不会落盘）' },
    { id: 'accounts', category: '账号', label: '网易账号', status: 'ok', detail: `${state.accounts.length} 个账号` },
    { id: 'mode', category: '配置', label: '房间策略', status: 'ok', detail: '未发现冲突' },
    { id: 'locks', category: '账号', label: '账号占用', status: state.running.size ? 'skip' : 'ok', detail: state.running.size ? '有房间运行中，跳过账号占用检查' : '账号占用检查通过' },
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
      web_port: state.config.web_port,
    },
    summary,
    checks,
  };
};

function backup(includePassword) {
  const config = JSON.parse(JSON.stringify(state.config));
  if (!includePassword) {
    config.profiles.forEach(profile => {
      profile.room_password = '';
    });
  }
  return {
    format: 2,
    app: 'CubeDock',
    version: 'demo',
    exported_at: nowISO(),
    config,
    accounts: accountView(),
    note: '演示备份：包含配置方案和账号信息，不包含网易登录凭证或方案分配。',
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
    append('控制台', `管理员已断开玩家 ${player.name || player.id}`, 'info', player.room);
    return [`已断开 ${player.name || player.id}`];
  }
  if (name === 'transfer') {
    const [target, address] = arg.split(/\s+/);
    const player = state.players.find(item => item.id === target || item.name === target);
    if (!player || !address) throw new Error('用法：transfer <ID|名称> <host:port>');
    append('控制台', `已请求玩家 ${player.name} 转服到 ${address}`, 'info', player.room);
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

function addDemoAccount({ email = '', name = '', label = '', profile_id = 'default' } = {}) {
  const count = state.accounts.length + 1;
  const account = {
    email: email || `demo${count}@example.com`,
    label: label || '演示账号',
    name: name || `演示账号 ${count}`,
    uid: String(10000 + count),
    level: 5 + count,
    platform: 'interop',
    expires_at: future,
    profile_id,
  };
  state.accounts.push(account);
  const profile = profileOf(account);
  append('账号', `已添加演示账号 ${account.name}（${profile?.name || '默认配置'}）`);
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
      profile_id: fields.profile_id || current.profile_id,
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
          return json(diagnostics(parsed.searchParams.get('profile_id')));
        case 'GET backup':
          return json(backup(parsed.searchParams.get('include_password') === 'true'));
        case 'GET packs': {
          const id = parsed.searchParams.get('profile_id') || 'default';
          if (!state.packs[id]) state.packs[id] = { dir: `演示/packs/${id}`, packs: [] };
          return json(state.packs[id]);
        }
        case 'POST packs/refresh': {
          const id = body.profile_id || 'default';
          if (!state.packs[id]) state.packs[id] = { dir: `演示/packs/${id}`, packs: [] };
          if (!state.packs[id].packs.some(pack => pack.file.startsWith('demo-extra'))) {
            state.packs[id].packs.push({
              file: 'demo-extra_1.0.0.mcpack',
              name: '演示额外资源包',
              uuid: 'aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee',
              version: '1.0.0',
              size: 262144,
            });
          }
          return json({ ...state.packs[id], pulled: 1, skipped: 0, removed: [], warnings: [] });
        }
        case 'POST config':
          state.config = body.config || state.config;
          append('设置', '设置已保存（演示模式）。');
          return json({ ok: true });
        case 'POST restore':
          state.config = body.config || state.config;
          append('设置', '已从演示备份恢复配置。');
          return json({ ok: true, warnings: [] });
        case 'POST start':
          startAll();
          return json({ ok: true });
        case 'POST stop':
          stopAll();
          return json({ ok: true });
        case 'POST room/start':
          startAccount(Number(body.index));
          return json({ ok: true });
        case 'POST room/stop':
          stopAccount(Number(body.index));
          return json({ ok: true });
        case 'POST room/restart':
          stopAccount(Number(body.index));
          startAccount(Number(body.index));
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
          if (state.running.size) return failure('请先停止房间，再添加账号');
          const saved = upsertDemoAccount(body.index, { label: body.label, profile_id: body.profile_id });
          return json({
            ok: true,
            index: saved.index,
            name: saved.account.name,
            uid: saved.account.uid,
            level: saved.account.level,
            platform: 'interop',
            expires_at: future,
            profile_id: saved.account.profile_id,
          });
        }
        case 'POST session/interop-login': {
          if (state.running.size) return failure('请先停止房间，再添加账号');
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
            profile_id: body.profile_id,
          });
          return json({
            ok: true,
            index: saved.index,
            name: saved.account.name,
            uid: saved.account.uid,
            level: saved.account.level,
            platform: 'interop',
            expires_at: future,
            profile_id: saved.account.profile_id,
          });
        }
        case 'POST account/remove':
          if (state.running.size) return failure('请先停止房间，再移除账号');
          state.accounts.splice(Number(body.index), 1);
          return json({ ok: true });
        case 'POST account/assign-profile': {
          if (state.running.size) return failure('请先停止所有房间，再修改配置方案');
          const account = state.accounts[Number(body.index)];
          if (!account) return failure('账号序号无效');
          if (!state.config.profiles.some(profile => profile.id === body.profile_id)) {
            return failure('选择的配置方案不存在');
          }
          account.profile_id = body.profile_id;
          append('账号', `房间 ${Number(body.index) + 1} 已切换配置方案：${profileOf(account)?.name}`);
          return json({ ok: true, index: Number(body.index), profile_id: body.profile_id });
        }
        case 'POST account/profile': {
          const account = state.accounts[Number(body.index)];
          if (!account) return failure('账号序号无效');
          return json({ ok: true, name: account.name, uid: account.uid, level: account.level });
        }
        case 'GET friends': {
          const index = Number(parsed.searchParams.get('index'));
          if (!Number.isInteger(index)) return failure('账号序号无效');
          return json({ index, requests: state.friendRequests });
        }
        case 'POST friend/accept': {
          const uid = String(body.uid || '');
          const request = state.friendRequests.find(item => String(item.uid) === uid);
          if (!request) return failure('未找到好友申请');
          state.friendRequests = state.friendRequests.filter(item => String(item.uid) !== uid);
          append('账号', `演示：已同意 ${request.name} 的好友申请`);
          return json({ ok: true, index: Number(body.index), uid });
        }
        case 'POST player/room-kick': {
          const player = state.players.find(item => item.id === body.target || item.name === body.target);
          if (!player) return failure('未找到玩家');
          if (!player.room_player_id) return failure('玩家还没有房间玩家 ID');
          state.players = state.players.filter(item => item.id !== player.id);
          append('控制台', `演示：已请求从房间踢出玩家 ${player.name || player.id}`, 'info', player.room);
          return json(player);
        }
        case 'POST player/kick': {
          const player = state.players.find(item => item.id === body.target || item.name === body.target);
          if (!player) return failure('未找到玩家');
          state.players = state.players.filter(item => item.id !== player.id);
          append('控制台', `管理员已断开玩家 ${player.name || player.id}`, 'info', player.room);
          return json(player);
        }
        case 'POST player/transfer': {
          const player = state.players.find(item => item.id === body.target || item.name === body.target);
          if (!player) return failure('未找到玩家');
          if (!body.address) return failure('请填写目标地址');
          append('控制台', `已请求玩家 ${player.name} 转服到 ${body.address}`, 'info', player.room);
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

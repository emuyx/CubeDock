(function () {
  'use strict';

  var root = document.documentElement;
  var toast = document.getElementById('demo-toast');
  var toastTimer;

  function sleep(ms) {
    return new Promise(function (resolve) {
      window.setTimeout(resolve, ms);
    });
  }

  function notify(message) {
    if (!toast) return;
    toast.textContent = message;
    toast.classList.add('show');
    window.clearTimeout(toastTimer);
    toastTimer = window.setTimeout(function () {
      toast.classList.remove('show');
    }, 2400);
  }

  // Theme
  var themeToggle = document.querySelector('.theme-toggle');
  function syncThemeLabel() {
    if (!themeToggle) return;
    var dark = root.dataset.theme !== 'light';
    var text = dark ? '切换浅色主题' : '切换深色主题';
    themeToggle.setAttribute('aria-label', text);
    themeToggle.setAttribute('title', text);
  }
  if (themeToggle) {
    themeToggle.addEventListener('click', function () {
      var next = root.dataset.theme === 'light' ? 'dark' : 'light';
      root.dataset.theme = next;
      try {
        localStorage.setItem('cubedock-site-theme', next);
      } catch (error) {
        /* ignore */
      }
      syncThemeLabel();
    });
    syncThemeLabel();
  }

  // Mobile sidebar
  var sidebar = document.getElementById('demo-sidebar');
  document.getElementById('demo-menu')?.addEventListener('click', function () {
    sidebar?.classList.toggle('open');
  });

  // Page navigation
  var navItems = Array.prototype.slice.call(document.querySelectorAll('.demo-nav-item'));
  var pages = Array.prototype.slice.call(document.querySelectorAll('.demo-page'));

  function showPage(name) {
    navItems.forEach(function (item) {
      item.classList.toggle('active', item.getAttribute('data-page') === name);
    });
    pages.forEach(function (page) {
      page.classList.toggle('active', page.getAttribute('data-page') === name);
    });
    sidebar?.classList.remove('open');
  }

  navItems.forEach(function (item) {
    item.addEventListener('click', function () {
      showPage(item.getAttribute('data-page'));
    });
  });
  document.querySelectorAll('[data-goto]').forEach(function (button) {
    button.addEventListener('click', function () {
      showPage(button.getAttribute('data-goto'));
    });
  });

  // Generic demo actions
  document.querySelectorAll('[data-demo-action]').forEach(function (button) {
    button.addEventListener('click', function () {
      notify(button.getAttribute('data-demo-action') || '演示操作：不会连接真实服务');
    });
  });

  // Logs
  var logArea = document.getElementById('demo-log-area');
  var logCount = document.getElementById('demo-log-count');
  var logSearch = document.getElementById('demo-log-search');
  var showDebug = document.getElementById('demo-debug-lines');
  var autoScroll = document.getElementById('demo-autoscroll');
  var logs = [];
  var logClock = 0;

  function clock() {
    logClock++;
    var base = 22 * 3600 + logClock;
    var hh = String(Math.floor(base / 3600)).padStart(2, '0');
    var mm = String(Math.floor((base % 3600) / 60)).padStart(2, '0');
    var ss = String(base % 60).padStart(2, '0');
    return hh + ':' + mm + ':' + ss;
  }

  function addLog(message, level) {
    logs.push({ time: clock(), level: level || 'info', message: message });
    if (logs.length > 200) logs.shift();
    renderLogs();
  }

  function renderLogs() {
    if (!logArea) return;
    var query = (logSearch?.value || '').trim().toLowerCase();
    var includeDebug = !!showDebug?.checked;
    var visible = logs.filter(function (entry) {
      if (!includeDebug && entry.level === 'debug') return false;
      if (!query) return true;
      return (entry.message + ' ' + entry.level).toLowerCase().indexOf(query) >= 0;
    });
    logArea.textContent = '';
    if (visible.length === 0) {
      var empty = document.createElement('div');
      empty.className = 'demo-log-empty';
      empty.textContent = '暂无匹配日志';
      logArea.appendChild(empty);
    } else {
      visible.forEach(function (entry) {
        var line = document.createElement('div');
        line.className = 'demo-log-line ' + entry.level;
        var time = document.createElement('span');
        time.className = 'demo-log-time';
        time.textContent = entry.time;
        var level = document.createElement('span');
        level.className = 'demo-log-level';
        level.textContent = entry.level.toUpperCase();
        var message = document.createElement('span');
        message.className = 'demo-log-message';
        message.textContent = entry.message;
        line.appendChild(time);
        line.appendChild(level);
        line.appendChild(message);
        logArea.appendChild(line);
      });
    }
    if (logCount) logCount.textContent = '最近 ' + visible.length + ' 条';
    if (autoScroll?.checked) logArea.scrollTop = logArea.scrollHeight;
  }

  logSearch?.addEventListener('input', renderLogs);
  showDebug?.addEventListener('change', renderLogs);
  document.getElementById('demo-debug-toggle')?.addEventListener('change', function (event) {
    addLog(event.target.checked ? 'Debug 详细日志已开启' : 'Debug 详细日志已关闭', event.target.checked ? 'debug' : 'info');
  });
  document.getElementById('demo-export')?.addEventListener('click', function () {
    notify('演示：日志已导出（没有真实文件）');
  });

  // Room state
  var FAKE_ROOM = '#888888';
  var roomOnline = false;
  var starting = false;
  var players = 0;
  var uptimeSeconds = 0;
  var uptimeTimer = null;
  var startButton = document.getElementById('demo-start');
  var stopButton = document.getElementById('demo-stop');
  var joinButton = document.getElementById('demo-join');
  var copyButton = document.getElementById('demo-copy-room');
  var roomIdText = document.getElementById('demo-room-id');
  var roomNote = document.getElementById('demo-room-note');
  var phaseChip = document.getElementById('demo-phase-chip');
  var topPhase = document.getElementById('demo-top-phase');
  var topUpstream = document.getElementById('demo-top-upstream');
  var upstreamChip = document.getElementById('demo-upstream-chip');
  var upstreamNote = document.getElementById('demo-upstream-note');
  var backendState = document.getElementById('demo-backend-state');
  var onlinePlayers = document.getElementById('demo-online-players');
  var uptimeText = document.getElementById('demo-uptime');
  var sidebarPlayers = document.getElementById('demo-player-count');
  var eventsList = document.getElementById('demo-events');

  function setPhase(text) {
    if (phaseChip) phaseChip.textContent = text;
    if (topPhase) topPhase.textContent = text;
  }

  function formatUptime(seconds) {
    var mm = String(Math.floor(seconds / 60)).padStart(2, '0');
    var ss = String(seconds % 60).padStart(2, '0');
    return mm + ':' + ss;
  }

  function addEvent(message) {
    if (!eventsList) return;
    var item = document.createElement('li');
    var time = document.createElement('span');
    time.className = 'demo-event-time';
    time.textContent = clock();
    var dot = document.createElement('span');
    dot.className = 'demo-event-dot';
    item.appendChild(time);
    item.appendChild(dot);
    item.appendChild(document.createTextNode(message));
    eventsList.prepend(item);
    while (eventsList.children.length > 6) eventsList.removeChild(eventsList.lastChild);
  }

  function updatePlayerViews() {
    if (onlinePlayers) onlinePlayers.textContent = String(players);
    if (sidebarPlayers) sidebarPlayers.textContent = String(players);
    var rowPlayers = document.getElementById('demo-row-players');
    if (rowPlayers) rowPlayers.textContent = String(players);
    var rowStatus = document.getElementById('demo-row-status');
    if (rowStatus) {
      rowStatus.textContent = roomOnline ? '在线' : '已停止';
      rowStatus.className = roomOnline ? 'demo-chip demo-chip-good' : 'demo-chip';
    }
    var accountRoom = document.getElementById('demo-account-room-1');
    if (accountRoom) accountRoom.textContent = roomOnline ? FAKE_ROOM : '—';
  }

  function startUptime() {
    window.clearInterval(uptimeTimer);
    uptimeSeconds = 0;
    if (uptimeText) uptimeText.textContent = '00:00';
    uptimeTimer = window.setInterval(function () {
      uptimeSeconds++;
      if (uptimeText) uptimeText.textContent = formatUptime(uptimeSeconds);
    }, 1000);
  }

  async function startRoom() {
    if (roomOnline || starting) return;
    starting = true;
    if (startButton) startButton.disabled = true;
    if (stopButton) stopButton.disabled = true;
    if (joinButton) joinButton.disabled = true;
    if (copyButton) copyButton.disabled = true;

    setPhase('检查上游');
    addLog('检查上游服务器 127.0.0.1:19132 ...');
    await sleep(450);
    setPhase('鉴权中');
    addLog('互通版鉴权通过，房间协议 43');
    await sleep(450);
    setPhase('创建房间');
    addLog('创建本地联机房间 #888888（演示）');
    await sleep(550);

    roomOnline = true;
    starting = false;
    players = 0;
    setPhase('在线');
    if (topUpstream) topUpstream.textContent = '上游在线';
    if (upstreamChip) {
      upstreamChip.textContent = '已连接';
      upstreamChip.className = 'demo-chip demo-chip-good';
    }
    if (upstreamNote) upstreamNote.textContent = '演示握手完成 · 1.21.130';
    if (backendState) {
      backendState.textContent = '在线';
      backendState.className = 'demo-good';
    }
    if (roomIdText) {
      roomIdText.textContent = FAKE_ROOM;
      roomIdText.classList.add('demo-room-live');
    }
    if (roomNote) roomNote.textContent = '演示房间号 · 明显是假的，不会连接真实网易房间';
    if (copyButton) copyButton.disabled = false;
    if (stopButton) stopButton.disabled = false;
    if (joinButton) joinButton.disabled = false;
    updatePlayerViews();
    startUptime();
    addLog('房间 #888888 已上线；房主机器人持续保活');
    addEvent('房间 #888888 已上线（演示）');
    notify('演示：房间已创建，房间号 #888888（假）');
  }

  function stopRoom() {
    if (!roomOnline) return;
    roomOnline = false;
    players = 0;
    window.clearInterval(uptimeTimer);
    setPhase('已停止');
    if (topUpstream) topUpstream.textContent = '上游离线';
    if (upstreamChip) {
      upstreamChip.textContent = '未连接';
      upstreamChip.className = 'demo-chip';
    }
    if (upstreamNote) upstreamNote.textContent = '等待服务器响应';
    if (backendState) {
      backendState.textContent = '离线';
      backendState.className = '';
    }
    if (roomIdText) {
      roomIdText.textContent = '—';
      roomIdText.classList.remove('demo-room-live');
    }
    if (roomNote) roomNote.textContent = '启动房间后，房间号会显示在这里。';
    if (copyButton) copyButton.disabled = true;
    if (startButton) startButton.disabled = false;
    if (stopButton) stopButton.disabled = true;
    if (joinButton) joinButton.disabled = true;
    if (uptimeText) uptimeText.textContent = '—';
    updatePlayerViews();
    addLog('房间已停止，房主机器人断开');
    addEvent('房间已停止');
    notify('演示：房间已停止');
  }

  startButton?.addEventListener('click', startRoom);
  stopButton?.addEventListener('click', stopRoom);

  joinButton?.addEventListener('click', function () {
    if (!roomOnline) {
      notify('演示：请先启动房间');
      return;
    }
    if (players >= 9) {
      notify('演示：房间已满（最多 9 位玩家）');
      return;
    }
    players++;
    updatePlayerViews();
    addLog('玩家 Alice 已加入服务器（' + players + '/9）');
    addEvent('玩家 Alice 已加入服务器（' + players + '/9）');
    notify('演示：模拟玩家已加入，在线玩家 ' + players);
  });

  copyButton?.addEventListener('click', async function () {
    if (!roomOnline) return;
    try {
      await navigator.clipboard.writeText('888888');
      notify('已复制演示房间号 888888');
    } catch (error) {
      notify('演示房间号：888888');
    }
  });

  // Accounts
  var accountCount = 3;
  var accountsBody = document.getElementById('demo-accounts');
  var accountDialog = document.getElementById('demo-account-dialog');
  var accountForm = document.getElementById('demo-account-form');
  var accountCountNodes = [
    document.getElementById('demo-account-count'),
    document.getElementById('demo-stat-accounts')
  ];

  function updateAccountViews() {
    accountCountNodes.forEach(function (node) {
      if (node) node.textContent = String(accountCount);
    });
  }

  function accountRow(email, name, index) {
    var row = document.createElement('tr');
    ['房间 ' + index, email, name].forEach(function (value) {
      var cell = document.createElement('td');
      cell.textContent = value;
      row.appendChild(cell);
    });
    var status = document.createElement('td');
    var chip = document.createElement('span');
    chip.className = 'demo-chip demo-chip-warn';
    chip.textContent = '待验证';
    status.appendChild(chip);
    row.appendChild(status);
    var room = document.createElement('td');
    room.textContent = '—';
    row.appendChild(room);
    var count = document.createElement('td');
    count.textContent = '0';
    row.appendChild(count);
    var action = document.createElement('td');
    var verify = document.createElement('button');
    verify.type = 'button';
    verify.className = 'demo-small-btn';
    verify.setAttribute('data-row-action', 'verify');
    verify.textContent = '验证';
    var remove = document.createElement('button');
    remove.type = 'button';
    remove.className = 'demo-small-btn demo-small-danger';
    remove.setAttribute('data-row-action', 'remove');
    remove.textContent = '移除';
    action.appendChild(verify);
    action.appendChild(remove);
    row.appendChild(action);
    return row;
  }

  function openAccountDialog() {
    if (!accountDialog) return;
    if (typeof accountDialog.showModal === 'function') accountDialog.showModal();
    else accountDialog.setAttribute('open', '');
  }

  function closeAccountDialog() {
    if (!accountDialog) return;
    if (typeof accountDialog.close === 'function') accountDialog.close();
    else accountDialog.removeAttribute('open');
  }

  document.getElementById('demo-add-account')?.addEventListener('click', openAccountDialog);
  document.getElementById('demo-dialog-cancel')?.addEventListener('click', closeAccountDialog);
  accountDialog?.addEventListener('click', function (event) {
    if (event.target === accountDialog) closeAccountDialog();
  });

  accountForm?.addEventListener('submit', function (event) {
    event.preventDefault();
    var data = new FormData(accountForm);
    var email = String(data.get('email') || '').trim();
    if (!email) {
      notify('演示：请先填写邮箱');
      return;
    }
    accountCount++;
    accountsBody?.appendChild(accountRow(email, '演示玩家', accountCount));
    accountForm.reset();
    closeAccountDialog();
    updateAccountViews();
    addLog('已添加演示账号：' + email);
    notify('演示：已添加占位账号（不会登录网易）');
  });

  document.addEventListener('click', function (event) {
    var button = event.target.closest('[data-row-action]');
    if (!button) return;
    var row = button.closest('tr');
    if (!row) return;
    if (button.getAttribute('data-row-action') === 'remove') {
      row.remove();
      accountCount = accountsBody ? accountsBody.children.length : Math.max(0, accountCount - 1);
      updateAccountViews();
      addLog('演示账号已移除');
      notify('演示：账号已移除（仅前端）');
      return;
    }
    var chip = row.querySelector('.demo-chip');
    if (chip) {
      chip.className = 'demo-chip demo-chip-good';
      chip.textContent = '已验证';
    }
    addLog('演示：账号验证通过');
    notify('演示：账号验证通过（未连接网易）');
  });

  // Transfer
  document.getElementById('demo-connection-mode')?.addEventListener('change', function (event) {
    var note = document.getElementById('demo-mode-note');
    if (!note) return;
    note.textContent =
      event.target.value === 'transfer'
        ? '当前为转服模式：开房时启动 limbo，预加载资源包后通过 Transfer 进入目标服务器。'
        : '当前为代理模式，limbo 不会启动。切换到转服模式后才会在开房时启动 limbo。';
  });
  document.getElementById('demo-transfer-form')?.addEventListener('submit', function (event) {
    event.preventDefault();
    addLog('转服设置已保存（演示）');
    notify('演示：转服设置已保存（仅前端）');
  });

  function addPackRow(name, uuid, version, size, status, warn) {
    var body = document.getElementById('demo-packs');
    if (!body) return;
    var row = document.createElement('tr');
    [name, uuid, version, size].forEach(function (value) {
      var cell = document.createElement('td');
      cell.textContent = value;
      row.appendChild(cell);
    });
    var statusCell = document.createElement('td');
    var chip = document.createElement('span');
    chip.className = warn ? 'demo-chip demo-chip-warn' : 'demo-chip demo-chip-good';
    chip.textContent = status;
    statusCell.appendChild(chip);
    row.appendChild(statusCell);
    body.appendChild(row);
  }

  document.getElementById('demo-refresh-packs')?.addEventListener('click', function () {
    addLog('资源包列表已刷新（占位数据）');
    notify('演示：资源包列表已刷新（占位数据）');
  });
  document.getElementById('demo-pull-packs')?.addEventListener('click', function () {
    addPackRow('演示拉取包.mcpack', '44444444-5555-6666-7777-888888888888', '1.0.0', '6.1 MB', '已拉取', false);
    addLog('从后端服拉取资源包（模拟）');
    notify('演示：模拟拉取完成（不会连接后端）');
  });

  // Console commands
  var commandInput = document.getElementById('demo-command');

  function runCommand(raw) {
    var command = String(raw || '').trim();
    if (!command) {
      addLog('请输入命令', 'warn');
      return;
    }
    addLog('> ' + command);
    if (command === 'help') {
      addLog('可用命令：help、list、kick <ID|名称>、transfer <ID|名称> <host:port>');
    } else if (command === 'list') {
      if (players > 0) {
        addLog('p1  Alice    房间 1  在线  00:0' + Math.min(9, uptimeSeconds) + '  上行 128 包 / 下行 342 包');
      } else {
        addLog('当前没有在线玩家；可先在主页启动房间并模拟玩家加入');
      }
    } else if (command.indexOf('kick ') === 0) {
      if (players > 0) {
        players--;
        updatePlayerViews();
        addLog('演示：已请求断开 ' + command.slice(5) + '（不会影响真实玩家）');
      } else {
        addLog('未找到在线玩家；演示房间当前没有玩家');
      }
    } else if (command.indexOf('transfer ') === 0) {
      addLog('演示：已注入 Transfer 包（POC，未发送真实数据）');
    } else {
      addLog('未知命令 "' + command + '"；输入 help 查看可用命令', 'warn');
    }
  }

  document.getElementById('demo-run')?.addEventListener('click', function () {
    if (!commandInput) return;
    runCommand(commandInput.value);
    commandInput.value = '';
    commandInput.focus();
  });
  commandInput?.addEventListener('keydown', function (event) {
    if (event.key !== 'Enter') return;
    event.preventDefault();
    runCommand(commandInput.value);
    commandInput.value = '';
  });
  document.querySelectorAll('[data-command]').forEach(function (button) {
    button.addEventListener('click', function () {
      runCommand(button.getAttribute('data-command'));
    });
  });

  // Settings
  document.getElementById('demo-settings-form')?.addEventListener('submit', function (event) {
    event.preventDefault();
    addLog('房间设置已保存（演示）');
    notify('演示：设置已保存（仅前端）');
  });

  // Topbar actions
  document.getElementById('demo-refresh')?.addEventListener('click', function () {
    addLog('已刷新演示状态');
    notify('演示：状态已刷新');
  });
  document.getElementById('demo-admin')?.addEventListener('click', function () {
    notify('演示：管理员菜单（修改密码 / 退出登录）');
  });

  // Boot animation
  var boot = document.getElementById('demo-boot');
  var bootOutput = document.getElementById('demo-boot-output');
  var bootHint = document.getElementById('demo-boot-hint');
  var bootEnter = document.getElementById('demo-boot-enter');
  var bootSkip = document.getElementById('demo-boot-skip');
  var replay = document.getElementById('demo-replay');
  var bootRun = 0;
  var bootDone = false;

  var bootLines = [
    { text: 'CubeDock 方块码头 v0.1.1', cls: 'boot-strong', speed: 16 },
    { text: '┏━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┓', cls: 'boot-amber', speed: 3 },
    { text: '┃  CUBEDOCK  ▸  方块码头                               ┃', cls: 'boot-amber', speed: 5 },
    { text: '┃  网易本地联机  →  网易协议上游  ·  RakNet v8 直通    ┃', cls: 'boot-emerald', speed: 5 },
    { text: '┗━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┛', cls: 'boot-amber', speed: 3 },
    { text: '  CubeDock 方块码头 v0.1.1  ·  demo  ·  windows/amd64', cls: 'boot-muted', speed: 5 },
    { text: '', speed: 0 },
    { text: '[OK] 加载 config.json', cls: 'boot-ok', speed: 14 },
    { text: '[OK] 清理资源包目录', cls: 'boot-ok', speed: 14 },
    { text: '[OK] 初始化控制台 0.0.0.0:18321', cls: 'boot-ok', speed: 12 },
    { text: '[OK] 管理员账号 admin', cls: 'boot-ok', speed: 14 },
    { text: '[OK] 等待用户进入 WebUI...', cls: 'boot-ok', speed: 12 },
    { text: '', speed: 0 },
    { text: '> 演示模式：所有数据均为占位，不会连接网易。', cls: 'boot-warn', speed: 10 }
  ];

  function appendBootLine(line) {
    if (!bootOutput) return null;
    var row = document.createElement('div');
    row.className = 'demo-boot-line ' + (line.cls || '');
    row.textContent = line.text;
    bootOutput.appendChild(row);
    bootOutput.scrollTop = bootOutput.scrollHeight;
    return row;
  }

  async function typeBoot() {
    var run = ++bootRun;
    bootDone = false;
    if (bootOutput) bootOutput.textContent = '';
    if (bootHint) bootHint.textContent = '正在启动…';
    if (bootEnter) bootEnter.disabled = true;
    for (var index = 0; index < bootLines.length; index++) {
      if (run !== bootRun) return;
      var line = bootLines[index];
      var row = appendBootLine({ text: '', cls: line.cls });
      for (var cursor = 0; cursor < line.text.length; cursor++) {
        row.textContent += line.text[cursor];
        bootOutput.scrollTop = bootOutput.scrollHeight;
        await sleep(line.speed || 8);
        if (run !== bootRun) return;
      }
      await sleep(90);
    }
    if (run !== bootRun) return;
    bootDone = true;
    if (bootHint) bootHint.textContent = '按 Enter 进入 WebUI 演示';
    if (bootEnter) bootEnter.disabled = false;
  }

  function renderBootInstant() {
    bootRun++;
    if (bootOutput) bootOutput.textContent = '';
    bootLines.forEach(function (line) {
      appendBootLine(line);
    });
    bootDone = true;
    if (bootHint) bootHint.textContent = '按 Enter 进入 WebUI 演示';
    if (bootEnter) bootEnter.disabled = false;
  }

  function dismissBoot() {
    if (!boot) return;
    boot.classList.add('demo-boot-hidden');
    boot.setAttribute('aria-hidden', 'true');
    document.body.classList.remove('demo-booting');
  }

  function showBoot() {
    if (!boot) return;
    boot.classList.remove('demo-boot-hidden');
    boot.removeAttribute('aria-hidden');
    document.body.classList.add('demo-booting');
    if (bootEnter) bootEnter.disabled = true;
    if (bootHint) bootHint.textContent = '正在启动…';
    typeBoot();
  }

  bootSkip?.addEventListener('click', function () {
    renderBootInstant();
    dismissBoot();
  });
  bootEnter?.addEventListener('click', dismissBoot);
  replay?.addEventListener('click', showBoot);
  document.addEventListener('keydown', function (event) {
    if (event.key !== 'Enter' || !boot || boot.classList.contains('demo-boot-hidden')) return;
    if (!bootDone) renderBootInstant();
    dismissBoot();
  });

  // Initial state
  document.body.classList.add('demo-booting');
  updateAccountViews();
  updatePlayerViews();
  addLog('控制台已就绪', 'info');
  addLog('输入 help 查看命令；演示数据均为占位', 'info');
  typeBoot();
})();

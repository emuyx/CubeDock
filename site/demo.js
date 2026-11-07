(function () {
  'use strict';

  var root = document.documentElement;
  var toast = document.getElementById('demo-toast');
  var toastTimer;

  function notify(message) {
    if (!toast) return;
    toast.textContent = message;
    toast.classList.add('show');
    window.clearTimeout(toastTimer);
    toastTimer = window.setTimeout(function () {
      toast.classList.remove('show');
    }, 2400);
  }

  // Theme toggle
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
  var menuButton = document.getElementById('demo-menu');
  if (menuButton && sidebar) {
    menuButton.addEventListener('click', function () {
      sidebar.classList.toggle('open');
    });
  }

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
    if (sidebar) sidebar.classList.remove('open');
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

  // Demo room state
  var roomState = document.getElementById('demo-room-state');
  var roomHint = document.getElementById('demo-room-hint');
  document.getElementById('demo-start')?.addEventListener('click', function () {
    if (roomState) roomState.textContent = '运行中';
    if (roomHint) roomHint.textContent = '演示房间已启动（不会创建真实房间）';
    notify('演示：房间已启动，未连接网易');
  });
  document.getElementById('demo-stop')?.addEventListener('click', function () {
    if (roomState) roomState.textContent = '已停止';
    if (roomHint) roomHint.textContent = '演示房间已停止';
    notify('演示：房间已停止');
  });

  // Account dialog
  var accountDialog = document.getElementById('demo-account-dialog');
  var accountForm = document.getElementById('demo-account-form');
  var accountsBody = document.getElementById('demo-accounts');

  function openAccountDialog() {
    if (!accountDialog) return;
    if (typeof accountDialog.showModal === 'function') {
      accountDialog.showModal();
    } else {
      accountDialog.setAttribute('open', '');
    }
  }

  function closeAccountDialog() {
    if (!accountDialog) return;
    if (typeof accountDialog.close === 'function') {
      accountDialog.close();
    } else {
      accountDialog.removeAttribute('open');
    }
  }

  document.getElementById('demo-add-account')?.addEventListener('click', openAccountDialog);
  document.getElementById('demo-dialog-cancel')?.addEventListener('click', closeAccountDialog);
  accountDialog?.addEventListener('click', function (event) {
    if (event.target === accountDialog) closeAccountDialog();
  });

  function accountRow(email, name) {
    var row = document.createElement('tr');
    var cells = [email, name, String(1000000000 + Math.floor(Math.random() * 900000000))];
    cells.forEach(function (value) {
      var cell = document.createElement('td');
      cell.textContent = value;
      row.appendChild(cell);
    });
    var statusCell = document.createElement('td');
    var chip = document.createElement('span');
    chip.className = 'demo-chip demo-chip-good';
    chip.textContent = '演示账号';
    statusCell.appendChild(chip);
    row.appendChild(statusCell);

    var actionCell = document.createElement('td');
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
    actionCell.appendChild(verify);
    actionCell.appendChild(remove);
    row.appendChild(actionCell);
    return row;
  }

  accountForm?.addEventListener('submit', function (event) {
    event.preventDefault();
    var data = new FormData(accountForm);
    var email = String(data.get('email') || '').trim();
    if (!email) {
      notify('演示：请先填写邮箱');
      return;
    }
    if (accountsBody) accountsBody.appendChild(accountRow(email, '演示玩家'));
    accountForm.reset();
    closeAccountDialog();
    notify('演示：已添加占位账号（不会登录网易）');
  });

  // Row actions (delegated so newly added rows work too)
  document.addEventListener('click', function (event) {
    var button = event.target.closest('[data-row-action]');
    if (!button) return;
    var row = button.closest('tr');
    if (!row) return;
    if (button.getAttribute('data-row-action') === 'remove') {
      row.remove();
      notify('演示：账号已移除（仅前端）');
      return;
    }
    var chip = row.querySelector('.demo-chip');
    if (chip) {
      chip.className = 'demo-chip demo-chip-good';
      chip.textContent = '已验证';
    }
    notify('演示：账号验证通过（未连接网易）');
  });

  // Transfer form and packs
  document.getElementById('demo-transfer-form')?.addEventListener('submit', function (event) {
    event.preventDefault();
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
    notify('演示：资源包列表已刷新（占位数据）');
  });
  document.getElementById('demo-pull-packs')?.addEventListener('click', function () {
    addPackRow(
      '演示拉取包.mcpack',
      '44444444-5555-6666-7777-888888888888',
      '1.0.0',
      '6.1 MB',
      '已拉取',
      false
    );
    notify('演示：模拟拉取完成（不会连接后端）');
  });

  // Console
  var output = document.getElementById('demo-console-output');
  var commandInput = document.getElementById('demo-command');

  function appendOutput(line) {
    if (!output) return;
    output.textContent += '\n' + line;
    output.scrollTop = output.scrollHeight;
  }

  function runCommand(raw) {
    var command = String(raw || '').trim();
    if (!command) {
      appendOutput('> 请输入命令');
      return;
    }
    appendOutput('> ' + command);
    if (command === 'help') {
      appendOutput('可用命令：help、list、kick <ID|名称>、transfer <ID|名称> <host:port>');
    } else if (command === 'list') {
      appendOutput('p1  Alice    房间 1  在线  00:02:14  上行 128 包 / 下行 342 包');
      appendOutput('p2  Bob      房间 1  在线  00:00:42  上行 21 包 / 下行 67 包');
    } else if (command.indexOf('kick ') === 0) {
      appendOutput('演示：已请求断开 ' + command.slice(5) + '（不会影响真实玩家）');
    } else if (command.indexOf('transfer ') === 0) {
      appendOutput('演示：已注入 Transfer 包（POC，未发送真实数据）');
    } else {
      appendOutput('未知命令 "' + command + '"；输入 help 查看可用命令');
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
    notify('演示：设置已保存（仅前端）');
  });
})();

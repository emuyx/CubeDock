(function () {
  'use strict';

  var root = document.documentElement;
  var toggle = document.querySelector('.theme-toggle');

  function syncThemeLabel() {
    if (!toggle) return;
    var dark = root.dataset.theme !== 'light';
    toggle.setAttribute('aria-label', dark ? '切换浅色主题' : '切换深色主题');
    toggle.setAttribute('title', dark ? '切换浅色主题' : '切换深色主题');
  }

  if (toggle) {
    toggle.addEventListener('click', function () {
      var next = root.dataset.theme === 'light' ? 'dark' : 'light';
      root.dataset.theme = next;
      try {
        localStorage.setItem('cubedock-site-theme', next);
      } catch (error) {
        /* ignore storage failures */
      }
      syncThemeLabel();
    });
    syncThemeLabel();
  }

  document.querySelectorAll('.copy-btn').forEach(function (button) {
    button.addEventListener('click', function () {
      var target = document.getElementById(button.getAttribute('data-copy-target'));
      if (!target) return;
      var text = target.innerText;
      var done = function () {
        var previous = button.textContent;
        button.textContent = '已复制';
        window.setTimeout(function () {
          button.textContent = previous;
        }, 1600);
      };
      if (navigator.clipboard && navigator.clipboard.writeText) {
        navigator.clipboard.writeText(text).then(done).catch(function () {
          fallbackCopy(text, done);
        });
      } else {
        fallbackCopy(text, done);
      }
    });
  });

  function fallbackCopy(text, done) {
    var area = document.createElement('textarea');
    area.value = text;
    area.setAttribute('readonly', '');
    area.style.position = 'fixed';
    area.style.opacity = '0';
    document.body.appendChild(area);
    area.select();
    try {
      if (document.execCommand('copy')) done();
    } catch (error) {
      /* ignore copy failures */
    }
    document.body.removeChild(area);
  }

  var reveals = document.querySelectorAll('.reveal');
  var reduced = window.matchMedia && window.matchMedia('(prefers-reduced-motion: reduce)').matches;
  if (!('IntersectionObserver' in window) || reduced) {
    reveals.forEach(function (item) {
      item.classList.add('visible');
    });
  } else {
    var observer = new IntersectionObserver(
      function (entries) {
        entries.forEach(function (entry) {
          if (!entry.isIntersecting) return;
          entry.target.classList.add('visible');
          observer.unobserve(entry.target);
        });
      },
      { threshold: 0.12, rootMargin: '0px 0px -8% 0px' }
    );
    reveals.forEach(function (item) {
      observer.observe(item);
    });
  }

  var year = document.getElementById('year');
  if (year) year.textContent = String(new Date().getFullYear());
})();

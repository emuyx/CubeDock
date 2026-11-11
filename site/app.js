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

  var navToggle = document.querySelector('.nav-toggle');
  var siteNav = document.querySelector('.site-nav');

  function setSiteNav(open) {
    if (!siteNav || !navToggle) return;
    siteNav.dataset.open = open ? 'true' : 'false';
    navToggle.classList.toggle('is-open', open);
    navToggle.setAttribute('aria-expanded', open ? 'true' : 'false');
    navToggle.setAttribute('aria-label', open ? '关闭导航' : '打开导航');
  }

  if (navToggle && siteNav) {
    navToggle.addEventListener('click', function (event) {
      event.stopPropagation();
      setSiteNav(siteNav.dataset.open !== 'true');
    });
    siteNav.addEventListener('click', function (event) {
      if (event.target.closest('a')) setSiteNav(false);
    });
    document.addEventListener('click', function (event) {
      if (siteNav.dataset.open !== 'true') return;
      if (event.target.closest('.site-nav') || event.target.closest('.nav-toggle')) return;
      setSiteNav(false);
    });
    document.addEventListener('keydown', function (event) {
      if (event.key === 'Escape') setSiteNav(false);
    });
    var wide = window.matchMedia('(min-width: 1001px)');
    var closeOnWide = function (event) {
      if (event.matches) setSiteNav(false);
    };
    if (wide.addEventListener) wide.addEventListener('change', closeOnWide);
    else if (wide.addListener) wide.addListener(closeOnWide);
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

  var docsContent = document.querySelector('.docs-content');
  if (docsContent) {
    var docSections = Array.prototype.slice.call(docsContent.querySelectorAll('.doc-section'));
    var docToc = document.getElementById('docs-toc');
    var docLinks = docToc
      ? Array.prototype.slice.call(docToc.querySelectorAll('nav a'))
      : [];
    var docHero = docsContent.querySelector('.docs-hero');
    var docTitle = document.getElementById('docs-current-title');
    var docCount = document.getElementById('docs-page-count');
    var docPrev = document.getElementById('docs-prev');
    var docNext = document.getElementById('docs-next');
    var docBackdrop = document.querySelector('.docs-backdrop');
    var docMenu = document.querySelector('.docs-menu-toggle');
    var docClose = document.querySelector('.docs-drawer-close');
    var docIds = docSections.map(function (section) {
      return section.id;
    });
    var docTitles = docSections.map(function (section) {
      var heading = section.querySelector('h2');
      return heading ? heading.textContent.trim() : section.id;
    });

    function setDocDrawer(open) {
      if (!docToc) return;
      docToc.classList.toggle('is-open', open);
      if (docBackdrop) docBackdrop.classList.toggle('is-open', open);
      if (docMenu) docMenu.setAttribute('aria-expanded', open ? 'true' : 'false');
      document.body.classList.toggle('docs-drawer-open', open);
    }

    function currentDocId() {
      var hash = decodeURIComponent((location.hash || '').replace(/^#/, ''));
      return docIds.indexOf(hash) >= 0 ? hash : 'overview';
    }

    function showDocSection(id, options) {
      options = options || {};
      var index = docIds.indexOf(id);
      if (index < 0) index = 0;
      id = docIds[index];
      docSections.forEach(function (section, position) {
        var active = position === index;
        section.hidden = !active;
        section.classList.toggle('is-active', active);
      });
      if (docHero) docHero.hidden = id !== 'overview';
      docLinks.forEach(function (link) {
        var active = link.getAttribute('href') === '#' + id;
        if (active) link.setAttribute('aria-current', 'page');
        else link.removeAttribute('aria-current');
      });
      if (docTitle) docTitle.textContent = docTitles[index];
      if (docCount) docCount.textContent = index + 1 + ' / ' + docSections.length;
      if (docPrev) {
        docPrev.hidden = index === 0;
        if (index > 0) {
          docPrev.href = '#' + docIds[index - 1];
          docPrev.querySelector('strong').textContent = docTitles[index - 1];
        }
      }
      if (docNext) {
        var last = index === docSections.length - 1;
        docNext.hidden = last;
        if (!last) {
          docNext.href = '#' + docIds[index + 1];
          docNext.querySelector('strong').textContent = docTitles[index + 1];
        }
      }
      document.title = docTitles[index] + ' · CubeDock 使用文档';
      if (options.scroll) {
        var offset = window.innerWidth <= 1000 ? 120 : 84;
        var top = docsContent.getBoundingClientRect().top + window.scrollY - offset;
        window.scrollTo({
          top: Math.max(0, top),
          behavior: options.instant ? 'auto' : 'smooth',
        });
      }
      setDocDrawer(false);
    }

    function activateDocLink(event) {
      var href = event.currentTarget.getAttribute('href') || '';
      if (href.charAt(0) !== '#') return;
      event.preventDefault();
      var id = href.slice(1);
      setDocDrawer(false);
      if (location.hash === href) {
        showDocSection(id, { scroll: true });
      } else {
        history.pushState(null, '', href);
        showDocSection(id, { scroll: true });
      }
    }

    if (docSections.length) {
      showDocSection(currentDocId(), { scroll: false });
      if (location.hash && docIds.indexOf(location.hash.slice(1)) >= 0) {
        requestAnimationFrame(function () {
          showDocSection(currentDocId(), { scroll: true, instant: true });
        });
      }
    }

    docLinks.forEach(function (link) {
      link.addEventListener('click', activateDocLink);
    });
    if (docPrev) docPrev.addEventListener('click', activateDocLink);
    if (docNext) docNext.addEventListener('click', activateDocLink);

    window.addEventListener('hashchange', function () {
      showDocSection(currentDocId(), { scroll: true });
    });
    window.addEventListener('popstate', function () {
      showDocSection(currentDocId(), { scroll: true });
    });

    if (docMenu) {
      docMenu.addEventListener('click', function () {
        setDocDrawer(!docToc.classList.contains('is-open'));
      });
    }
    if (docClose) docClose.addEventListener('click', function () { setDocDrawer(false); });
    if (docBackdrop) docBackdrop.addEventListener('click', function () { setDocDrawer(false); });
    document.addEventListener('keydown', function (event) {
      if (event.key === 'Escape') setDocDrawer(false);
    });
    var wideDocs = window.matchMedia('(min-width: 1001px)');
    var closeDocsWide = function (event) {
      if (event.matches) setDocDrawer(false);
    };
    if (wideDocs.addEventListener) wideDocs.addEventListener('change', closeDocsWide);
    else if (wideDocs.addListener) wideDocs.addListener(closeDocsWide);
  }

  var year = document.getElementById('year');
  if (year) year.textContent = String(new Date().getFullYear());
})();

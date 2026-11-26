// =============================================================================
// SEAM TEXT — the scripts' own texts, in the viewer's language
//
// The page carries them in one attribute of <html>, data-i18n-texts, a JSON
// object written by the layout from the locale files (scriptTextsJSON,
// internal/ui/layout.go). SeamText(name, fallback) returns one, or the English
// fallback on a page that does not carry it.
// =============================================================================
window.SeamText = (function () {
  var texts = null;
  return function (name, fallback) {
    if (texts === null) {
      texts = {};
      try { texts = JSON.parse(document.documentElement.getAttribute('data-i18n-texts') || '{}') || {}; } catch (e) { texts = {}; }
    }
    return (typeof texts[name] === 'string' && texts[name]) || fallback;
  };
}());

// =============================================================================
// SEAM NAV — collapsible sidebar groups with localStorage persistence
//
// Group IDs mirror Go's navGroupID(): lowercase, accents stripped, spaces→hyphens.
// localStorage key: 'seam-nav-groups'  |  format: { "operacional": false, ... }
// Absent keys default to expanded. Active groups are always kept expanded.
//
// Collapse lifecycle:
//   1. Inline <script> in <head> injects !important CSS so collapsed groups
//      are hidden before the sidebar renders — no visible flash.
//   2. SeamNav.init() removes that <style> element, then applies the correct
//      max-height via inline JS style (no !important = JS wins from here on).
//   3. Toggle clicks animate with the transition already enabled.
// =============================================================================

window.SeamNav = (function () {
  'use strict';

  var STORAGE_KEY = 'seam-nav-groups';

  function loadState() {
    try { return JSON.parse(localStorage.getItem(STORAGE_KEY) || '{}'); }
    catch (e) { return {}; }
  }

  function saveState(state) {
    try { localStorage.setItem(STORAGE_KEY, JSON.stringify(state)); }
    catch (e) {}
  }

  function isExpanded(id, state) {
    return state[id] !== false; // default: expanded
  }

  // Apply expanded/collapsed state to one group.
  // Always uses a fixed 600px ceiling for expansion — avoids the scrollHeight=0
  // problem that occurs when measuring a currently-collapsed element.
  function applyGroup(id, expanded) {
    // querySelectorAll covers both desktop and mobile sidebars
    document.querySelectorAll('[data-nav-group-id="' + id + '"]').forEach(function(el) {
      el.style.maxHeight = expanded ? '600px' : '0';
      el.style.opacity   = expanded ? '1'     : '0';
    });
    document.querySelectorAll('[data-nav-arrow-id="' + id + '"]').forEach(function(el) {
      el.style.transform = expanded ? 'rotate(0deg)' : 'rotate(-90deg)';
    });
  }

  function toggle(id) {
    var state    = loadState();
    var expanded = !isExpanded(id, state);
    state[id]    = expanded;
    saveState(state);
    applyGroup(id, expanded);
  }

  function init() {
    // Step 1 — remove the flash-prevention <style> injected by the head script.
    // That style uses !important which would silently beat any JS inline style.
    // Once removed, JS inline styles are authoritative.
    document.querySelectorAll('style').forEach(function (el) {
      if (el.textContent.indexOf('nav-group-') !== -1) {
        el.parentNode.removeChild(el);
      }
    });

    // Step 2 — disable transitions while setting initial state so there is no
    // animation on page load (the head script already hid collapsed groups).
    var state = loadState();
    var btns  = document.querySelectorAll('[data-nav-group]');

    btns.forEach(function (btn) {
      var id        = btn.dataset.navGroup;
      var hasActive = btn.dataset.navGroupActive === 'true';
      // Active group always expands and its saved state is corrected so future
      // navigations don't incorrectly show it collapsed.
      var expanded;
      if (hasActive) {
        expanded  = true;
        state[id] = true;
      } else {
        expanded = isExpanded(id, state);
      }

      document.querySelectorAll('[data-nav-group-id="' + id + '"]').forEach(function(el) {
        el.style.transition = 'none';
        el.style.maxHeight  = expanded ? '600px' : '0';
        el.style.opacity    = expanded ? '1'     : '0';
      });
      document.querySelectorAll('[data-nav-arrow-id="' + id + '"]').forEach(function(el) {
        el.style.transition = 'none';
        el.style.transform  = expanded ? 'rotate(0deg)' : 'rotate(-90deg)';
      });
    });

    saveState(state);

    // Step 3 — re-enable transitions after one paint cycle so subsequent
    // toggle clicks animate smoothly.
    requestAnimationFrame(function () {
      btns.forEach(function (btn) {
        var id    = btn.dataset.navGroup;
        var items = document.getElementById('nav-group-' + id);
        var arrow = document.getElementById('nav-arrow-' + id);
        if (items) items.style.transition = 'max-height 200ms ease, opacity 150ms ease';
        if (arrow) arrow.style.transition = '';  // restore Tailwind transition-transform class
      });
    });
  }

  return { toggle: toggle, init: init };
})();


/**
 * Seam AMS - Base JavaScript
 * Handles notifications and HTMX configuration
 * Note: Dark mode is handled by minty's built-in DarkMode
 */

(function() {
  'use strict';

  // =============================================================================
  // TOAST NOTIFICATIONS
  // =============================================================================

  const Toast = {
    container: null,
    
    init() {
      this.container = document.getElementById('toast-container');
      if (!this.container) {
        this.container = document.createElement('div');
        this.container.id = 'toast-container';
        this.container.className = 'fixed bottom-4 right-4 z-50 space-y-2';
        document.body.appendChild(this.container);
      }
    },
    
    show(message, type = 'info', duration = 5000) {
      const toast = document.createElement('div');
      toast.className = `toast flex items-center gap-3 px-4 py-3 rounded-lg shadow-lg ${this.getTypeClasses(type)}`;
      
      const icon = this.getIcon(type);
      toast.innerHTML = `
        <span class="material-icons text-lg">${icon}</span>
        <span class="flex-1">${this.escapeHtml(message)}</span>
        <button class="hover:opacity-70" onclick="this.parentElement.remove()">
          <span class="material-icons text-lg">close</span>
        </button>
      `;
      
      this.container.appendChild(toast);
      
      if (duration > 0) {
        setTimeout(() => {
          toast.classList.add('toast-exit');
          setTimeout(() => toast.remove(), 200);
        }, duration);
      }
      
      return toast;
    },
    
    success(message, duration) { return this.show(message, 'success', duration); },
    error(message, duration) { return this.show(message, 'error', duration); },
    warning(message, duration) { return this.show(message, 'warning', duration); },
    info(message, duration) { return this.show(message, 'info', duration); },
    
    getTypeClasses(type) {
      const classes = {
        success: 'bg-green-600 text-white',
        error: 'bg-red-600 text-white',
        warning: 'bg-yellow-500 text-white',
        info: 'bg-gray-800 text-white'
      };
      return classes[type] || classes.info;
    },
    
    getIcon(type) {
      const icons = { success: 'check_circle', error: 'error', warning: 'warning', info: 'info' };
      return icons[type] || icons.info;
    },
    
    escapeHtml(text) {
      const div = document.createElement('div');
      div.textContent = text;
      return div.innerHTML;
    }
  };

  // =============================================================================
  // HTMX CONFIGURATION
  // =============================================================================

  const HTMXConfig = {
    init() {
      // Add CSRF token to all requests
      document.body.addEventListener('htmx:configRequest', (event) => {
        const csrfToken = document.querySelector('meta[name="csrf-token"]')?.content;
        if (csrfToken) {
          event.detail.headers['X-CSRF-Token'] = csrfToken;
        }
      });
      
      // Handle toast messages from response headers
      document.body.addEventListener('htmx:afterRequest', (event) => {
        const xhr = event.detail.xhr;
        const toastMessage = xhr.getResponseHeader('X-Toast-Message');
        const toastType = xhr.getResponseHeader('X-Toast-Type') || 'info';
        if (toastMessage) {
          Toast.show(toastMessage, toastType);
        }
      });
      
      // Handle errors
      document.body.addEventListener('htmx:responseError', (event) => {
        let message = 'An error occurred. Please try again.';
        try {
          const response = JSON.parse(event.detail.xhr.responseText);
          if (response.error) message = response.error;
        } catch (e) {}
        Toast.error(message);
      });
      
      // Handle connection errors
      document.body.addEventListener('htmx:sendError', () => {
        Toast.error('Connection error. Please check your network.');
      });
    }
  };

  // =============================================================================
  // TABLE SORTING
  // =============================================================================

  const TableSort = {
    init() {
      document.querySelectorAll('th[data-sort]').forEach(header => {
        header.addEventListener('click', () => this.sortTable(header));
      });
    },
    
    sortTable(header) {
      const table = header.closest('table');
      const tbody = table.querySelector('tbody');
      const rows = Array.from(tbody.querySelectorAll('tr'));
      const isAsc = header.classList.contains('sort-asc');
      
      table.querySelectorAll('th[data-sort]').forEach(th => {
        th.classList.remove('sort-asc', 'sort-desc');
      });
      header.classList.add(isAsc ? 'sort-desc' : 'sort-asc');
      
      const columnIndex = Array.from(header.parentNode.children).indexOf(header);
      rows.sort((a, b) => {
        const aVal = a.children[columnIndex]?.textContent.trim() || '';
        const bVal = b.children[columnIndex]?.textContent.trim() || '';
        const aNum = parseFloat(aVal), bNum = parseFloat(bVal);
        
        if (!isNaN(aNum) && !isNaN(bNum)) {
          return isAsc ? bNum - aNum : aNum - bNum;
        }
        return isAsc ? bVal.localeCompare(aVal) : aVal.localeCompare(bVal);
      });
      
      rows.forEach(row => tbody.appendChild(row));
    }
  };

  // =============================================================================
  // FK JUMP LINKS
  // =============================================================================
  // Keeps "open in new tab" links next to FK selects in sync with the selected
  // value. The select carries data-jump-base="/route"; the link has id="jump-{name}".
  // Called on init and after every HTMX swap into #seam-modal-body.

  const FKJumps = {
    init() {
      document.querySelectorAll('select[data-jump-base]').forEach(function(sel) {
        var link = document.getElementById('jump-' + sel.name);
        if (!link) return;
        function update() {
          var val = parseInt(sel.value, 10);
          if (val > 0) {
            link.href = sel.dataset.jumpBase + '/' + val + '/edit';
            link.style.display = '';
          } else {
            link.href = '#';
            link.style.display = 'none';
          }
        }
        // Avoid double-binding on re-init after HTMX swaps
        if (!sel.dataset.jumpInit) {
          sel.addEventListener('change', update);
          sel.dataset.jumpInit = '1';
        }
        update();
      });

      // Asset-type select uses data-jump-base but links to /{id} not /{id}/edit
      var atSel = document.getElementById('type_id');
      var atLink = document.getElementById('jump-type_id');
      if (atSel && atLink) {
        function updateAt() {
          var val = parseInt(atSel.value, 10);
          if (val > 0) {
            atLink.href = '/asset-types/' + val;
            atLink.style.display = '';
          } else {
            atLink.href = '#';
            atLink.style.display = 'none';
          }
        }
        if (!atSel.dataset.jumpInit) {
          atSel.addEventListener('change', updateAt);
          atSel.dataset.jumpInit = '1';
        }
        updateAt();
      }
    }
  };

  // =============================================================================
  // UTILITIES
  // =============================================================================

  const Utils = {
    formatDate(dateString, options = {}) {
      return new Date(dateString).toLocaleDateString(undefined, {
        year: 'numeric', month: 'short', day: 'numeric', ...options
      });
    },
    
    formatNumber(num) {
      return num.toString().replace(/\B(?=(\d{3})+(?!\d))/g, ',');
    },
    
    debounce(func, wait) {
      let timeout;
      return function(...args) {
        clearTimeout(timeout);
        timeout = setTimeout(() => func(...args), wait);
      };
    },
    
    async copyToClipboard(text) {
      try {
        await navigator.clipboard.writeText(text);
        Toast.success(SeamText('copied', 'Copied to clipboard'));
        return true;
      } catch (err) {
        Toast.error(SeamText('copy-failed', 'Failed to copy'));
        return false;
      }
    }
  };

  // =============================================================================
  // INITIALIZATION
  // =============================================================================

  function init() {
    Toast.init();
    HTMXConfig.init();
    TableSort.init();
    SeamNav.init();
    FKJumps.init();
  }

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', init);
  } else {
    init();
  }

  // Re-initialize after HTMX swaps
  document.body.addEventListener('htmx:afterSwap', () => {
    TableSort.init();
    FKJumps.init();
  });

  // Expose utilities globally
  window.Seam = { Toast, Utils };

})();

// =============================================================================


// =============================================================================
// SEAM MODAL — same visual language as the FSM editor property dialog
// Usage:
//   SeamModal.open(title)        — open with empty body (HTMX will fill it)
//   SeamModal.close()            — dismiss programmatically
//   SeamModal.setTitle(t)        — update header title
// The modal body element is <div id="seam-modal-body">.
// HTMX targets this div to load form partials.
// =============================================================================

// =============================================================================
// SEAM POPOVER — compact summary card anchored to a jump (info) button
// =============================================================================

window.SeamPopover = (function () {
  'use strict';
  var _card = null, _btn = null;

  function _outsideClose(e) {
    if (_card && !_card.contains(e.target) && e.target !== _btn) close();
  }

  function show(btn, name) {
    var sel = document.querySelector('select[name="' + name + '"]');
    if (!sel) return;
    var id = parseInt(sel.value, 10);
    if (id <= 0) return;
    var base = sel.dataset.jumpBase || btn.dataset.jumpBase || '';
    if (!base) return;
    var url = base + '/' + id + '/summary';

    if (_btn === btn && _card) { close(); return; }
    close();
    _btn = btn;

    var dark = document.documentElement.classList.contains('dark');
    _card = document.createElement('div');
    Object.assign(_card.style, {
      position:     'absolute',    zIndex:       '99999',
      top:          '100%',        right:        '0',
      marginTop:    '4px',         width:        '270px',
      background:   dark ? '#1e293b' : '#ffffff',
      border:       '1px solid ' + (dark ? '#4f46e5' : '#c7d2fe'),
      borderRadius: '8px',         boxShadow:    '0 8px 32px rgba(0,0,0,0.22)',
      padding:      '10px 12px',   fontSize:     '13px',
      color:        dark ? '#e2e8f0' : '#1e293b',  lineHeight: '1.5',
    });
    _card.innerHTML = '<div style="color:#94a3b8;font-size:12px;padding:4px 0">Cargando\u2026</div>';

    var container = btn.closest('.mb-1') || btn.parentElement;
    container.style.position = 'relative';
    container.appendChild(_card);

    fetch(url, { headers: { 'HX-Request': 'true' } })
      .then(function (r) { return r.text(); })
      .then(function (html) { if (_card) _card.innerHTML = html; })
      .catch(function ()    { if (_card) { _card.innerHTML = '<div style="color:#ef4444;font-size:12px"></div>'; _card.firstChild.textContent = SeamText('load-failed', 'Could not load'); } });

    setTimeout(function () {
      document.addEventListener('click', _outsideClose, true);
    }, 0);
  }

  function close() {
    if (_card && _card.parentNode) _card.parentNode.removeChild(_card);
    _card = null; _btn = null;
    document.removeEventListener('click', _outsideClose, true);
  }

  return { show: show, close: close };
}());

// =============================================================================
// SEAM MODAL 2 — stacked modal for FK entity in-context editing
// =============================================================================

window.SeamModal2 = (function () {
  'use strict';
  var _backdrop = null, _modal = null, _footer = null, _onKey = null;

  function _dark() { return document.documentElement.classList.contains('dark'); }
  function _css(el, p) { Object.assign(el.style, p); }

  function _build(anchorEl) {
    var dark = _dark();
    var _cont = (anchorEl instanceof Element) ? anchorEl : document.body;
    var _anch = (_cont !== document.body);

    _backdrop = document.createElement('div');
    if (_anch) {
      _cont.style.position = 'relative';
      if (!_cont.style.minHeight) _cont.style.minHeight = '360px';
      _css(_backdrop, { position:'absolute', inset:'0', background:'rgba(0,0,0,0)',
        zIndex:'100', display:'flex', alignItems:'flex-start', justifyContent:'center',
        paddingTop:'40px', transition:'background 180ms ease',
        minHeight:'100%' });
    } else {
      _css(_backdrop, { position:'fixed', inset:'0', background:'rgba(0,0,0,0)',
        zIndex:'10000', display:'flex', alignItems:'center', justifyContent:'center',
        transition:'background 180ms ease' });
    }

    _modal = document.createElement('div');
    _css(_modal, { background: dark ? '#1e293b' : '#ffffff',
      borderRadius:'10px', boxShadow:'0 24px 80px rgba(0,0,0,0.5)',
      border:'2px solid ' + (dark ? '#4f46e5' : '#6366f1'),
      width:'760px', maxWidth:'calc(100vw - 48px)',
      maxHeight:'calc(100vh - 80px)',
      display:'flex', flexDirection:'column', overflow:'hidden',
      fontFamily:'inherit', opacity:'0', transform:'scale(0.88)',
      transition:'opacity 160ms ease, transform 160ms ease' });

    var header = document.createElement('div');
    _css(header, { padding:'10px 16px', flexShrink:'0',
      borderBottom:'1px solid ' + (dark ? '#334155' : '#e0e7ff'),
      display:'flex', alignItems:'center', justifyContent:'space-between',
      background: dark ? '#1e3a5f' : '#eef2ff' });
    var titleEl = document.createElement('span');
    titleEl.id = 'seam-modal2-title';
    _css(titleEl, { fontWeight:'600', fontSize:'13px', color: dark ? '#c7d2fe' : '#3730a3' });
    var closeBtn = document.createElement('button');
    closeBtn.innerHTML = '<span class="material-icons" style="font-size:16px;line-height:1">close</span>';
    _css(closeBtn, { background:'none', border:'none', cursor:'pointer',
      color: dark ? '#818cf8' : '#6366f1', padding:'2px',
      display:'flex', alignItems:'center' });
    closeBtn.addEventListener('click', function () { close(); });
    header.append(titleEl, closeBtn);

    var body = document.createElement('div');
    body.id = 'seam-modal2-body';
    _css(body, { overflowY:'auto', padding:'16px 20px',
      maxHeight:'calc(100vh - 200px)',
      color: dark ? '#e2e8f0' : '#1e293b' });
    body.innerHTML = '<div style="text-align:center;padding:32px 0;color:#94a3b8;font-size:13px">Cargando\u2026</div>';

    _footer = document.createElement('div');
    _footer.id = 'seam-modal2-footer';
    _css(_footer, { display:'flex', alignItems:'center', justifyContent:'flex-end',
      padding:'8px 16px', gap:'8px',
      borderTop:'1px solid ' + (dark ? '#334155' : '#e2e8f0'),
      flexShrink:'0', background: dark ? '#1e293b' : '#ffffff', minHeight:'48px' });

    _modal.append(header, body, _footer);
    _backdrop.appendChild(_modal);
    _backdrop.addEventListener('click', function (e) { if (e.target === _backdrop) close(); });
    _onKey = function (e) { if (e.key === 'Escape') close(); };
    document.addEventListener('keydown', _onKey);
    _cont.appendChild(_backdrop);
    requestAnimationFrame(function () { requestAnimationFrame(function () {
      _backdrop.style.background = 'rgba(0,0,0,0.28)';
      _modal.style.opacity = '1'; _modal.style.transform = 'scale(1)';
    }); });
  }

  function open(title, url, opts) {
    if (_backdrop) close(true);
    var _ae = (opts && opts.anchorEl instanceof Element) ? opts.anchorEl : null;
    _build(_ae);
    var t = document.getElementById('seam-modal2-title');
    if (t) t.textContent = title;
    // Apply optional width override (issue 5 — narrow listing dialogs)
    if (opts && opts.maxWidth && _modal) {
      _modal.style.width = opts.maxWidth;
      _modal.style.maxWidth = 'calc(100vw - 48px)';
    }
    htmx.ajax('GET', url, { target: '#seam-modal2-body', swap: 'innerHTML' });
  }

  function close(immediate) {
    if (!_backdrop) return;
    if (_onKey) { document.removeEventListener('keydown', _onKey); _onKey = null; }
    var b = _backdrop, m = _modal;
    _backdrop = null; _modal = null; _footer = null;
    if (immediate) { if (b.parentNode) b.parentNode.removeChild(b); return; }
    b.style.background = 'rgba(0,0,0,0)';
    m.style.opacity = '0'; m.style.transform = 'scale(0.88)';
    setTimeout(function () { if (b.parentNode) b.parentNode.removeChild(b); }, 180);
  }

  return { open: open, close: close };
}());

// ── SeamUserMenu ───────────────────────────────────────────────────────────
window.SeamUserMenu = (function () {
  'use strict';
  var _open = false;
  function toggle() {
    var menu = document.getElementById('seam-user-menu');
    if (!menu) return;
    _open = !_open;
    menu.style.display = _open ? 'block' : 'none';
    if (_open) {
      // Close on outside click
      setTimeout(function () {
        document.addEventListener('click', function outside(e) {
          if (!menu.contains(e.target) && !e.target.closest('#seam-user-btn')) {
            _open = false; menu.style.display = 'none';
            document.removeEventListener('click', outside);
          }
        });
      }, 10);
    }
  }
  return { toggle: toggle };
}());

// ── SeamModuleSelector ────────────────────────────────────────────────────
// Mirrors SeamUserMenu exactly (MODULE_SPEC.md §4) -- same toggle-and-
// close-on-outside-click behaviour, same element-ID-pair convention
// (seam-module-btn / seam-module-menu), deliberately not shared code with
// SeamUserMenu since the two menus can be open independently and neither
// depends on the other's internal _open state.
window.SeamModuleSelector = (function () {
  'use strict';
  var _open = false;
  function toggle() {
    var menu = document.getElementById('seam-module-menu');
    if (!menu) return;
    var btn = document.getElementById('seam-module-btn');
    function set(open) {
      _open = open;
      menu.style.display = open ? 'block' : 'none';
      if (btn) btn.setAttribute('aria-expanded', open ? 'true' : 'false');
    }
    set(!_open);
    if (_open) {
      setTimeout(function () {
        function done() {
          document.removeEventListener('click', outside);
          document.removeEventListener('keydown', esc);
        }
        function outside(e) {
          if (!_open) { done(); return; }
          if (!menu.contains(e.target) && !e.target.closest('#seam-module-btn')) { set(false); done(); }
        }
        // Escape closes the panel and gives the focus back to its button
        function esc(e) {
          if (!_open) { done(); return; }
          if (e.key === 'Escape') { set(false); done(); if (btn) btn.focus(); }
        }
        document.addEventListener('click', outside);
        document.addEventListener('keydown', esc);
      }, 10);
    }
  }
  return { toggle: toggle };
}());

// Move data-modal-footer content to SeamModal2 footer after HTMX swap
document.body.addEventListener('htmx:afterSwap', function (e) {
  if (!e.detail.target || e.detail.target.id !== 'seam-modal2-body') return;
  var footerSrc = e.detail.target.querySelector('[data-modal-footer]');
  var footerDst = document.getElementById('seam-modal2-footer');
  if (footerSrc && footerDst) {
    footerDst.innerHTML = '';
    while (footerSrc.firstChild) footerDst.appendChild(footerSrc.firstChild);
    footerSrc.parentNode.removeChild(footerSrc);
  }
});

document.body.addEventListener('seamModal2Saved', function () { SeamModal2.close(); });

// =============================================================================
// SEAM SELECTOR (T-96) -- the "choose" half of the Selector dialog
// (internal/ui/selector.go). A selector row carries data-id and data-label;
// choosing it dispatches a 'seam:selector-chosen' CustomEvent on document
// with detail {key, id, label, target} (id and label are strings, target is
// the id of the opener's hidden input or '') and closes the modal. The opener
// listens for the event and matches detail.target to its own input. Values are
// read with dataset and never written back as HTML.
// =============================================================================
window.SeamSelector = (function () {
  'use strict';
  function choose(row) {
    if (!row || !row.dataset || !row.closest) return;
    var box = row.closest('[data-selector-key]');
    var id = row.dataset.id;
    if (!box || !id) return;
    document.dispatchEvent(new CustomEvent('seam:selector-chosen', {
      detail: {
        key: box.dataset.selectorKey || '',
        id: id,
        label: row.dataset.label || '',
        target: box.dataset.selectorTarget || ''
      }
    }));
    if (window.SeamModal2) SeamModal2.close();
  }
  return { choose: choose };
}());

// =============================================================================
// SEAM REPEATABLE (indexed naming) -- add/remove for repeatable fields that use
// indexed names such as blocks[0][lines][1][name] (internal/formengine/
// repeatable_indexed.go). Each container carries data-rep, data-token (the
// index token its template uses, e.g. __D0__) and data-next (the next free
// index). Rows are made from the <template> next to the rows, never by cloning
// a live row, so no widget state (autocomplete lists, typed values) is copied.
// Indices only grow, so DOM order is ascending index order.
// =============================================================================
window.SeamRepeatable = (function () {
  'use strict';
  // Bounds on the number of rows (schema minItems / maxItems): the rows
  // container carries data-min and data-max when the field has them
  // (internal/formengine/repeatable_bounds.go). The server renders the buttons
  // in the right state; this keeps them right as rows come and go, and refuses
  // to go past a bound even if a button were enabled by hand. Rows that are
  // shrinking away (fe-row-out) are not counted.
  function bound(c, attr) {
    var n = parseInt(c && c.getAttribute ? c.getAttribute(attr) : '', 10);
    return n > 0 ? n : 0;
  }
  function rowsOf(c) {
    return Array.prototype.filter.call(c.children, function (n) {
      return n.classList && n.classList.contains('repeatable-row') && !n.classList.contains('fe-row-out');
    });
  }
  function canAdd(c) {
    var max = bound(c, 'data-max');
    return !max || rowsOf(c).length < max;
  }
  function canRemove(c) {
    var min = bound(c, 'data-min');
    return !min || rowsOf(c).length > min;
  }
  function addButtonOf(c) {
    var n = c.nextElementSibling;
    while (n && !(n.classList && n.classList.contains('fe-add'))) n = n.nextElementSibling;
    return n;
  }
  function sync(c) {
    if (!c) return;
    var min = bound(c, 'data-min'), max = bound(c, 'data-max');
    if (!min && !max) return;
    var rows = rowsOf(c), n = rows.length;
    rows.forEach(function (r) {
      Array.prototype.forEach.call(r.children, function (x) {
        if (x.classList && x.classList.contains('fe-remove')) x.disabled = !!min && n <= min;
      });
    });
    var add = addButtonOf(c);
    if (add) add.disabled = !!max && n >= max;
  }
  function add(btn) {
    var box = btn && btn.closest ? btn.closest('[data-rep]') : null;
    if (!box) return;
    var tpl = box.querySelector(':scope > template');
    var rows = box.querySelector(':scope > .rep-rows');
    if (!tpl || !rows) return;
    if (!canAdd(rows)) return;
    var idx = parseInt(box.dataset.next || '0', 10);
    if (isNaN(idx)) idx = 0;
    box.dataset.next = String(idx + 1);
    var holder = document.createElement('div');
    holder.innerHTML = tpl.innerHTML.split(box.dataset.token).join(String(idx));
    var row = holder.firstElementChild;
    if (!row) return;
    rows.appendChild(row);
    if (window.SeamPicker) SeamPicker.init(row);
    if (window.SeamWidgets) SeamWidgets.init(row);
    sync(rows);
  }
  function remove(btn) {
    var row = btn && btn.closest ? btn.closest('.repeatable-row') : null;
    if (!row) return;
    var c = row.parentNode;
    if (c && !canRemove(c)) return;
    row.remove();
    sync(c);
  }
  return { add: add, remove: remove, canAdd: canAdd, canRemove: canRemove, sync: sync };
}());

// =============================================================================
// SEAM SELECT -- a searchable combobox in place of the browser's own select
// popup, for every <select data-seam-select> (the form engine's select widget).
// The native <select> stays in the DOM, visually hidden inside the wrapper, and
// remains what the form submits: choosing an option sets it and fires input +
// change, so htmx triggers, FK jumps and form handlers see what they always saw.
// Progressive: without this script the native select simply shows.
//   * enhanced at load and by a MutationObserver, so rows added to a repeatable
//     and htmx swaps are covered; a cloned row arrives with a stale wrapper,
//     which is unwrapped and rebuilt from the select's own state;
//   * the options are read when the panel opens, so a select whose options
//     change is always current; the trigger follows value/disabled changes;
//   * the panel lives on <body> (position: fixed), so no overflow:hidden
//     ancestor or modal can clip it; it flips above the trigger when short of
//     room below;
//   * keyboard: Enter/Space/ArrowDown/ArrowUp open; in the panel ArrowUp/Down,
//     Home/End, Enter select, Escape closes, Tab closes; typing filters
//     (case- and accent-insensitive). Lists of up to SEARCH_MIN options show no
//     search box and are driven from the list itself.
// Labels are written with textContent only, never as HTML.
// =============================================================================
window.SeamSelect = (function () {
  'use strict';
  var SEARCH_MIN = 8;
  var state = new WeakMap();   // select -> { wrap, trigger, label }
  var openState = null;        // the one open panel: { sel, panel, list, search, items, active, off... }

  function fold(s) {
    s = String(s || '').toLowerCase();
    return s.normalize ? s.normalize('NFD').replace(/[\u0300-\u036f]/g, '') : s;
  }
  function el(tag, cls, text) {
    var n = document.createElement(tag);
    if (cls) n.className = cls;
    if (text != null) n.textContent = text;
    return n;
  }
  function icon(name) { return el('span', 'material-icons fe-ss-chev', name); }

  // An option's icon (data-icon on the <option>, x-seam-option-icons) is drawn
  // before its text, or after it when the select says data-ss-icon-pos="right".
  // The name is re-checked here and written as text, never as markup.
  var ICON_RE = /^[a-z][a-z0-9_]{0,63}$/;
  function fillOption(node, iconName, text, right) {
    node.textContent = '';
    var t = el('span', 'fe-ss-text', text);
    if (iconName && ICON_RE.test(iconName)) {
      var i = el('span', 'material-icons fe-ss-icon', iconName);
      i.setAttribute('aria-hidden', 'true');
      if (right) { node.appendChild(t); node.appendChild(i); } else { node.appendChild(i); node.appendChild(t); }
    } else {
      node.appendChild(t);
    }
  }

  function readOptions(sel) {
    var out = [];
    Array.prototype.forEach.call(sel.children, function (c) {
      if (c.tagName === 'OPTGROUP') {
        out.push({ group: c.label || '' });
        Array.prototype.forEach.call(c.children, function (o) {
          out.push({ value: o.value, label: o.textContent, icon: o.getAttribute('data-icon') || '', disabled: o.disabled || c.disabled });
        });
      } else if (c.tagName === 'OPTION') {
        out.push({ value: c.value, label: c.textContent, icon: c.getAttribute('data-icon') || '', disabled: c.disabled });
      }
    });
    return out;
  }

  function refresh(sel) {
    var st = state.get(sel);
    if (!st) return;
    var o = sel.selectedIndex >= 0 ? sel.options[sel.selectedIndex] : null;
    var empty = !o || o.value === '';
    fillOption(st.label, o ? o.getAttribute('data-icon') : '', o ? o.textContent : '', sel.getAttribute('data-ss-icon-pos') === 'right');
    st.trigger.classList.toggle('fe-ss-placeholder', empty);
    st.trigger.disabled = !!sel.disabled;
    st.wrap.classList.toggle('fe-ss-disabled', !!sel.disabled);
  }

  function enhance(sel) {
    if (!sel || sel.tagName !== 'SELECT' || state.has(sel)) return;
    var parent = sel.parentNode;
    if (!parent) return;
    // A clone of an enhanced row carries the old wrapper: unwrap it first.
    if (parent.classList && parent.classList.contains('fe-ss')) {
      var old = parent;
      old.parentNode.insertBefore(sel, old);
      old.remove();
      parent = sel.parentNode;
    }
    var wrap = el('div', 'fe-ss');
    var trigger = el('button', 'fe-ss-trigger');
    trigger.type = 'button';
    trigger.setAttribute('role', 'combobox');
    trigger.setAttribute('aria-haspopup', 'listbox');
    trigger.setAttribute('aria-expanded', 'false');
    var al = sel.getAttribute('aria-label');
    if (al) trigger.setAttribute('aria-label', al);
    if (/border-red-500/.test(sel.className)) wrap.classList.add('fe-ss-error');
    var label = el('span', 'fe-ss-label');
    trigger.appendChild(label);
    trigger.appendChild(icon('expand_more'));
    parent.insertBefore(wrap, sel);
    wrap.appendChild(sel);
    wrap.appendChild(trigger);
    sel.classList.add('fe-ss-native');
    sel.tabIndex = -1;
    sel.setAttribute('aria-hidden', 'true');
    state.set(sel, { wrap: wrap, trigger: trigger, label: label });
    refresh(sel);

    trigger.addEventListener('click', function () { toggle(sel); });
    trigger.addEventListener('keydown', function (ev) {
      if (ev.key === 'ArrowDown' || ev.key === 'ArrowUp' || ev.key === 'Enter' || ev.key === ' ') {
        ev.preventDefault();
        open(sel);
      }
    });
    // a <label for>, a failed required check or a script focusing the select
    // lands on the visible control instead
    sel.addEventListener('focus', function () { trigger.focus(); });
    sel.addEventListener('change', function () { refresh(sel); });
    if (window.MutationObserver) {
      new MutationObserver(function () { refresh(sel); })
        .observe(sel, { childList: true, subtree: true, attributes: true, attributeFilter: ['disabled', 'selected'] });
    }
    if (sel.form) sel.form.addEventListener('reset', function () { setTimeout(function () { refresh(sel); }, 0); });
  }

  function toggle(sel) {
    if (openState && openState.sel === sel) close(true); else open(sel);
  }

  function open(sel) {
    var st = state.get(sel);
    if (!st || sel.disabled) return;
    if (openState) close(false);
    var opts = readOptions(sel);
    var real = opts.filter(function (o) { return o.group === undefined && o.value !== ''; }).length;
    var min = parseInt(sel.getAttribute('data-ss-min'), 10);
    if (!(min >= 1)) min = SEARCH_MIN;
    var showSearch = real >= min;

    var panel = el('div', 'fe-ss-panel');
    panel.tabIndex = -1;
    var search = null;
    if (showSearch) {
      var sw = el('div', 'fe-ss-searchwrap');
      sw.appendChild(el('span', 'material-icons fe-ss-searchicon', 'search'));
      search = el('input', 'fe-ss-search');
      search.type = 'text';
      search.autocomplete = 'off';
      search.setAttribute('aria-label', sel.getAttribute('data-ss-search') || 'Search');
      search.placeholder = sel.getAttribute('data-ss-search') || '';
      sw.appendChild(search);
      panel.appendChild(sw);
    }
    var list = el('ul', 'fe-ss-list');
    list.setAttribute('role', 'listbox');
    panel.appendChild(list);
    var none = el('div', 'fe-ss-none', sel.getAttribute('data-ss-empty') || '');
    none.hidden = true;
    panel.appendChild(none);

    var o = { sel: sel, panel: panel, list: list, search: search, none: none, items: [], active: -1 };
    var iconRight = sel.getAttribute('data-ss-icon-pos') === 'right';
    opts.forEach(function (op) {
      if (op.group !== undefined) {
        var g = el('li', 'fe-ss-group', op.group);
        g.setAttribute('role', 'presentation');
        list.appendChild(g);
        o.items.push({ node: g, group: true, text: fold(op.group) });
        return;
      }
      var li = el('li', 'fe-ss-opt');
      fillOption(li, op.icon, op.label, iconRight);
      li.setAttribute('role', 'option');
      if (op.value === '') li.classList.add('fe-ss-opt-empty');
      if (op.disabled) { li.classList.add('fe-ss-opt-off'); li.setAttribute('aria-disabled', 'true'); }
      if (op.value === sel.value) { li.classList.add('fe-ss-opt-sel'); li.setAttribute('aria-selected', 'true'); }
      li.addEventListener('mousedown', function (ev) { ev.preventDefault(); });
      li.addEventListener('click', function () { if (!op.disabled) choose(sel, op.value); });
      li.addEventListener('mousemove', function () { setActive(o, o.items.indexOf(item)); });
      var item = { node: li, group: false, value: op.value, disabled: op.disabled, text: fold(op.label) };
      list.appendChild(li);
      o.items.push(item);
    });

    document.body.appendChild(panel);
    openState = o;
    st.trigger.setAttribute('aria-expanded', 'true');
    st.wrap.classList.add('fe-ss-open');
    place(o);

    var cur = -1;
    o.items.forEach(function (it, i) { if (!it.group && it.value === sel.value) cur = i; });
    setActive(o, cur >= 0 ? cur : nextEnabled(o, -1, 1));
    (search || panel).focus();

    panel.addEventListener('keydown', function (ev) { onKey(o, ev); });
    if (search) search.addEventListener('input', function () { filter(o); });
    o.onDoc = function (ev) {
      if (panel.contains(ev.target) || st.wrap.contains(ev.target)) return;
      close(false);
    };
    o.onMove = function (ev) { if (!panel.contains(ev.target)) place(o); };
    document.addEventListener('mousedown', o.onDoc, true);
    window.addEventListener('resize', o.onMove);
    window.addEventListener('scroll', o.onMove, true);
  }

  function place(o) {
    var st = state.get(o.sel);
    var r = st.trigger.getBoundingClientRect();
    var p = o.panel;
    var vh = window.innerHeight || document.documentElement.clientHeight;
    p.style.minWidth = r.width + 'px';
    p.style.left = Math.max(4, Math.min(r.left, (window.innerWidth || 0) - r.width - 4)) + 'px';
    var below = vh - r.bottom - 8, above = r.top - 8;
    var want = Math.min(p.scrollHeight || 240, 320);
    if (below < want && above > below) {
      p.style.top = ''; p.style.bottom = (vh - r.top + 4) + 'px';
      p.style.maxHeight = Math.max(120, above) + 'px';
    } else {
      p.style.bottom = ''; p.style.top = (r.bottom + 4) + 'px';
      p.style.maxHeight = Math.max(120, below) + 'px';
    }
  }

  function nextEnabled(o, from, dir) {
    var n = o.items.length;
    for (var k = 1; k <= n; k++) {
      var i = from + dir * k;
      if (i < 0 || i >= n) return from >= 0 && from < n ? from : -1;
      var it = o.items[i];
      if (!it.group && !it.disabled && !it.node.hidden) return i;
    }
    return from;
  }

  function setActive(o, i) {
    if (o.active >= 0 && o.items[o.active]) o.items[o.active].node.classList.remove('fe-ss-opt-active');
    o.active = i;
    if (i >= 0 && o.items[i]) {
      var nd = o.items[i].node;
      nd.classList.add('fe-ss-opt-active');
      var lt = nd.offsetTop, lb = lt + nd.offsetHeight;
      if (lt < o.list.scrollTop) o.list.scrollTop = lt;
      else if (lb > o.list.scrollTop + o.list.clientHeight) o.list.scrollTop = lb - o.list.clientHeight;
    }
  }

  function filter(o) {
    var q = fold(o.search.value).trim();
    var any = false, groupNode = null, groupHas = false;
    o.items.forEach(function (it) {
      if (it.group) {
        if (groupNode) groupNode.hidden = !groupHas;
        groupNode = it.node; groupHas = false;
        return;
      }
      var show = !q || it.text.indexOf(q) !== -1;
      it.node.hidden = !show;
      if (show) { any = true; groupHas = true; }
    });
    if (groupNode) groupNode.hidden = !groupHas;
    o.none.hidden = any;
    var first = -1;
    for (var i = 0; i < o.items.length; i++) {
      var it2 = o.items[i];
      if (!it2.group && !it2.disabled && !it2.node.hidden) { first = i; break; }
    }
    setActive(o, first);
  }

  function onKey(o, ev) {
    var k = ev.key;
    if (k === 'ArrowDown') { ev.preventDefault(); setActive(o, nextEnabled(o, o.active, 1)); }
    else if (k === 'ArrowUp') { ev.preventDefault(); setActive(o, nextEnabled(o, o.active, -1)); }
    else if (k === 'Home') { ev.preventDefault(); setActive(o, nextEnabled(o, -1, 1)); }
    else if (k === 'End') { ev.preventDefault(); setActive(o, nextEnabled(o, o.items.length, -1)); }
    else if (k === 'Enter') {
      ev.preventDefault();
      var it = o.items[o.active];
      if (it && !it.group && !it.disabled) choose(o.sel, it.value);
    }
    else if (k === 'Escape') { ev.preventDefault(); ev.stopPropagation(); close(true); }
    else if (k === 'Tab') { close(false); }
  }

  function choose(sel, value) {
    var changed = sel.value !== value;
    sel.value = value;
    refresh(sel);
    close(true);
    if (changed) {
      sel.dispatchEvent(new Event('input', { bubbles: true }));
      sel.dispatchEvent(new Event('change', { bubbles: true }));
    }
  }

  function close(refocus) {
    var o = openState;
    if (!o) return;
    openState = null;
    document.removeEventListener('mousedown', o.onDoc, true);
    window.removeEventListener('resize', o.onMove);
    window.removeEventListener('scroll', o.onMove, true);
    o.panel.remove();
    var st = state.get(o.sel);
    if (st) {
      st.trigger.setAttribute('aria-expanded', 'false');
      st.wrap.classList.remove('fe-ss-open');
      if (refocus) st.trigger.focus();
    }
  }

  function scan(root) {
    if (!root || !root.querySelectorAll) return;
    if (root.matches && root.matches('select[data-seam-select]')) enhance(root);
    Array.prototype.forEach.call(root.querySelectorAll('select[data-seam-select]'), enhance);
  }
  function start() {
    scan(document);
    if (!window.MutationObserver) return;
    new MutationObserver(function (muts) {
      muts.forEach(function (m) {
        Array.prototype.forEach.call(m.addedNodes, function (n) { if (n.nodeType === 1) scan(n); });
      });
    }).observe(document.documentElement, { childList: true, subtree: true });
  }
  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', start);
  else start();
  return { enhance: enhance, scan: scan, refresh: refresh, close: function () { close(false); } };
}());

// =============================================================================
// SEAM REORDER -- moving the rows of a repeatable whose container carries
// data-reorder (x-seam-reorder). Each row has a .fe-drag handle:
//   * pointer: press the handle and drag; the row follows the pointer and swaps
//     places with its neighbours as its centre passes theirs (the page scrolls
//     near its top and bottom edge); Escape puts it back;
//   * keyboard: with the handle focused, ArrowUp/ArrowDown move the row one
//     place, Home/End to the first/last.
// Flat repeatables post parallel arrays in DOM order, so moving the rows is
// enough. Indexed ones carry a hidden name[i][_pos] per row; sync() writes the
// row's position into it after every move and again on submit, which is what
// the server orders by. Nothing here runs for a repeatable without data-reorder.
// =============================================================================
window.SeamReorder = (function () {
  'use strict';
  var EDGE = 40, STEP = 14;
  var drag = null;

  // A moved row is re-inserted, which SeamRowFx would take for a new row and grow in
  // again; the flag (cleared once the observers have run) tells it to leave the row be.
  function keep(row) {
    row.setAttribute('data-fe-moved', '1');
    setTimeout(function () { row.removeAttribute('data-fe-moved'); }, 0);
  }

  function rowsOf(c) {
    return Array.prototype.filter.call(c.children, function (n) { return n.classList && n.classList.contains('repeatable-row'); });
  }
  function containerOf(handle) {
    var row = handle.closest ? handle.closest('.repeatable-row') : null;
    var c = row && row.parentNode;
    return c && c.hasAttribute && c.hasAttribute('data-reorder') ? { row: row, c: c } : null;
  }
  function sync(c) {
    rowsOf(c).forEach(function (row, i) {
      Array.prototype.forEach.call(row.children, function (n) {
        if (n.tagName === 'INPUT' && n.type === 'hidden' && /\[_pos\]$/.test(n.name)) n.value = String(i);
      });
    });
  }
  function syncAll(root) {
    if (!root || !root.querySelectorAll) return;
    if (root.hasAttribute && root.hasAttribute('data-reorder')) sync(root);
    Array.prototype.forEach.call(root.querySelectorAll('[data-reorder]'), sync);
  }
  function done(c, row) {
    sync(c);
    c.dispatchEvent(new CustomEvent('seam:reorder', { bubbles: true, detail: { row: row, index: rowsOf(c).indexOf(row) } }));
  }

  function onDown(ev) {
    var h = ev.target && ev.target.closest ? ev.target.closest('.fe-drag') : null;
    if (!h || (ev.button != null && ev.button !== 0)) return;
    var ctx = containerOf(h);
    if (!ctx) return;
    ev.preventDefault();
    drag = {
      handle: h, row: ctx.row, c: ctx.c, pointer: ev.pointerId,
      startY: ev.clientY, startScroll: window.pageYOffset || 0, comp: 0, y: ev.clientY,
      origNext: ctx.row.nextElementSibling, moved: false, raf: 0
    };
    if (h.setPointerCapture && ev.pointerId != null) { try { h.setPointerCapture(ev.pointerId); } catch (e) { /* synthetic */ } }
    h.focus();
    document.addEventListener('pointermove', onMove);
    document.addEventListener('pointerup', onUp);
    document.addEventListener('pointercancel', onCancel);
    document.addEventListener('keydown', onEsc, true);
  }
  function apply() {
    var d = drag;
    if (!d) return;
    var dy = (d.y - d.startY) + ((window.pageYOffset || 0) - d.startScroll) + d.comp;
    d.row.style.transform = 'translateY(' + dy + 'px)';
  }
  function swapIfNeeded() {
    var d = drag, moved = true;
    while (moved) {
      moved = false;
      var r = d.row.getBoundingClientRect(), centre = r.top + r.height / 2;
      var prev = d.row.previousElementSibling, next = d.row.nextElementSibling;
      var target = null, before = true;
      if (prev && prev.classList.contains('repeatable-row')) {
        var pr = prev.getBoundingClientRect();
        if (centre < pr.top + pr.height / 2) { target = prev; before = true; }
      }
      if (!target && next && next.classList.contains('repeatable-row')) {
        var nr = next.getBoundingClientRect();
        if (centre > nr.top + nr.height / 2) { target = next; before = false; }
      }
      if (!target) break;
      var topBefore = r.top;
      keep(d.row);
      d.c.insertBefore(d.row, before ? target : target.nextElementSibling);
      var topAfter = d.row.getBoundingClientRect().top;
      d.comp += topBefore - topAfter;
      apply();
      d.moved = true;
      moved = true;
    }
  }
  function onMove(ev) {
    var d = drag;
    if (!d || (d.pointer != null && ev.pointerId !== d.pointer)) return;
    d.y = ev.clientY;
    d.row.classList.add('fe-row-dragging');
    apply();
    swapIfNeeded();
    if (!d.raf) d.raf = window.requestAnimationFrame(autoscroll);
  }
  function autoscroll() {
    var d = drag;
    if (!d) return;
    d.raf = 0;
    var vh = window.innerHeight, dy = 0;
    if (d.y < EDGE) dy = -STEP; else if (d.y > vh - EDGE) dy = STEP;
    if (dy) {
      window.scrollBy(0, dy);
      apply();
      swapIfNeeded();
      d.raf = window.requestAnimationFrame(autoscroll);
    }
  }
  function finish(cancel) {
    var d = drag;
    if (!d) return;
    drag = null;
    if (d.raf) window.cancelAnimationFrame(d.raf);
    document.removeEventListener('pointermove', onMove);
    document.removeEventListener('pointerup', onUp);
    document.removeEventListener('pointercancel', onCancel);
    document.removeEventListener('keydown', onEsc, true);
    d.row.classList.remove('fe-row-dragging');
    d.row.style.transform = '';
    if (cancel && d.moved) { keep(d.row); d.c.insertBefore(d.row, d.origNext); }
    if (d.moved) done(d.c, d.row); else sync(d.c);
  }
  function onUp(ev) { if (drag && (drag.pointer == null || ev.pointerId === drag.pointer)) finish(false); }
  function onCancel() { finish(true); }
  function onEsc(ev) { if (ev.key === 'Escape') { ev.preventDefault(); ev.stopPropagation(); finish(true); } }

  function say(c, row) {
    var rows = rowsOf(c), n = rows.indexOf(row) + 1;
    var live = document.getElementById('fe-reorder-live');
    if (!live) {
      live = document.createElement('div');
      live.id = 'fe-reorder-live';
      live.className = 'fe-sr-only';
      live.setAttribute('aria-live', 'polite');
      document.body.appendChild(live);
    }
    live.textContent = n + ' / ' + rows.length;
  }
  function onKey(ev) {
    var h = ev.target && ev.target.closest ? ev.target.closest('.fe-drag') : null;
    if (!h || drag) return;
    var k = ev.key;
    if (k !== 'ArrowUp' && k !== 'ArrowDown' && k !== 'Home' && k !== 'End') return;
    var ctx = containerOf(h);
    if (!ctx) return;
    ev.preventDefault();
    var rows = rowsOf(ctx.c), i = rows.indexOf(ctx.row), to = i;
    if (k === 'ArrowUp') to = Math.max(0, i - 1);
    else if (k === 'ArrowDown') to = Math.min(rows.length - 1, i + 1);
    else if (k === 'Home') to = 0;
    else to = rows.length - 1;
    if (to === i) return;
    keep(ctx.row);
    if (to < i) ctx.c.insertBefore(ctx.row, rows[to]);
    else ctx.c.insertBefore(ctx.row, rows[to].nextElementSibling);
    h.focus();
    done(ctx.c, ctx.row);
    say(ctx.c, ctx.row);
  }

  document.addEventListener('pointerdown', onDown);
  document.addEventListener('keydown', onKey);
  // submit-time sync: whatever happened to the rows, the posted positions match the DOM
  document.addEventListener('submit', function (ev) { syncAll(ev.target); }, true);
  return { sync: sync, syncAll: syncAll };
}());

// =============================================================================
// SEAM ROW FX -- fast grow and shrink of repeatable rows, only inside an
// element marked data-fe-fx (the Demo page). One place for every repeatable,
// flat or indexed, because it does not touch their handlers:
//   grow   a MutationObserver sees a .repeatable-row appended after the page
//          was drawn and gives it .fe-row-in for a moment (CSS animation);
//   shrink a capture-phase click on .fe-remove holds the click back, plays
//          .fe-row-out, then clicks again so the repeatable's own handler runs
//          on a row that is back to normal (the flat handler clones the row).
// prefers-reduced-motion switches both off. The duration matches app.css.
// =============================================================================
window.SeamRowFx = (function () {
  'use strict';
  var MS = 150;
  var busy = new WeakSet();     // rows whose shrink is playing
  var passing = new WeakSet();  // rows whose second click must go through
  function reduced() {
    return !!(window.matchMedia && window.matchMedia('(prefers-reduced-motion: reduce)').matches);
  }
  // body[data-fe-fxmode]: "scoped" (default: only inside [data-fe-fx]), "all" or "off"
  function inScope(el) {
    var m = (document.body && document.body.getAttribute('data-fe-fxmode')) || 'scoped';
    if (m === 'off') return false;
    if (m === 'all') return !!el;
    return !!(el && el.closest && el.closest('[data-fe-fx]'));
  }

  if (window.MutationObserver) {
    new MutationObserver(function (muts) {
      if (reduced()) return;
      muts.forEach(function (m) {
        Array.prototype.forEach.call(m.addedNodes, function (n) {
          if (n.nodeType !== 1 || !n.classList.contains('repeatable-row') || !inScope(n) || n.hasAttribute('data-fe-moved')) return;
          n.classList.add('fe-row-in');
          setTimeout(function () { n.classList.remove('fe-row-in'); }, MS + 30);
        });
      });
    }).observe(document.documentElement, { childList: true, subtree: true });
  }

  document.addEventListener('click', function (ev) {
    var btn = ev.target && ev.target.closest ? ev.target.closest('.fe-remove') : null;
    if (!btn || !inScope(btn) || reduced()) return;
    var row = btn.closest('.repeatable-row');
    if (!row) return;
    if (passing.has(row)) { passing.delete(row); return; }  // second pass: the real handler runs
    ev.stopPropagation();
    ev.preventDefault();
    if (busy.has(row)) return;
    busy.add(row);
    row.classList.add('fe-row-out');
    setTimeout(function () {
      busy.delete(row);
      row.classList.remove('fe-row-out');
      passing.add(row);
      btn.click();
    }, MS);
  }, true);
  return {};
}());

// =============================================================================
// SEAM PICKER (any-picker widget, internal/formengine/widgets.go) -- one shared
// initialiser for every [data-seam-picker] on the page, including rows added
// later (SeamRepeatable.add calls init(row)). Nothing is looked up by a
// per-widget id: each picker gets generated ids at init, so the ids a
// repeatable row repeats cannot collide. Labels are written with .value /
// .textContent only, never as HTML.
// =============================================================================
// =============================================================================
// SEAM HIGHLIGHT -- safe match highlighting for picker result items. Builds the
// text with DOM nodes (a <mark class="seam-hl"> around each match), never with
// innerHTML, so a record label can never inject markup. Case- and accent-
// insensitive, like the server-side user search. How the match looks is CSS
// (autocomplete-seam.css and app.css): the primary colour by default, or a
// translucent marker with data-hl="background" on the search box.
// =============================================================================
window.SeamHighlight = (function () {
  'use strict';
  function fold(s) {
    s = String(s);
    return (s.normalize ? s.normalize('NFD').replace(/[\u0300-\u036f]/g, '') : s).toLowerCase();
  }
  function into(el, text, query) {
    el.textContent = '';
    text = String(text == null ? '' : text);
    var q = fold(String(query == null ? '' : query).trim());
    var hay = fold(text);
    // An exotic character whose folded form changes the length would shift the
    // match offsets: show the plain text rather than a wrong highlight.
    if (!q || hay.length !== text.length) { el.appendChild(document.createTextNode(text)); return; }
    var pos = 0, idx;
    while ((idx = hay.indexOf(q, pos)) >= 0) {
      if (idx > pos) el.appendChild(document.createTextNode(text.slice(pos, idx)));
      var m = document.createElement('mark');
      m.className = 'seam-hl';
      m.textContent = text.slice(idx, idx + q.length);
      el.appendChild(m);
      pos = idx + q.length;
    }
    if (pos < text.length) el.appendChild(document.createTextNode(text.slice(pos)));
  }
  return { into: into };
}());

// =============================================================================
// SEAM LIST -- row click for navigation lists. A row that contains a key link
// (a[data-row-link], rendered by ListCellLink) opens that link when the row is
// clicked anywhere that is not itself interactive: links, buttons, inputs,
// selects, labels and anything marked data-no-row-click keep their own
// behaviour, and a click that ends a text selection is left alone. Ctrl/Cmd/
// Shift-click opens the link in a new tab like a link would. The key link stays
// the keyboard and screen-reader target; the row click is a pointer convenience.
// =============================================================================
window.SeamList = (function () {
  'use strict';
  var INTERACTIVE = 'a,button,input,select,textarea,label,summary,[data-no-row-click]';
  document.addEventListener('click', function (ev) {
    if (ev.defaultPrevented || ev.button !== 0) return;
    var t = ev.target;
    if (!t || !t.closest) return;
    var row = t.closest('tr');
    if (!row) return;
    var link = row.querySelector('a[data-row-link]');
    if (!link) return;
    var inner = t.closest(INTERACTIVE);
    if (inner && row.contains(inner)) return;
    var sel = window.getSelection ? String(window.getSelection()) : '';
    if (sel.length > 0) return;
    if (ev.ctrlKey || ev.metaKey || ev.shiftKey) { window.open(link.href, '_blank'); return; }
    link.click();
  });
  return {};
}());

// =============================================================================
// SEAM WIDGETS -- the shared initialiser of the form engine widgets that used to
// carry their own inline <script>: the input mask (text and tel), barcode, gps
// and signature. Their markup is now inert and carries data attributes:
//   input[data-seam-mask]            a masked text/tel input
//   [data-seam-widget="barcode"]     input + <svg>, data-format = the symbology
//   [data-seam-widget="gps"]         input + [data-gps-locate] + [data-gps-map]
//   [data-seam-widget="signature"]   hidden input + <canvas>
//   [data-seam-widget="query-picker"] hidden input + search box; the dropdown is built here
// Everything is found relative to the widget's own root, never by a per-field
// id, so the ids a repeatable row repeats cannot collide. Inline scripts do not
// run in rows a repeatable adds from its <template> (innerHTML), which is why
// the old per-widget scripts only ever worked in the first, server-rendered
// row; SeamRepeatable.add now calls SeamWidgets.init(row) on the new row, the
// way it already calls SeamPicker.init. init is idempotent (data-sw-ready), and
// also runs at DOMContentLoaded and after every htmx swap.
// A library (Leaflet, JsBarcode) that the markup's own <script src> did not
// load -- a template-only repeatable never emits one into the page -- is loaded
// here once, on first use.
// =============================================================================
window.SeamWidgets = (function () {
  'use strict';
  var pending = {};

  // need resolves with window[name], loading src once if it is not there yet. It never waits on
  // a <script> element already in the page: htmx replaces the script elements of a swapped-in
  // fragment with fresh ones, so a listener on the original would never fire. It polls briefly (a
  // <script src> that came with the page or fragment may be about to run), then loads src itself.
  function need(name, src) {
    if (window[name]) return Promise.resolve(window[name]);
    if (!pending[src]) {
      pending[src] = new Promise(function (resolve, reject) {
        var tries = 0;
        (function wait() {
          if (window[name]) return resolve(window[name]);
          if (++tries < 6) return setTimeout(wait, 50);
          var s = document.createElement('script');
          s.src = src;
          s.addEventListener('load', function () { resolve(window[name]); });
          s.addEventListener('error', function () { delete pending[src]; reject(new Error('cannot load ' + src)); });
          document.head.appendChild(s);
        }());
      });
    }
    return pending[src];
  }

  function needCSS(href) {
    if (document.querySelector('link[href="' + href + '"]')) return;
    var l = document.createElement('link');
    l.rel = 'stylesheet';
    l.href = href;
    document.head.appendChild(l);
  }

  // ---- input mask: 9 = digit, A = upper-case letter, a = letter, * = alphanumeric
  function applyMask(mask, raw) {
    var out = '', ri = 0;
    for (var mi_ = 0; mi_ < mask.length && ri < raw.length; mi_++) {
      var m = mask[mi_], c = raw[ri];
      if (m === '9') {
        if (/[0-9]/.test(c)) { out += c; ri++; } else { ri++; mi_--; }
      } else if (m === 'A') {
        if (/[a-zA-Z]/.test(c)) { out += c.toUpperCase(); ri++; } else { ri++; mi_--; }
      } else if (m === 'a') {
        if (/[a-zA-Z]/.test(c)) { out += c; ri++; } else { ri++; mi_--; }
      } else if (m === '*') {
        if (/[a-zA-Z0-9]/.test(c)) { out += c; ri++; } else { ri++; mi_--; }
      } else {
        out += m;
        if (c === m) ri++;
      }
    }
    return out;
  }

  function initMask(el) {
    var mask = el.getAttribute('data-seam-mask');
    if (!mask) return;
    el.addEventListener('input', function () {
      var pos = el.selectionStart;
      var before = el.value.length;
      el.value = applyMask(mask, el.value.replace(/[^a-zA-Z0-9]/g, ''));
      var after = el.value.length;
      el.selectionStart = el.selectionEnd = Math.max(0, pos + (after - before));
    });
  }

  // ---- barcode: a live JsBarcode preview; the input turns red while the value
  // is not valid for the symbology (a normal, transient state while typing)
  function initBarcode(root) {
    var input = root.querySelector('input[type="text"]');
    var svg = root.querySelector('svg');
    var format = root.getAttribute('data-format') || 'EAN13';
    if (!input || !svg) return;
    // aria-invalid drives a CSS rule (app.css) that wins over the theme and focus styles
    function setInvalid(bad) {
      if (bad) input.setAttribute('aria-invalid', 'true'); else input.removeAttribute('aria-invalid');
    }
    function render() {
      if (!input.value) { svg.innerHTML = ''; setInvalid(false); return; }
      if (!window.JsBarcode) return;
      try {
        window.JsBarcode(svg, input.value, {
          format: format,
          width: 2,
          height: 60,
          displayValue: true,
          // black bars on white with a quiet zone: readable (and scannable) in any theme
          background: '#ffffff',
          lineColor: '#000000',
          margin: 10,
          // JsBarcode does not throw when a valid callback is given, so the previous drawing
          // would stay on screen under the red border: clear it
          valid: function (ok) { setInvalid(!ok); if (!ok) svg.innerHTML = ''; }
        });
      } catch (e) {
        svg.innerHTML = '';
      }
    }
    input.addEventListener('input', render);
    need('JsBarcode', '/static/vendor/jsbarcode-ean-upc@3.12.3.min.js').then(render).catch(function () {});
  }

  // ---- gps: a Leaflet map with one draggable marker, kept in step with the
  // "lat,lng" text input
  function parseLatLng(s) {
    var parts = (s || '').split(',');
    if (parts.length !== 2) return null;
    var lat = parseFloat(parts[0]), lng = parseFloat(parts[1]);
    if (isNaN(lat) || isNaN(lng)) return null;
    return [lat, lng];
  }

  // The pin is an inline SVG in a divIcon: no image file, no path to resolve (Leaflet's
  // own default marker builds its URL from a detected path and broke under /static/).
  var PIN_SVG = '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 30 42" width="30" height="42" aria-hidden="true"><path d="M15 1C7.3 1 1 7.3 1 15c0 10.5 14 26 14 26s14-15.5 14-26C29 7.3 22.7 1 15 1z" fill="#4f46e5" stroke="#312e81" stroke-width="1.5"/><circle cx="15" cy="15" r="5.5" fill="#fff"/></svg>';

  // The map sits in a dropdown panel under the input row (like the icon-picker's grid) and
  // is built the first time the panel opens, when its box has a real size.
  function initGPS(root) {
    var input = root.querySelector('input[type="text"]');
    var toggle = root.querySelector('[data-gps-toggle]');
    var panel = root.querySelector('[data-gps-panel]');
    var mapEl = root.querySelector('[data-gps-map]');
    var locate = root.querySelector('[data-gps-locate]');
    var done = root.querySelector('[data-gps-close]');
    var expand = root.querySelector('[data-gps-expand]');
    if (!input || !toggle || !panel || !mapEl || !locate) return;
    var api = null;
    // Two ways to open, like the icon-picker: a dropdown under the pin button, aligned to its
    // right edge (the default), or the full width of the field, in the page flow. The server sets
    // the starting mode (x-seam-panel); the expand button switches it.
    var wide = root.getAttribute('data-panel') === 'wide';
    function fit() {
      panel.classList.remove('fe-gps-panel--left');
      if (wide || panel.hidden) return;
      // right-aligned by default; if that would push the panel off the left edge of the screen
      // (a narrow cell near the left), anchor it to the left instead
      if (panel.getBoundingClientRect().left < 8) panel.classList.add('fe-gps-panel--left');
    }
    // Wide means the width of the CONTAINER, not of the field: in a repeatable's row the field is one
    // narrow cell, so while the panel is open and wide the cell takes the whole row (form layout:
    // every grid column; row layout: a line of its own).
    function spanRow(on) {
      var el = root.closest('.fe-cell') || root;
      if (el.parentElement && el.parentElement.classList.contains('repeatable-row')) el.classList.toggle('fe-gps-span', on);
    }
    function applyMode() {
      panel.classList.toggle('fe-gps-panel--wide', wide);
      spanRow(wide && !panel.hidden);
      if (expand) {
        expand.setAttribute('aria-pressed', wide ? 'true' : 'false');
        var lab = expand.getAttribute(wide ? 'data-label-collapse' : 'data-label-expand');
        if (lab) { expand.title = lab; expand.setAttribute('aria-label', lab); }
        if (expand.firstElementChild) expand.firstElementChild.textContent = wide ? 'close_fullscreen' : 'open_in_full';
      }
      fit();
      if (api) api.refresh();
    }

    function isOpen() { return !panel.hidden; }
    function close() { panel.hidden = true; toggle.setAttribute('aria-expanded', 'false'); spanRow(false); }
    function open() {
      panel.hidden = false;
      toggle.setAttribute('aria-expanded', 'true');
      applyMode();
      needCSS('/static/vendor/leaflet@1.9.4.css');
      need('L', '/static/vendor/leaflet@1.9.4.js').then(function (L) {
        if (!api) api = build(L);
        api.refresh();
      }).catch(function () {});
    }
    toggle.addEventListener('click', function () { if (isOpen()) close(); else open(); });
    if (done) done.addEventListener('click', close);
    if (expand) expand.addEventListener('click', function () { wide = !wide; applyMode(); });
    root.addEventListener('keydown', function (ev) {
      if (ev.key === 'Escape' && isOpen()) { close(); toggle.focus(); }
    });
    document.addEventListener('click', function (ev) { if (isOpen() && !root.contains(ev.target)) close(); });
    // typing coordinates by hand moves the marker once the map exists
    input.addEventListener('change', function () { if (api) api.fromInput(); });

    function build(L) {
      var pin = L.divIcon({ className: 'seam-pin', html: PIN_SVG, iconSize: [30, 42], iconAnchor: [15, 41] });
      var start = parseLatLng(input.value);
      var map = L.map(mapEl).setView(start || [0, 0], start ? 13 : 2);
      L.tileLayer('https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png', {
        attribution: '&copy; OpenStreetMap contributors'
      }).addTo(map);
      var marker = null;
      function place(lat, lng) {
        if (marker) { marker.setLatLng([lat, lng]); return; }
        marker = L.marker([lat, lng], { draggable: true, icon: pin }).addTo(map);
        marker.on('dragend', function () {
          var p = marker.getLatLng();
          setPoint(p.lat, p.lng, false);
        });
      }
      function setPoint(lat, lng, recenter) {
        input.value = lat.toFixed(6) + ',' + lng.toFixed(6);
        place(lat, lng);
        if (recenter) map.setView([lat, lng], 15);
      }
      map.on('click', function (ev) { setPoint(ev.latlng.lat, ev.latlng.lng, false); });
      locate.addEventListener('click', function () {
        if (!navigator.geolocation) return;
        navigator.geolocation.getCurrentPosition(function (pos) {
          setPoint(pos.coords.latitude, pos.coords.longitude, true);
        });
      });
      if (window.ResizeObserver) {
        new window.ResizeObserver(function () { map.invalidateSize(); }).observe(mapEl);
      }
      return {
        // on every open: re-measure, and show the stored point if there is one
        refresh: function () {
          map.invalidateSize();
          var p = parseLatLng(input.value);
          if (p) { place(p[0], p[1]); map.setView(p, Math.max(map.getZoom(), 13)); }
        },
        fromInput: function () {
          var p = parseLatLng(input.value);
          if (p) setPoint(p[0], p[1], true);
        }
      };
    }
  }

  // ---- signature: a canvas the user draws on; the stroke is serialised into
  // the hidden input as a data:image/png;base64 string
  function initSignature(root) {
    var hidden = root.querySelector('input[type="hidden"]');
    var canvas = root.querySelector('canvas');
    if (!hidden || !canvas) return;
    var ctx = canvas.getContext('2d');
    function blank() {
      ctx.fillStyle = '#fff';
      ctx.fillRect(0, 0, canvas.width, canvas.height);
    }
    blank();
    ctx.strokeStyle = '#1f2937';
    ctx.lineWidth = 2;
    ctx.lineJoin = 'round';
    ctx.lineCap = 'round';
    if (hidden.value) {
      var img = new Image();
      img.onload = function () { ctx.drawImage(img, 0, 0, canvas.width, canvas.height); };
      img.src = hidden.value;
    }
    var drawing = false;
    function pos(e) {
      var r = canvas.getBoundingClientRect();
      return { x: (e.clientX - r.left) * (canvas.width / r.width), y: (e.clientY - r.top) * (canvas.height / r.height) };
    }
    canvas.addEventListener('pointerdown', function (e) {
      drawing = true;
      var p = pos(e);
      ctx.beginPath();
      ctx.moveTo(p.x, p.y);
    });
    canvas.addEventListener('pointermove', function (e) {
      if (!drawing) return;
      var p = pos(e);
      ctx.lineTo(p.x, p.y);
      ctx.stroke();
    });
    window.addEventListener('pointerup', function () {
      if (!drawing) return;
      drawing = false;
      hidden.value = canvas.toDataURL('image/png');
    });
    root._swClear = function () {
      blank();
      hidden.value = '';
    };
  }

  // The Clear button: finds its own signature by DOM position, not by name.
  function clearSignature(btn) {
    var root = btn && btn.closest ? btn.closest('[data-seam-widget="signature"]') : null;
    if (root && typeof root._swClear === 'function') root._swClear();
  }

  // ---- query-picker (docs/QUERY_PICKER.md) ---------------------------------
  // Choose one record from the rows of the field's own OQL query. The markup
  // carries a sealed token (data-qp-token) and the column headings and types
  // (data-qp-cols); the query itself never leaves the server. Typing asks
  // GET /picker/query for the first rows, after a pause, from a minimum number
  // of characters, dropping any answer a newer request has overtaken.
  // The panel is built with DOM calls only: result text never becomes markup.
  var qpSeq = 0;
  var QP_BADGE = {
    success: 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-300',
    warning: 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900 dark:text-yellow-300',
    danger: 'bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-300',
    info: 'bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-300',
    neutral: 'bg-gray-100 text-gray-800 dark:bg-gray-700 dark:text-gray-300'
  };

  function initQueryPicker(root) {
    var hidden = root.querySelector('input[type="hidden"]');
    var search = root.querySelector('input[type="text"]');
    if (!hidden || !search) return;
    var token = root.dataset.qpToken || '';
    var cols = [];
    try { cols = JSON.parse(root.dataset.qpCols || '[]'); } catch (e) { cols = []; }
    var min = parseInt(root.dataset.qpMin || '2', 10) || 2;
    var detailLine = root.querySelector('[data-qp-detail-line]');
    var browseBtn = root.querySelector('[data-qp-browse]');
    qpSeq++;
    root.dataset.qpid = 'seam-qp-' + qpSeq;

    var panel = null, items = [], rows = [], active = -1;
    var timer = null, ctl = null, asked = 0, suppress = false;

    function msg(name, n) { return String(root.dataset[name] || '').replace('%d', String(n)); }
    function endpoint(extra) { return '/picker/query?s=' + encodeURIComponent(token) + extra; }
    function setLabel(v) { suppress = true; search.value = v; suppress = false; }
    function hlMode() { return search.getAttribute('data-hl') || 'color'; }

    function syncValidity() {
      if (!search.required) return;
      search.setCustomValidity(!hidden.value && search.value ? (root.dataset.tSelect || 'Please choose an item from the list') : '');
    }

    function isNum(col) { return col.k === 'number' || col.k === 'money'; }

    // cellNode draws one cell: a badge, an icon, a tick, or text with the match marked.
    function cellNode(cell, col, query) {
      var el = document.createElement('span');
      var text = cell && cell.v != null ? String(cell.v) : '';
      if (col.k === 'badge') {
        if (text) {
          el.className = 'inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ' + (QP_BADGE[cell.b] || QP_BADGE.neutral);
          el.textContent = text;
        }
      } else if (col.k === 'icon') {
        if (text) { el.className = 'material-icons fe-qp-glyph'; el.textContent = text; }
      } else if (col.k === 'bool') {
        el.className = 'material-icons fe-qp-glyph';
        el.textContent = text === 'true' ? 'check' : 'remove';
      } else if (query && col.k === 'text' && hlMode() !== 'none' && window.SeamHighlight) {
        window.SeamHighlight.into(el, text, query);
      } else {
        el.textContent = text;
      }
      return el;
    }

    function showDetail(row) {
      if (!detailLine) return;
      detailLine.textContent = '';
      var shown = 0;
      if (row && root.dataset.qpDetail === '1') {
        row.cells.forEach(function (c, i) {
          var col = cols[i] || {};
          if (!c || c.v === '' || c.v == null || (col.k !== 'bool' && row.label.indexOf(c.v) >= 0)) return;
          if (col.k === 'bool' && c.v !== 'true') return;
          if (shown) detailLine.appendChild(document.createTextNode(' · '));
          detailLine.appendChild(cellNode(c, col, ''));
          shown++;
        });
      }
      detailLine.hidden = shown === 0;
    }

    function choose(row) {
      hidden.value = row.value;
      setLabel(row.label);
      showDetail(row);
      close();
      syncValidity();
      // Writing .value from script fires no event: tell listeners the selection changed.
      hidden.dispatchEvent(new Event('change', { bubbles: true }));
    }

    function layout() {
      var want = root.dataset.qpLayout || 'auto';
      if (want === 'table' || want === 'rows') return want;
      return root.clientWidth >= Math.max(420, cols.length * 120) ? 'table' : 'rows';
    }

    function place() {
      if (!panel) return;
      if (!document.body.contains(root)) { close(); return; }
      var r = search.getBoundingClientRect();
      var vw = window.innerWidth || document.documentElement.clientWidth;
      var vh = window.innerHeight || document.documentElement.clientHeight;
      panel.style.minWidth = Math.min(r.width, vw - 16) + 'px';
      panel.style.maxWidth = (vw - 16) + 'px';
      var w = panel.offsetWidth;
      panel.style.left = Math.max(8, Math.min(r.left, vw - w - 8)) + 'px';
      var below = vh - r.bottom - 8, above = r.top - 8;
      var want = Math.min(panel.scrollHeight || 240, 420);
      if (below < want && above > below) {
        panel.style.top = ''; panel.style.bottom = (vh - r.top + 4) + 'px';
        panel.style.maxHeight = Math.max(120, above) + 'px';
      } else {
        panel.style.bottom = ''; panel.style.top = (r.bottom + 4) + 'px';
        panel.style.maxHeight = Math.max(120, below) + 'px';
      }
    }

    function close() {
      if (timer) { clearTimeout(timer); timer = null; }
      if (ctl) { ctl.abort(); ctl = null; }
      asked++; // an answer still on its way is no longer wanted
      if (!panel) return;
      panel.remove();
      panel = null; items = []; rows = []; active = -1;
      search.setAttribute('aria-expanded', 'false');
      search.removeAttribute('aria-activedescendant');
      window.removeEventListener('resize', place);
      window.removeEventListener('scroll', place, true);
    }

    function open() {
      if (panel) { panel.textContent = ''; return; }
      panel = document.createElement('div');
      panel.className = 'fe-qp-panel';
      panel.id = root.dataset.qpid + '-panel';
      panel.setAttribute('role', 'listbox');
      panel.setAttribute('data-hl', hlMode());
      // keep the focus in the box while the pointer works in the panel
      panel.addEventListener('mousedown', function (ev) { ev.preventDefault(); });
      document.body.appendChild(panel);
      search.setAttribute('aria-expanded', 'true');
      search.setAttribute('aria-controls', panel.id);
      window.addEventListener('resize', place);
      window.addEventListener('scroll', place, true);
    }

    function setActive(i) {
      if (!items.length) { active = -1; return; }
      if (i < 0) i = items.length - 1;
      if (i >= items.length) i = 0;
      if (active >= 0 && items[active]) {
        items[active].classList.remove('fe-qp-active');
        items[active].setAttribute('aria-selected', 'false');
      }
      active = i;
      items[i].classList.add('fe-qp-active');
      items[i].setAttribute('aria-selected', 'true');
      search.setAttribute('aria-activedescendant', items[i].id);
      if (items[i].scrollIntoView) items[i].scrollIntoView({ block: 'nearest' });
    }

    function note(text) {
      open();
      var n = document.createElement('div');
      n.className = 'fe-ss-none';
      n.textContent = text;
      panel.appendChild(n);
      items = []; rows = []; active = -1;
      place();
    }

    function wire(el, i) {
      el.id = root.dataset.qpid + '-opt-' + i;
      el.setAttribute('role', 'option');
      el.setAttribute('aria-selected', 'false');
      el.addEventListener('click', function () { choose(rows[i]); });
      el.addEventListener('mousemove', function () { if (active !== i) setActive(i); });
      items.push(el);
    }

    function render(res, query) {
      var list = res && Array.isArray(res.rows) ? res.rows : [];
      if (!list.length) { note(root.dataset.tNone || 'No results'); return; }
      open();
      rows = list; items = []; active = -1;
      var kind = layout();
      if (kind === 'table') {
        var table = document.createElement('table');
        table.className = 'fe-qp-table';
        var thead = document.createElement('thead'), htr = document.createElement('tr');
        cols.forEach(function (col) {
          var th = document.createElement('th');
          th.textContent = col.t || '';
          if (isNum(col)) th.className = 'fe-qp-num';
          htr.appendChild(th);
        });
        thead.appendChild(htr); table.appendChild(thead);
        var tbody = document.createElement('tbody');
        list.forEach(function (row, i) {
          var tr = document.createElement('tr');
          tr.className = 'fe-qp-row';
          cols.forEach(function (col, j) {
            var td = document.createElement('td');
            var cls = [];
            if (isNum(col)) cls.push('fe-qp-num');
            if (col.key) cls.push('fe-qp-key');
            if (col.muted || col.k === 'date' || col.k === 'datetime') cls.push('fe-qp-muted');
            td.className = cls.join(' ');
            td.appendChild(cellNode(row.cells[j], col, query));
            tr.appendChild(td);
          });
          wire(tr, i);
          tbody.appendChild(tr);
        });
        table.appendChild(tbody);
        var scroller = document.createElement('div');
        scroller.className = 'fe-qp-scroll';
        scroller.appendChild(table);
        panel.appendChild(scroller);
      } else {
        var ul = document.createElement('ul');
        ul.className = 'fe-qp-list';
        var keyAt = 0;
        cols.forEach(function (col, j) { if (col.key) keyAt = j; });
        list.forEach(function (row, i) {
          var li = document.createElement('li');
          li.className = 'fe-qp-item';
          var main = document.createElement('div'); main.className = 'fe-qp-item-main';
          var title = document.createElement('div'); title.className = 'fe-qp-item-title fe-qp-key';
          title.appendChild(cellNode(row.cells[keyAt], cols[keyAt] || {}, query));
          var sub = document.createElement('div'); sub.className = 'fe-qp-item-sub';
          var side = document.createElement('div'); side.className = 'fe-qp-item-side';
          cols.forEach(function (col, j) {
            var c = row.cells[j];
            if (j === keyAt || !c || c.v === '' || c.v == null) return;
            if (col.k === 'badge' || col.k === 'number' || col.k === 'bool' || col.k === 'icon') {
              var wrap = document.createElement('div');
              wrap.appendChild(cellNode(c, col, ''));
              side.appendChild(wrap);
            } else {
              if (sub.childNodes.length) sub.appendChild(document.createTextNode(' · '));
              sub.appendChild(cellNode(c, col, query));
            }
          });
          main.appendChild(title);
          if (sub.childNodes.length) main.appendChild(sub);
          li.appendChild(main);
          if (side.childNodes.length) li.appendChild(side);
          wire(li, i);
          ul.appendChild(li);
        });
        panel.appendChild(ul);
      }
      var foot = document.createElement('div');
      foot.className = 'fe-qp-foot';
      var left = document.createElement('span');
      if (res.more) left.textContent = msg(browseBtn ? 'tMoreBrowse' : 'tMore', list.length);
      foot.appendChild(left);
      if (kind === 'table') {
        var right = document.createElement('span');
        right.className = 'fe-qp-keys';
        right.textContent = root.dataset.tKeys || '';
        foot.appendChild(right);
      }
      if (res.more || kind === 'table') panel.appendChild(foot);
      place();
      setActive(0);
    }

    function run() {
      timer = null;
      var q = search.value.trim();
      if (q.length < min) { close(); return; }
      if (ctl) ctl.abort();
      ctl = window.AbortController ? new AbortController() : null;
      var mine = ++asked;
      fetch(endpoint('&q=' + encodeURIComponent(q)), ctl ? { signal: ctl.signal } : undefined)
        .then(function (r) { if (!r.ok) throw new Error('search failed'); return r.json(); })
        .then(function (res) { if (mine === asked && document.activeElement === search) render(res, q); })
        .catch(function (err) {
          if (mine !== asked || (err && err.name === 'AbortError')) return;
          note(root.dataset.tFailed || 'The search could not be completed');
        });
    }

    search.addEventListener('input', function () {
      if (suppress) return;
      var had = hidden.value !== '';
      hidden.value = '';
      showDetail(null);
      syncValidity();
      if (had) hidden.dispatchEvent(new Event('change', { bubbles: true }));
      if (timer) clearTimeout(timer);
      if (search.value.trim().length < min) { close(); return; }
      timer = setTimeout(run, 250);
    });

    search.addEventListener('keydown', function (ev) {
      if (ev.key === 'ArrowDown' || ev.key === 'ArrowUp') {
        if (!panel || !items.length) {
          if (!hidden.value && search.value.trim().length >= min && !timer) run();
          return;
        }
        ev.preventDefault();
        setActive(active + (ev.key === 'ArrowDown' ? 1 : -1));
      } else if (ev.key === 'Enter') {
        if (panel && active >= 0 && rows[active]) { ev.preventDefault(); choose(rows[active]); }
      } else if (ev.key === 'Escape') {
        if (panel) { ev.preventDefault(); ev.stopPropagation(); close(); }
      } else if (ev.key === 'Tab') {
        close();
      }
    });
    search.addEventListener('blur', function () { setTimeout(function () { if (document.activeElement !== search) close(); }, 120); });

    // resolve asks the server for the row of one value and shows it; a value
    // the query no longer returns is kept and marked, never cleared.
    function resolve(value, thenChange) {
      fetch(endpoint('&value=' + encodeURIComponent(value)))
        .then(function (r) { if (!r.ok) throw new Error('lookup failed'); return r.json(); })
        .then(function (res) {
          if (hidden.value !== String(value) && !thenChange) return;
          var row = res && res.rows && res.rows[0];
          if (row) {
            if (thenChange) { choose(row); } else { setLabel(row.label); showDetail(row); syncValidity(); }
            return;
          }
          setLabel(String(value));
          if (detailLine) {
            detailLine.textContent = root.dataset.tUnavailable || 'no longer available';
            detailLine.hidden = false;
          }
        })
        .catch(function () {});
    }
    root._qpResolve = function (value) { resolve(value, true); };

    if (browseBtn) {
      browseBtn.addEventListener('click', function () {
        if (!window.SeamModal2) return;
        close();
        window.SeamModal2.open(browseBtn.dataset.title || browseBtn.textContent,
          '/picker/query/browse?s=' + encodeURIComponent(token) + '&target=' + encodeURIComponent(root.dataset.qpid),
          { maxWidth: '1180px' });
      });
    }

    if (hidden.value) resolve(hidden.value, false);
    syncValidity();
  }

  // The Browse dialog (the Selector) reports its choice here; the picker asks
  // the server for that row, so the box and the detail line show what a typed
  // choice would have shown.
  document.addEventListener('seam:selector-chosen', function (ev) {
    var d = ev.detail || {};
    if (!d.target || String(d.target).indexOf('seam-qp-') !== 0) return;
    var root = document.querySelector('[data-qpid="' + d.target + '"]');
    if (root && typeof root._qpResolve === 'function') root._qpResolve(d.id);
  });

  var KINDS = { gps: initGPS, barcode: initBarcode, signature: initSignature, 'query-picker': initQueryPicker };

  function ready(el, fn) {
    if (el.dataset.swReady === '1') return;
    el.dataset.swReady = '1';
    fn(el);
  }

  function init(scope) {
    scope = scope || document;
    function each(sel, fn) {
      if (scope.matches && scope.matches(sel)) ready(scope, fn);
      if (scope.querySelectorAll) scope.querySelectorAll(sel).forEach(function (el) { ready(el, fn); });
    }
    Object.keys(KINDS).forEach(function (k) { each('[data-seam-widget="' + k + '"]', KINDS[k]); });
    each('input[data-seam-mask]', initMask);
  }

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', function () { init(document); });
  } else {
    init(document);
  }
  document.addEventListener('htmx:load', function (ev) { init(ev.target || document); });

  return { init: init, clearSignature: clearSignature, applyMask: applyMask, need: need };
}());

window.SeamPicker = (function () {
  'use strict';
  var seq = 0;

  function ensureID(root) {
    if (!root.dataset.pkid) { seq++; root.dataset.pkid = 'seam-pk-' + seq; }
    return root.dataset.pkid;
  }

  function initOne(root) {
    if (!root || root.dataset.pkReady === '1') return;
    var hidden = root.querySelector('input[type="hidden"]');
    var search = root.querySelector('input[type="text"]');
    if (!hidden || !search) return;
    if (typeof window.autoComplete !== 'function') {
      // A form swapped in by htmx brings its <script src> with it, and htmx:load does not wait
      // for that script: wait for the library, then initialise (never give up silently).
      if (window.SeamWidgets) {
        window.SeamWidgets.need('autoComplete', '/static/vendor/autocomplete@10.2.10.min.js')
          .then(function () { initOne(root); }).catch(function () {});
      }
      return;
    }
    root.dataset.pkReady = '1';
    ensureID(root);
    search.id = 'seam-pks-' + root.dataset.pkid.slice('seam-pk-'.length);
    var entity = root.dataset.entity || '';
    // data-endpoint (icon-picker) replaces the entity search; its ids are the
    // label itself (an icon name), so no label lookup is needed.
    var endpoint = root.dataset.endpoint || '';
    var iconMode = root.dataset.preview === 'icon';
    // data-open-on-focus: list the first results as soon as the box gets focus (asset-picker)
    var openOnFocus = root.dataset.openOnFocus === '1' && !endpoint;
    var preview = root.querySelector('[data-icon-preview]');
    var suppress = false;
    function setLabel(v) { suppress = true; search.value = v; suppress = false; }
    function showIcon(v) { if (preview) preview.textContent = v; }
    // x-seam-highlight, set by the server on the search box: "color" (default),
    // "background" (styled in CSS) or "none".
    function hlMode() { return search.getAttribute('data-hl') || 'color'; }

    if (hidden.value && !endpoint) {
      fetch('/picker/search?entity=' + encodeURIComponent(entity) + '&id=' + encodeURIComponent(hidden.value))
        .then(function (r) { return r.json(); })
        .then(function (res) { if (res && res.length > 0) setLabel(res[0].label); })
        .catch(function () {});
    }

    function syncValidity() {
      if (search.hasAttribute('required') || search.required) {
        if (!hidden.value && search.value) {
          search.setCustomValidity(SeamText('select-item', 'Please select an item from the list'));
        } else {
          search.setCustomValidity('');
        }
      }
    }

    var ac = new window.autoComplete({
      selector: '#' + search.id,
      placeHolder: search.placeholder,
      debounce: 250,
      threshold: openOnFocus ? 0 : 1,
      data: {
        src: async function (query) {
          // While the icon grid is open it is the browser: no dropdown beside it.
          if (root.querySelector('[data-icon-grid]')) return [];
          var resp = await fetch(endpoint
            ? endpoint + '?q=' + encodeURIComponent(query)
            : '/picker/search?entity=' + encodeURIComponent(entity) + '&q=' + encodeURIComponent(query));
          if (!resp.ok) return [];
          return await resp.json();
        },
        keys: ['label']
      },
      resultsList: { maxResults: 20 },
      resultItem: {
        element: function (item, data) {
          var label = data.value.label;
          var textHost = item;
          item.textContent = '';
          if (iconMode) {
            var g = document.createElement('span');
            g.className = 'material-icons fe-result-icon';
            g.textContent = label;
            item.appendChild(g);
            textHost = document.createElement('span');
            item.appendChild(textHost);
          }
          if (hlMode() === 'none') textHost.appendChild(document.createTextNode(label));
          else SeamHighlight.into(textHost, label, search.value);
        },
        highlight: false // the library's own highlight would need innerHTML; SeamHighlight does it safely
      },
      events: {
        input: {
          selection: function (event) {
            var sel = event.detail.selection.value;
            hidden.value = sel.id;
            setLabel(sel.label);
            showIcon(sel.id);
            syncValidity();
            // Writing .value from script fires no event: tell listeners (e.g. the
            // issue form's template loader, hx-trigger "change from:#field-asset_id")
            // that the selection changed.
            hidden.dispatchEvent(new Event('change', { bubbles: true }));
          }
        }
      }
    });
    root._ac = ac;
    if (openOnFocus) search.addEventListener('focus', function () { if (!search.value) ac.start(); });

    // A stale hidden id must not survive the user typing something new.
    search.addEventListener('input', function () {
      if (suppress) return;
      var had = hidden.value !== '';
      hidden.value = '';
      showIcon('');
      syncValidity();
      if (had) hidden.dispatchEvent(new Event('change', { bubbles: true }));
    });
    syncValidity();
  }

  function init(scope) {
    scope = scope || document;
    if (scope.matches && scope.matches('[data-seam-picker]')) initOne(scope);
    if (scope.querySelectorAll) scope.querySelectorAll('[data-seam-picker]').forEach(initOne);
  }

  // Browse button: open the Selector dialog for this picker's selector key.
  function browse(btn) {
    var root = btn && btn.closest ? btn.closest('[data-seam-picker]') : null;
    if (!root || !root.dataset.selector || !window.SeamModal2) return;
    SeamModal2.open(btn.dataset.title || btn.textContent,
      '/selector/' + encodeURIComponent(root.dataset.selector) + '?target=' + encodeURIComponent(ensureID(root)),
      { maxWidth: '720px' });
  }

  // The Selector dialog reports its choice here; match it to the picker by id.
  document.addEventListener('seam:selector-chosen', function (ev) {
    var d = ev.detail || {};
    if (!d.target) return;
    var root = document.querySelector('[data-pkid="' + d.target + '"]');
    if (!root) return;
    var hidden = root.querySelector('input[type="hidden"]');
    var search = root.querySelector('input[type="text"]');
    if (!hidden || !search) return;
    hidden.value = d.id;
    search.value = d.label;
    if (search.setCustomValidity) search.setCustomValidity('');
  });

  document.addEventListener('DOMContentLoaded', function () { init(document); });
  document.addEventListener('htmx:load', function (ev) { init(ev.target || document); });

  // ---- icon-picker Browse grid -------------------------------------------
  // A scrollable grid of every icon, built lazily (never part of the inert
  // markup), filtered by what is typed in the search box, appended in chunks
  // as it is scrolled. Choosing an icon sets the hidden value, the box and the
  // glyph preview by property assignment, then closes the grid.
  var iconCache = null;
  function loadIcons() {
    if (iconCache) return Promise.resolve(iconCache);
    return fetch('/picker/icons?all=1').then(function (r) { return r.json(); })
      .then(function (list) { iconCache = Array.isArray(list) ? list : []; return iconCache; });
  }

  function toggleGrid(btn) {
    var root = btn && btn.closest ? btn.closest('[data-seam-picker]') : null;
    if (!root) return;
    var open = root.querySelector('[data-icon-grid]');
    if (open) { open.remove(); return; }
    // The dropdown list and the grid are never both visible: close the list.
    if (root._ac && typeof root._ac.close === 'function') root._ac.close();
    loadIcons().then(function (all) { openGrid(root, all); }).catch(function () {});
  }

  function openGrid(root, all) {
    if (root.querySelector('[data-icon-grid]')) return;
    var hidden = root.querySelector('input[type="hidden"]');
    var search = root.querySelector('input[type="text"]');
    var preview = root.querySelector('[data-icon-preview]');
    if (!hidden || !search) return;

    var panel = document.createElement('div');
    panel.setAttribute('data-icon-grid', '1');
    panel.className = 'fe-icon-grid';
    root.appendChild(panel);

    var CHUNK = 150, items = [], shown = 0;
    function fill() {
      var end = Math.min(shown + CHUNK, items.length);
      for (var i = shown; i < end; i++) {
        var name = items[i];
        var b = document.createElement('button');
        b.type = 'button';
        b.setAttribute('data-icon', name);
        b.title = name;
        b.setAttribute('aria-label', name);
        b.className = 'fe-icon-cell';
        b.setAttribute('aria-pressed', name === hidden.value ? 'true' : 'false');
        var g = document.createElement('span');
        g.className = 'material-icons';
        g.textContent = name;
        b.appendChild(g);
        panel.appendChild(b);
      }
      shown = end;
    }
    function rebuild() {
      if (!panel.isConnected) { search.removeEventListener('input', rebuild); return; }
      var q = (search.value || '').trim().toLowerCase();
      if (hidden.value && search.value === hidden.value) q = ''; // a chosen icon is not a filter
      items = q ? all.filter(function (n) { return n.indexOf(q) >= 0; }) : all;
      panel.textContent = '';
      shown = 0;
      fill();
      panel.scrollTop = 0;
    }
    panel.addEventListener('scroll', function () {
      if (panel.scrollTop + panel.clientHeight >= panel.scrollHeight - 60) fill();
    });
    panel.addEventListener('click', function (ev) {
      var b = ev.target.closest ? ev.target.closest('button[data-icon]') : null;
      if (!b) return;
      var name = b.getAttribute('data-icon');
      hidden.value = name;
      search.value = name;
      if (preview) preview.textContent = name;
      panel.remove();
    });
    search.addEventListener('input', rebuild);
    rebuild();
  }

  return { init: init, browse: browse, toggleGrid: toggleGrid };
}());

window.SeamModal = (function () {
  'use strict';

  var _backdrop = null;
  var _modal    = null;
  var _footer   = null;
  var _onKey    = null;
  var _dirty    = false; // a record was created while the modal was open: the page behind is stale

  function _dark() {
    return document.documentElement.classList.contains('dark');
  }

  function _css(el, props) { Object.assign(el.style, props); }

  function _build() {
    var dark = _dark();

    _backdrop = document.createElement('div');
    _css(_backdrop, {
      position: 'fixed', inset: '0',
      background: 'rgba(0,0,0,0)',
      backdropFilter: 'blur(3px)',
      zIndex: '9998',
      display: 'flex', alignItems: 'center', justifyContent: 'center',
      transition: 'background 220ms ease',
    });

    // ── Display-adaptive sizing ──────────────────────────────────────────
    // physW = actual physical pixel count, reliable across scaling modes.
    // screen.width reports CSS logical pixels; multiplying by dpr gives
    // the real pixel count regardless of OS scaling factor.
    var _physW = (window.screen ? window.screen.width : window.innerWidth)
                 * (window.devicePixelRatio || 1);
    var _vw    = window.innerWidth;

    // 4K or better (3840+ physical pixels): widen and heighten the modal
    // so it uses the large canvas rather than floating in a sea of margin.
    var _is4K    = _physW >= 3840;

    // Narrow viewport — 1366×768 budget laptops, 13" MacBook Air, etc.
    // Tighter body padding reclaims ~16px of usable width on each side.
    var _narrow  = _vw < 1420;

    var _maxW    = _is4K ? 1800 : 1290;             // modal width cap
    var _maxH    = _is4K ? 1100 : 1000;             // modal height cap
    var _hMargin = _is4K ? '96px' : '32px';         // horizontal margin on each side
    var _bodyPad = _narrow ? '16px 16px' : '20px 24px';

    _modal = document.createElement('div');
    _css(_modal, {
      background:    dark ? '#1e293b' : '#ffffff',
      borderRadius:  '12px',
      boxShadow:     '0 20px 60px rgba(0,0,0,0.35)',
      width:         _maxW + 'px',
      maxWidth:      'calc(100vw - ' + _hMargin + ')',
      height:        'calc(100vh - 48px)',
      minHeight:     '500px',
      maxHeight:     _maxH + 'px',
      display:       'flex',
      flexDirection: 'column',
      overflow:      'hidden',
      fontFamily:    'inherit',
      opacity:       '0',
      transform:     'scale(0.72)',
      transition:    'opacity 200ms ease, transform 200ms ease',
    });

    // Header
    var header = document.createElement('div');
    _css(header, {
      padding: '16px 20px',
      borderBottom: '1px solid ' + (dark ? '#334155' : '#e2e8f0'),
      display: 'flex', alignItems: 'center', justifyContent: 'space-between',
      flexShrink: '0',
    });

    var titleEl = document.createElement('div');
    titleEl.id = 'seam-modal-title';
    _css(titleEl, { fontWeight: '600', fontSize: '15px', color: dark ? '#f1f5f9' : '#0f172a' });

    var closeBtn = document.createElement('button');
    closeBtn.innerHTML = '<span class="material-icons" style="font-size:18px;line-height:1">close</span>';
    _css(closeBtn, {
      background: 'none', border: 'none', cursor: 'pointer',
      color: dark ? '#94a3b8' : '#64748b', padding: '2px', borderRadius: '4px',
      display: 'flex', alignItems: 'center',
    });
    closeBtn.addEventListener('click', function () { close(); });
    header.append(titleEl, closeBtn);

    // Body (scrollable)
    var body = document.createElement('div');
    body.id = 'seam-modal-body';
    _css(body, {
      flex: '1', overflowY: 'auto', padding: _bodyPad,
      color: dark ? '#e2e8f0' : '#1e293b',
    });
    var spinner = document.createElement('div');
    _css(spinner, { textAlign: 'center', padding: '40px 0', color: dark ? '#64748b' : '#94a3b8', fontSize: '13px' });
    spinner.textContent = 'Cargando…';
    body.appendChild(spinner);

    // Footer — fixed, never scrolls, receives action buttons from the partial
    _footer = document.createElement('div');
    _footer.id = 'seam-modal-footer';
    _css(_footer, {
      display:        'flex',
      alignItems:     'center',
      justifyContent: 'space-between',
      padding:        '10px 20px',
      borderTop:      '1px solid ' + (dark ? '#334155' : '#e2e8f0'),
      flexShrink:     '0',
      background:     dark ? '#1e293b' : '#ffffff',
      minHeight:      '52px',
    });

    _modal.append(header, body, _footer);
    _backdrop.appendChild(_modal);

    // Dismiss on backdrop click
    _backdrop.addEventListener('click', function (e) {
      if (e.target === _backdrop) close();
    });

    _onKey = function (e) { if (e.key === 'Escape') close(); };
    document.addEventListener('keydown', _onKey);

    document.body.appendChild(_backdrop);

    // Animate in (double rAF: first frame commits start state, second triggers transition)
    requestAnimationFrame(function () {
      requestAnimationFrame(function () {
        _backdrop.style.background = 'rgba(0,0,0,0.48)';
        _modal.style.opacity = '1';
        _modal.style.transform = 'scale(1)';
      });
    });
  }

  function open(title) {
    if (_backdrop) close(true);   // replace existing
    _build();
    setTitle(title || '');
  }

  function close(immediate) {
    if (!_backdrop) return;
    if (_onKey) { document.removeEventListener('keydown', _onKey); _onKey = null; }
    var b = _backdrop, m = _modal;
    _backdrop = null; _modal = null; _footer = null;
    if (immediate) {
      if (b.parentNode) b.parentNode.removeChild(b);
      return;
    }
    b.style.background = 'rgba(0,0,0,0)';
    m.style.opacity    = '0';
    m.style.transform  = 'scale(0.72)';
    setTimeout(function () { if (b.parentNode) b.parentNode.removeChild(b); }, 220);
    // Closing after a record was created (Create and add photos keeps the modal open on the new
    // record): refresh the page behind it so the list shows what was created.
    if (_dirty) { _dirty = false; setTimeout(function () { window.location.reload(); }, 240); }
  }

  function setTitle(t) {
    var el = document.getElementById('seam-modal-title');
    if (el) el.textContent = t;
  }

  // Re-create a <script> the partial brought, so that it runs. ALL its attributes are copied (src,
  // type="module", ...) as well as its text: copying only the text turned <script src=...> into an
  // empty inline script, so no library or module a form loaded this way ever loaded, and the photo
  // uploader of the Assets form (a module) never existed inside the modal.
  function rerunScript(s) {
    var n = document.createElement('script');
    for (var i = 0; i < s.attributes.length; i++) n.setAttribute(s.attributes[i].name, s.attributes[i].value);
    // scripts created this way are async by default: keep the page's own order for classic ones
    if (n.src && n.type !== 'module') n.async = false;
    n.textContent = s.textContent;
    s.parentNode.replaceChild(n, s);
  }

  // Called by HTMX after it loads the form partial into #seam-modal-body.
  // 1. Moves [data-modal-footer] from the body into the fixed footer slot so
  //    action buttons are always visible outside the scrollable area.
  // 2. Re-runs inline <script> blocks the partial injected (tab switcher).
  document.body.addEventListener('htmx:afterSwap', function (e) {
    if (e.detail && e.detail.target && e.detail.target.id === 'seam-modal-body') {
      // Extract footer content from the partial and place in fixed footer slot
      var footerSlot    = document.getElementById('seam-modal-footer');
      var footerContent = e.detail.target.querySelector('[data-modal-footer]');
      if (footerSlot && footerContent) {
        footerSlot.innerHTML = '';
        footerSlot.appendChild(footerContent);
      }
      // Re-run any inline <script> blocks the partial injected
      e.detail.target.querySelectorAll('script').forEach(rerunScript);
    }
    // SeamModal2: extract [data-modal-footer] into its fixed footer slot
    if (e.detail && e.detail.target && e.detail.target.id === 'seam-modal2-body') {
      var slot2 = document.getElementById('seam-modal2-footer');
      var fc2   = e.detail.target.querySelector('[data-modal-footer]');
      if (slot2 && fc2) { slot2.innerHTML = ''; slot2.appendChild(fc2); }
      e.detail.target.querySelectorAll('script').forEach(rerunScript);
    }
  });

  // When HTMX receives an HX-Redirect header, close the modal first
  document.body.addEventListener('htmx:beforeHistoryUpdate', function () { close(); });

  // A response can say a record was created (HX-Trigger: seam:record-created).
  document.body.addEventListener('seam:record-created', function () { _dirty = true; });

  return { open: open, close: close, setTitle: setTitle };
})();

// =============================================================================
// SEAM CONFIRM — OS-style confirmation messagebox
// Usage:
//   SeamConfirm.show(message, onConfirm)         — basic confirm
//   SeamConfirm.showDanger(message, onConfirm)   — destructive action (red button)
//   SeamConfirm.showForm(form, message)           — show then submit a form on OK
// =============================================================================

window.SeamConfirm = (function () {
  'use strict';

  var defaults = {};
  function setDefaults(d) {
    Object.assign(defaults, d || {});
  }

  function _dark() { return document.documentElement.classList.contains('dark'); }
  function _css(el, p) { Object.assign(el.style, p); }

  function _defaultCancel() {
    if (defaults.cancelLabel) return defaults.cancelLabel;
    if (window.SeamConfirm && window.SeamConfirm.defaults && window.SeamConfirm.defaults.cancelLabel) {
      return window.SeamConfirm.defaults.cancelLabel;
    }
    var html = document.documentElement;
    var fromAttr = html && (html.getAttribute('data-i18n-cancel') || html.getAttribute('data-cancel-label'));
    if (fromAttr) return fromAttr;
    if (document.body) {
      fromAttr = document.body.getAttribute('data-i18n-cancel') || document.body.getAttribute('data-cancel-label');
      if (fromAttr) return fromAttr;
    }
    return 'Cancel';
  }

  function show(message, onConfirm, opts) {
    opts = opts || {};
    var dark        = _dark();
    var danger      = !!opts.danger;
    var title       = opts.title       || (danger ? SeamText('confirm-delete', 'Confirm deletion') : SeamText('confirm', 'Confirm'));
    var okLabel     = opts.okLabel     || (danger ? SeamText('delete', 'Delete') : SeamText('ok', 'OK'));
    var cancelLabel = opts.cancelLabel || _defaultCancel();
    var dangerName  = opts.dangerName  || '';

    var backdrop = document.createElement('div');
    _css(backdrop, {
      position: 'fixed', inset: '0', background: 'rgba(0,0,0,0)',
      backdropFilter: 'blur(2px)', zIndex: '20000',
      display: 'flex', alignItems: 'center', justifyContent: 'center',
      transition: 'background 150ms ease',
    });

    var box = document.createElement('div');
    _css(box, {
      background: dark ? '#1e293b' : '#ffffff',
      borderRadius: '10px',
      boxShadow: '0 20px 60px rgba(0,0,0,0.4)',
      border: '1px solid ' + (dark ? '#334155' : '#e2e8f0'),
      width: '380px', maxWidth: 'calc(100vw - 48px)',
      padding: '24px',
      fontFamily: 'inherit',
      opacity: '0', transform: 'scale(0.85)',
      transition: 'opacity 150ms ease, transform 150ms ease',
    });

    var titleEl = document.createElement('div');
    _css(titleEl, {
      fontWeight: '600', fontSize: '15px', marginBottom: '10px',
      color: dark ? '#f1f5f9' : '#0f172a',
    });
    titleEl.textContent = title;

    var msgEl = document.createElement('div');
    _css(msgEl, {
      fontSize: '14px', lineHeight: '1.6',
      color: dark ? '#94a3b8' : '#64748b',
      marginBottom: '22px',
    });
    if (dangerName) {
      // "Are you sure you want to delete the entry for <b>Name</b>?", from a
      // sentence with {name} in it, so that each language puts the name where
      // it belongs.
      var esc = function (t) { return t.replace(/[<>&"]/g, function(c){return{'<':'&lt;','>':'&gt;','&':'&amp;','"':'&quot;'}[c];}); };
      var parts = SeamText('delete-named', 'Are you sure you want to delete the entry for {name}?').split('{name}');
      msgEl.innerHTML = esc(parts[0]) + '<strong style="color:' +
        (dark ? '#e2e8f0' : '#1e293b') + '">' + esc(dangerName) + '</strong>' + esc(parts.slice(1).join(''));
    } else {
      msgEl.textContent = message;
    }

    var btnRow = document.createElement('div');
    _css(btnRow, { display: 'flex', justifyContent: 'flex-end', gap: '8px' });

    // Cancel is ALWAYS the safe/default button (filled indigo, focused first)
    var cancelBtn = document.createElement('button');
    cancelBtn.textContent = cancelLabel;
    _css(cancelBtn, {
      padding: '7px 18px', borderRadius: '7px', fontSize: '13px',
      fontWeight: '600', cursor: 'pointer',
      border: '2px solid #4f46e5',
      background: '#4f46e5', color: '#ffffff',
    });

    // OK/Delete button — only bordered in red, fills on hover/focus
    var okBtn = document.createElement('button');
    okBtn.textContent = okLabel;
    var _okBase = {
      padding: '7px 18px', borderRadius: '7px', fontSize: '13px',
      fontWeight: '500', cursor: 'pointer',
      border: '2px solid ' + (danger ? '#dc2626' : '#4f46e5'),
      background: 'transparent',
      color: danger ? '#dc2626' : '#4f46e5',
      transition: 'background 120ms, color 120ms',
    };
    _css(okBtn, _okBase);
    okBtn.addEventListener('mouseenter', function () {
      okBtn.style.background = danger ? '#dc2626' : '#4f46e5';
      okBtn.style.color = '#ffffff';
    });
    okBtn.addEventListener('mouseleave', function () {
      okBtn.style.background = 'transparent';
      okBtn.style.color = danger ? '#dc2626' : '#4f46e5';
    });
    okBtn.addEventListener('focus', function () {
      okBtn.style.background = danger ? '#dc2626' : '#4f46e5';
      okBtn.style.color = '#ffffff';
    });
    okBtn.addEventListener('blur', function () {
      okBtn.style.background = 'transparent';
      okBtn.style.color = danger ? '#dc2626' : '#4f46e5';
    });

    function close() {
      backdrop.style.background = 'rgba(0,0,0,0)';
      box.style.opacity = '0'; box.style.transform = 'scale(0.85)';
      setTimeout(function () {
        if (backdrop.parentNode) backdrop.parentNode.removeChild(backdrop);
      }, 150);
    }

    cancelBtn.addEventListener('click', close);
    okBtn.addEventListener('click', function () { close(); if (onConfirm) onConfirm(); });
    backdrop.addEventListener('click', function (e) { if (e.target === backdrop) close(); });
    document.addEventListener('keydown', function esc(e) {
      if (e.key === 'Escape') { close(); document.removeEventListener('keydown', esc); }
    });

    // Cancel is default: Tab goes Cancel → Delete; Enter on Cancel triggers close
    cancelBtn.addEventListener('keydown', function (e) {
      if (e.key === 'Enter') { e.preventDefault(); close(); }
    });

    btnRow.append(cancelBtn, okBtn);
    box.append(titleEl, msgEl, btnRow);
    backdrop.appendChild(box);
    document.body.appendChild(backdrop);

    requestAnimationFrame(function () { requestAnimationFrame(function () {
      backdrop.style.background = 'rgba(0,0,0,0.3)';
      box.style.opacity = '1'; box.style.transform = 'scale(1)';
      cancelBtn.focus(); // Cancel is the default focused button
    }); });
  }

  function showDanger(message, onConfirm, opts) {
    show(message, onConfirm, Object.assign({ danger: true }, opts || {}));
  }

  function showForm(form, message, opts) {
    var mergedOpts = Object.assign({ danger: true }, opts || {});
    show(mergedOpts.dangerName ? 'Are you sure you want to delete' : message,
      function () { form.onsubmit = null; form.submit(); },
      mergedOpts);
  }

  return { show: show, showDanger: showDanger, showForm: showForm, setDefaults: setDefaults, defaults: defaults };
}());

// =============================================================================
// SEAM TABS — shared tab-switching behaviour
// Usage: SeamTabs.wire('.my-tab-panel', '.my-tab-btn')
// Extracted 2026-07-28: internal/formengine's tab renderer and the static
// Asset form (internal/ui/asset_detail.go) had independently hand-typed
// byte-for-byte identical tab-switching logic under different class-name
// prefixes (fe-tab-* vs asset-tab-*). One shared, parameterised function
// here replaces both — same behaviour, one place to fix if it ever needs
// to change, no drift risk between two copies.
// =============================================================================
window.SeamTabs = (function () {
  'use strict';
  function wire(panelSelector, btnSelector) {
    var panels = document.querySelectorAll(panelSelector);
    var btns = document.querySelectorAll(btnSelector);
    panels.forEach(function (p, i) { if (i !== 0) p.style.display = 'none'; });
    btns.forEach(function (btn) {
      btn.addEventListener('click', function () {
        var t = btn.getAttribute('data-tab');
        panels.forEach(function (p) { p.style.display = 'none'; });
        btns.forEach(function (b) {
          b.classList.remove('border-indigo-600', 'text-indigo-600');
          b.classList.add('border-transparent', 'text-gray-500');
        });
        var panel = document.getElementById(t);
        if (panel) panel.style.display = '';
        btn.classList.remove('border-transparent', 'text-gray-500');
        btn.classList.add('border-indigo-600', 'text-indigo-600');
      });
    });
  }
  return { wire: wire };
}());

// A required field inside a tab that is not showing cannot take focus, so the browser refuses to
// submit without a word ("An invalid form control ... is not focusable"): pressing Save or Create
// from another tab seemed to do nothing. When the first invalid field of a form is in a hidden tab,
// open that tab and ask the browser to show the field's message there.
document.addEventListener('invalid', function (ev) {
  var el = ev.target;
  var panel = el && el.closest ? el.closest('.fe-tab-panel') : null;
  if (!panel || !panel.id || getComputedStyle(panel).display !== 'none') return;
  var form = el.form;
  var first = form ? Array.prototype.find.call(form.elements, function (c) { return c.willValidate && c.validity && !c.validity.valid; }) : el;
  if (first !== el) return; // the browser reports the first invalid field only
  var btn = document.querySelector('.fe-tab-btn[data-tab="' + (window.CSS && CSS.escape ? CSS.escape(panel.id) : panel.id) + '"]');
  if (!btn) return;
  btn.click();
  requestAnimationFrame(function () { if (el.reportValidity) el.reportValidity(); });
}, true);

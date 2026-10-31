import { icon } from './icons.js';

export function h(tag, props = {}, ...children) {
  const el = document.createElement(tag);
  applyProps(el, props);
  append(el, children);
  return el;
}

function applyProps(el, props) {
  if (!props) return;
  for (const [key, value] of Object.entries(props)) {
    if (value === undefined || value === null) continue;
    switch (key) {
      case 'class':
      case 'className':
        el.className = value;
        break;
      case 'html':
        el.innerHTML = value;
        break;
      case 'text':
        el.textContent = value;
        break;
      case 'style':
        if (typeof value === 'string') el.style.cssText = value;
        else Object.assign(el.style, value);
        break;
      case 'dataset':
        Object.assign(el.dataset, value);
        break;
      case 'ref':
        value(el);
        break;
      default:
        if (key.startsWith('on') && typeof value === 'function') {
          el.addEventListener(key.slice(2).toLowerCase(), value);
        } else if (value === true) {
          el.setAttribute(key, '');
        } else if (value === false) {
          el.removeAttribute(key);
        } else {
          el.setAttribute(key, value);
        }
    }
  }
}

function append(el, children) {
  for (const child of children.flat(Infinity)) {
    if (child === null || child === undefined || child === false) continue;
    el.append(child instanceof Node ? child : document.createTextNode(String(child)));
  }
}

export function escapeHtml(value) {
  return String(value)
    .replaceAll('&', '&amp;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')
    .replaceAll('"', '&quot;');
}

export function toast(message, type = 'info', timeout = 3800) {
  const host = document.getElementById('toasts');
  const icons = { ok: 'check', err: 'alert', info: 'info', warn: 'alert' };
  const el = h('div', { class: `toast ${type}` },
    h('span', { html: icon(icons[type] || 'info'), style: { color: type === 'err' ? 'var(--red)' : type === 'ok' ? 'var(--accent)' : 'var(--text-dim)' } }),
    h('div', { class: 'msg' }, message),
  );
  host.append(el);
  const remove = () => {
    el.style.opacity = '0';
    el.style.transform = 'translateY(4px)';
    el.style.transition = 'opacity .15s, transform .15s';
    setTimeout(() => el.remove(), 170);
  };
  el.addEventListener('click', remove);
  if (timeout > 0) setTimeout(remove, timeout);
  return el;
}

const modalStack = [];

export function openModal({ title, subtitle = '', body = null, wide = false, onClose = null }) {
  const overlay = h('div', { class: 'modal-overlay' });
  const closeBtn = h('button', { class: 'btn ghost icon', title: '关闭 (Esc)', html: icon('x') });
  const head = h('div', { class: 'modal-head' },
    h('h3', {}, title),
    subtitle ? h('span', { class: 'sub' }, subtitle) : null,
    closeBtn,
  );
  const bodyEl = h('div', { class: 'modal-body' });
  if (body) append(bodyEl, [body]);
  const footEl = h('div', { class: 'modal-foot' });
  const modal = h('div', { class: 'modal' + (wide ? ' wide' : '') }, head, bodyEl, footEl);
  overlay.append(modal);
  document.getElementById('modals').append(overlay);

  let closed = false;
  const entry = { close };
  function close() {
    if (closed) return;
    closed = true;
    overlay.remove();
    const idx = modalStack.indexOf(entry);
    if (idx >= 0) modalStack.splice(idx, 1);
    if (onClose) onClose();
  }
  modalStack.push(entry);
  closeBtn.addEventListener('click', close);
  overlay.addEventListener('mousedown', (e) => { if (e.target === overlay) close(); });
  const firstInput = bodyEl.querySelector('input, select, textarea, button');
  if (firstInput) setTimeout(() => firstInput.focus(), 30);
  return { overlay, modal, body: bodyEl, footer: footEl, head, close };
}

document.addEventListener('keydown', (e) => {
  if (e.key !== 'Escape') return;
  if (modalStack.length) {
    e.preventDefault();
    modalStack[modalStack.length - 1].close();
    return;
  }
  hideContextMenu();
  hideDrawer();
}, true);

export function confirmDialog({ title, message, confirmText = '确定', cancelText = '取消', danger = false }) {
  return new Promise((resolve) => {
    const m = openModal({ title, body: h('div', { class: 'modal-msg', html: message }) });
    const cancel = h('button', { class: 'btn', onClick: () => { resolve(false); m.close(); } }, cancelText);
    const ok = h('button', { class: 'btn ' + (danger ? 'danger' : 'primary'), onClick: () => { resolve(true); m.close(); } }, confirmText);
    m.footer.append(h('span', { class: 'spacer' }), cancel, ok);
    m.overlay.addEventListener('mousedown', (e) => { if (e.target === m.overlay) resolve(false); });
    setTimeout(() => ok.focus(), 40);
  });
}

/* ---------- context menu ---------- */
let ctxEl = null;

export function contextMenu(x, y, items) {
  hideContextMenu();
  const el = document.getElementById('context-menu');
  el.innerHTML = '';
  el.classList.remove('hidden');
  for (const item of items) {
    if (!item || item.sep) {
      el.append(h('div', { class: 'ctx-sep' }));
      continue;
    }
    const row = h('div', { class: 'ctx-item' + (item.danger ? ' danger' : '') },
      item.icon ? h('span', { html: icon(item.icon), style: { display: 'flex' } }) : null,
      h('span', {}, item.label),
      item.shortcut ? h('span', { class: 'shortcut' }, item.shortcut) : null,
    );
    row.addEventListener('mousedown', (e) => {
      e.preventDefault();
      e.stopPropagation();
      hideContextMenu();
      if (item.action) item.action();
    });
    el.append(row);
  }
  el.style.left = '0px';
  el.style.top = '0px';
  const rect = el.getBoundingClientRect();
  const left = Math.min(x, window.innerWidth - rect.width - 8);
  const top = Math.min(y, window.innerHeight - rect.height - 8);
  el.style.left = Math.max(6, left) + 'px';
  el.style.top = Math.max(6, top) + 'px';
  ctxEl = el;
}

export function hideContextMenu() {
  const el = document.getElementById('context-menu');
  if (el) el.classList.add('hidden');
  ctxEl = null;
}

document.addEventListener('mousedown', (e) => {
  const el = document.getElementById('context-menu');
  if (el && !el.classList.contains('hidden') && !el.contains(e.target)) hideContextMenu();
}, true);
window.addEventListener('scroll', hideContextMenu, true);

/* ---------- drawer ---------- */
let drawerEls = null;
export function openDrawer({ title, body, onClose }) {
  closeDrawer();
  const overlay = h('div', { class: 'drawer-overlay' });
  const drawer = h('div', { class: 'drawer' },
    h('div', { class: 'drawer-head' },
      h('h3', {}, title),
      h('span', { class: 'flex-1' }),
      h('button', { class: 'btn ghost icon', html: icon('x'), onClick: () => closeDrawer() }),
    ),
    h('div', { class: 'drawer-body' }, body || ''),
  );
  overlay.addEventListener('mousedown', () => closeDrawer());
  document.body.append(overlay, drawer);
  drawerEls = { overlay, drawer, onClose };
  return drawer;
}

export function closeDrawer() {
  if (!drawerEls) return;
  drawerEls.overlay.remove();
  drawerEls.drawer.remove();
  if (drawerEls.onClose) drawerEls.onClose();
  drawerEls = null;
}

export function hideDrawer() { closeDrawer(); }

/* ---------- clipboard / download ---------- */
export async function copyText(text) {
  try {
    await navigator.clipboard.writeText(text);
    toast('已复制到剪贴板', 'ok', 1600);
  } catch {
    const ta = document.createElement('textarea');
    ta.value = text;
    ta.style.position = 'fixed';
    ta.style.opacity = '0';
    document.body.append(ta);
    ta.select();
    document.execCommand('copy');
    ta.remove();
    toast('已复制到剪贴板', 'ok', 1600);
  }
}

export function downloadBlob(blob, filename) {
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = filename;
  document.body.append(a);
  a.click();
  a.remove();
  setTimeout(() => URL.revokeObjectURL(url), 4000);
}

export function downloadText(text, filename, mime = 'text/plain;charset=utf-8') {
  downloadBlob(new Blob([text], { type: mime }), filename);
}

/* ---------- formatting ---------- */
export function formatBytes(n) {
  n = Number(n) || 0;
  if (n < 1024) return n + ' B';
  const units = ['KB', 'MB', 'GB', 'TB'];
  let i = -1;
  do { n /= 1024; i++; } while (n >= 1024 && i < units.length - 1);
  return n.toFixed(n >= 100 ? 0 : n >= 10 ? 1 : 2) + ' ' + units[i];
}

export function formatNumber(n) {
  if (n === null || n === undefined) return '—';
  return Number(n).toLocaleString('en-US');
}

export function formatMs(ms) {
  if (ms === null || ms === undefined) return '—';
  if (ms < 1000) return ms + ' ms';
  return (ms / 1000).toFixed(2) + ' s';
}

export function timeAgo(iso) {
  const t = new Date(iso).getTime();
  if (!t) return '';
  const diff = Math.max(0, Date.now() - t);
  const s = Math.floor(diff / 1000);
  if (s < 60) return s + ' 秒前';
  const m = Math.floor(s / 60);
  if (m < 60) return m + ' 分钟前';
  const hr = Math.floor(m / 60);
  if (hr < 24) return hr + ' 小时前';
  const d = Math.floor(hr / 24);
  if (d < 30) return d + ' 天前';
  return new Date(iso).toLocaleDateString();
}

export function debounce(fn, ms = 200) {
  let timer = null;
  return (...args) => {
    clearTimeout(timer);
    timer = setTimeout(() => fn(...args), ms);
  };
}

export function clamp(v, min, max) { return Math.max(min, Math.min(max, v)); }

export function csvEscape(value) {
  const s = value === null || value === undefined ? '' : String(value);
  if (/[",\n\r]/.test(s)) return '"' + s.replaceAll('"', '""') + '"';
  return s;
}

export function toCSV(columns, rows) {
  const head = columns.map((c) => csvEscape(c.name)).join(',');
  const body = rows.map((row) => row.map((v) => csvEscape(v)).join(',')).join('\n');
  return '\ufeff' + head + '\n' + body + '\n';
}

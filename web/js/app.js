import { api, apiGet, apiPost, TOKEN, VERSION } from './api.js';
import {
  h, toast, openModal, confirmDialog, contextMenu, copyText, downloadBlob, downloadText,
  formatBytes, formatNumber, formatMs, timeAgo, debounce, escapeHtml, openDrawer, closeDrawer,
} from './dom.js';
import { icon } from './icons.js';
import { Grid, gridToCSV } from './grid.js';
import { SQLEditor } from './editor.js';
import { openAssistant } from './ai.js';
import { setLang, getLang, translateTree } from './i18n.js';

const $ = (sel) => document.querySelector(sel);

const S = {
  version: VERSION,
  connections: [],
  multiSelect: false,
  selectedConns: new Set(),
  settings: { theme: 'dark', language: 'en', pageSize: 200, maxRows: 1000, fontSize: 13, fontFamily: 'system', background: '', defaultSort: true },
  tabs: [],
  activeId: null,
  seq: 0,
  conns: new Map(),
  tableDetails: new Map(),
  search: '',
};

const KEYWORDS = `
SELECT FROM WHERE INSERT INTO VALUES UPDATE SET DELETE CREATE TABLE ALTER DROP TRUNCATE
JOIN INNER LEFT RIGHT FULL OUTER CROSS ON USING AS AND OR NOT NULL IS IN BETWEEN LIKE
GROUP BY ORDER HAVING LIMIT OFFSET UNION ALL DISTINCT CASE WHEN THEN ELSE END EXISTS
PRIMARY KEY FOREIGN REFERENCES UNIQUE INDEX CONSTRAINT DEFAULT AUTO_INCREMENT COMMENT
ENGINE CHARSET COLLATE SHOW DATABASES TABLES COLUMNS INDEXES VIEW TRIGGER PROCEDURE
FUNCTION EVENT DESCRIBE EXPLAIN ANALYZE BEGIN START TRANSACTION COMMIT ROLLBACK
GRANT REVOKE USER IDENTIFIED PASSWORD WITH OPTION LOCK UNLOCK READ WRITE IF CALL USE
DATABASE SCHEMA TEMPORARY REPLACE IGNORE ASC DESC OVER WINDOW RECURSIVE
`.trim().split(/\s+/);

/* ------------------------------------------------------------------ */
/* bootstrap                                                          */
/* ------------------------------------------------------------------ */

async function init() {
  bindStaticIcons();
  bindChrome();
  applyTheme(S.settings.theme);
  try {
    const state = await apiGet('/api/state');
    S.connections = state.connections || [];
    S.settings = Object.assign(S.settings, state.settings || {});
    setLang(S.settings.language || 'en');
    applyTheme(S.settings.theme);
    applyFontSettings();
    applyBackground();
    document.getElementById('version-chip').textContent = 'v' + (state.app?.version || VERSION);
  } catch (err) {
    setLang('en');
    toast('无法连接 NavMini 服务：' + err.message, 'err', 8000);
  }
  translateTree(document.body);
  renderTree();
  openWelcome();
  translateTree(document.body);
  bindShortcuts();
  window.__navmini.openSettings = openSettings;
}

function bindStaticIcons() {
  $('#btn-new-connection').innerHTML = icon('plus');
  $('#btn-multi-conn').innerHTML = icon('check');
  $('#btn-refresh-tree').innerHTML = icon('refresh');
  $('#btn-new-query').innerHTML = icon('plus');
  $('#btn-monitor').innerHTML = icon('activity');
  $('#btn-settings-top').innerHTML = icon('settings');
  $('#btn-ai').innerHTML = icon('sparkles');
  $('#btn-about').innerHTML = icon('info');
  $('#btn-theme').innerHTML = icon('moon');
  $('.search-icon').innerHTML = icon('search');
}

function bindChrome() {
  $('#btn-new-connection').addEventListener('click', () => openConnectionDialog());
  $('#btn-multi-conn').addEventListener('click', () => toggleConnMultiSelect());
  $('#btn-refresh-tree').addEventListener('click', refreshTree);
  $('#btn-new-query').addEventListener('click', () => newQueryFromContext());
  $('#btn-monitor').addEventListener('click', () => openMonitor());
  $('#btn-ai').addEventListener('click', () => openAIAssistantFromActive());
  $('#btn-about').addEventListener('click', openAbout);
  $('#btn-settings').addEventListener('click', openSettings);
  $('#btn-settings-top').addEventListener('click', () => openSettings());
  $('#btn-history').addEventListener('click', openHistory);
  $('#btn-saved').addEventListener('click', () => openSavedQueries());
  $('#btn-export-connections').addEventListener('click', openExportConnections);
  $('#btn-theme').addEventListener('click', toggleTheme);
  $('#tree-search').addEventListener('input', debounce((e) => { S.search = e.target.value.trim(); renderTree(); }, 120));
  bindSidebarResize();
}

function applyFontSettings() {
  const family = S.settings.fontFamily || 'system';
  const ui = family === 'mono'
    ? 'ui-monospace, "SF Mono", Menlo, Consolas, monospace'
    : family === 'serif'
      ? 'Georgia, "Times New Roman", "Songti SC", serif'
      : '-apple-system, BlinkMacSystemFont, "SF Pro Text", "Helvetica Neue", "PingFang SC", "Microsoft YaHei", sans-serif';
  document.documentElement.style.setProperty('--ui', ui);
  for (const tab of S.tabs) if (tab.editor) applyEditorFont(tab.editor);
}

function applyBackground() {
  const bg = S.settings.background || '';
  if (bg) {
    document.body.style.setProperty('--bg-image', `url("/api/background?v=${encodeURIComponent(bg)}")`);
    document.body.classList.add('has-bg');
  } else {
    document.body.style.removeProperty('--bg-image');
    document.body.classList.remove('has-bg');
  }
}

function applyTheme(theme) {
  if (theme !== 'light') theme = 'dark';
  S.settings.theme = theme;
  document.documentElement.dataset.theme = theme;
  const btn = $('#btn-theme');
  if (btn) btn.innerHTML = icon(theme === 'dark' ? 'moon' : 'sun');
}

async function toggleTheme() {
  applyTheme(S.settings.theme === 'dark' ? 'light' : 'dark');
  try { await apiPost('/api/settings', S.settings); } catch { /* non-fatal */ }
}

/* ------------------------------------------------------------------ */
/* connection state helpers                                           */
/* ------------------------------------------------------------------ */

function connById(id) { return S.connections.find((c) => c.id === id); }

function connState(id) {
  if (!S.conns.has(id)) {
    S.conns.set(id, {
      sessionId: null, connected: false, server: null, databases: null, objects: new Map(),
      openDbs: new Set(), expanded: false, loading: false, opening: null, error: null,
    });
  }
  return S.conns.get(id);
}

async function openSession(connId, database = '') {
  return apiPost('/api/session/open', { connectionId: connId, database });
}

async function ensureTreeSession(connId) {
  const cs = connState(connId);
  if (cs.sessionId) return cs.sessionId;
  if (!cs.opening) {
    cs.opening = openSession(connId).then((v) => {
      cs.sessionId = v.id;
      cs.connected = true;
      cs.server = v.server;
      return v.id;
    }).finally(() => { cs.opening = null; });
  }
  return cs.opening;
}

async function ensureTabSession(tab) {
  if (tab.sessionId) return tab.sessionId;
  if (tab.opening) return tab.opening;
  tab.opening = openSession(tab.connId).then((v) => {
    tab.sessionId = v.id;
    tab.server = v.server;
    const cs = connState(tab.connId);
    cs.connected = true;
    renderTree();
    return v.id;
  }).finally(() => { tab.opening = null; });
  return tab.opening;
}

function isStaleSession(err) {
  const msg = (err && err.message) || '';
  return /session not found|session is closed|bad connection|invalid connection|connection is already closed/i.test(msg);
}

async function withTabSession(tab, fn) {
  try {
    return await fn(await ensureTabSession(tab));
  } catch (err) {
    if (!isStaleSession(err)) throw err;
    tab.sessionId = null;
    tab.opening = null;
    return await fn(await ensureTabSession(tab));
  }
}

function setStatus(connText, dbText, message, rowsText) {
  $('#status-conn').textContent = connText || '未连接';
  $('#status-db').textContent = dbText || '';
  if (message !== undefined) $('#status-message').textContent = message || '';
  if (rowsText !== undefined) $('#status-rows').textContent = rowsText || '';
}

function updateStatusForTab(tab) {
  if (!tab) return setStatus('未连接', '', '', '');
  const conn = connById(tab.connId);
  const name = conn ? conn.name : (tab.server?.version ? 'MySQL' : '未连接');
  setStatus(name, tab.db || '', '', '');
}

/* ------------------------------------------------------------------ */
/* sidebar / tree                                                     */
/* ------------------------------------------------------------------ */

function tableCacheKey(connId, db, table) { return `${connId}|${db}|${table}`; }

async function refreshDatabase(connId, dbName) {
  const cs = connState(connId);
  try {
    const sid = await ensureTreeSession(connId);
    const objs = await apiGet(`/api/objects?session=${encodeURIComponent(sid)}&db=${encodeURIComponent(dbName)}`);
    cs.objects.set(dbName, objs);
    for (const key of [...S.tableDetails.keys()]) {
      if (key.startsWith(`${connId}|${dbName}|`)) S.tableDetails.delete(key);
    }
    for (const tab of S.tabs) {
      if (tab.kind === 'table' && tab.connId === connId && tab.db === dbName) {
        tab.detail = null;
        loadTableDetail(tab);
      }
    }
    renderTree();
    toast(`已刷新 ${dbName} 的结构`, 'ok', 2200);
  } catch (err) { toast(err.message, 'err', 5000); }
}

async function refreshTableDetail(connId, db, table) {
  S.tableDetails.delete(tableCacheKey(connId, db, table));
  let tab = S.tabs.find((t) => t.kind === 'table' && t.connId === connId && t.db === db && t.table === table);
  if (!tab) tab = openTable(connId, db, table);
  else {
    activateTab(tab.id);
    tab.detail = null;
    await loadTableDetail(tab);
    loadTableRows(tab);
  }
}

function refreshTree() {
  S.tableDetails.clear();
  for (const cs of S.conns.values()) {
    cs.databases = null;
    cs.objects.clear();
    cs.openDbs.clear();
  }
  renderTree();
}

function matches(text, q) { return (text || '').toLowerCase().includes(q); }

function toggleConnMultiSelect(force) {
  S.multiSelect = typeof force === 'boolean' ? force : !S.multiSelect;
  if (!S.multiSelect) S.selectedConns.clear();
  document.querySelector('#btn-multi-conn')?.classList.toggle('primary', S.multiSelect);
  renderTree();
}

function toggleConnSelection(id) {
  if (!S.multiSelect) return;
  if (S.selectedConns.has(id)) S.selectedConns.delete(id);
  else S.selectedConns.add(id);
  renderTree();
}

function buildConnBulkBar() {
  const all = S.connections.length > 0 && S.connections.every((c) => S.selectedConns.has(c.id));
  const allBox = h('input', { type: 'checkbox', checked: all });
  allBox.addEventListener('change', () => {
    S.selectedConns.clear();
    if (allBox.checked) for (const c of S.connections) S.selectedConns.add(c.id);
    renderTree();
  });
  const del = h('button', { class: 'btn sm danger', disabled: !S.selectedConns.size }, `删除${S.selectedConns.size ? ` (${S.selectedConns.size})` : ''}`);
  del.addEventListener('click', deleteSelectedConnections);
  return h('div', { class: 'conn-bulk' },
    h('label', { class: 'check' }, allBox, h('span', {}, '全选')),
    h('span', { class: 'faint' }, `${S.selectedConns.size} / ${S.connections.length}`),
    h('span', { class: 'spacer' }),
    del,
    h('button', { class: 'btn sm', onClick: () => toggleConnMultiSelect(false) }, '退出'),
  );
}

async function deleteSelectedConnections() {
  const ids = [...S.selectedConns];
  if (!ids.length) return;
  const names = S.connections.filter((c) => ids.includes(c.id)).map((c) => c.name);
  const ok = await confirmDialog({
    title: '批量删除连接',
    message: `确定删除选中的 <b>${ids.length}</b> 个连接？<br><span class="faint">${escapeHtml(names.join(', '))}</span><br>仅删除本地配置，不影响数据库。`,
    confirmText: '删除',
    danger: true,
  });
  if (!ok) return;
  try {
    await apiPost('/api/connections/delete-bulk', { ids });
    for (const id of ids) {
      const cs = S.conns.get(id);
      if (cs?.sessionId) apiPost('/api/session/close', { session: cs.sessionId }).catch(() => {});
      S.conns.delete(id);
    }
    for (const t of [...S.tabs]) if (ids.includes(t.connId)) closeTab(t.id);
    S.selectedConns.clear();
    await reloadConnections();
    renderTree();
    toast(`已删除 ${ids.length} 个连接`, 'ok');
  } catch (err) { toast(err.message, 'err', 6000); }
}

function renderTree() {
  const tree = $('#tree');
  const scrollTop = tree.scrollTop;
  tree.innerHTML = '';
  const q = S.search.toLowerCase();
  if (S.multiSelect) tree.append(buildConnBulkBar());
  let shown = 0;
  for (const conn of S.connections) {
    const cs = connState(conn.id);
    const connHit = !q || matches(conn.name, q) || matches(conn.host, q) || matches(conn.user, q) || matches(conn.group, q);
    let childHit = false;
    if (q && cs.databases) {
      for (const db of cs.databases) {
        if (matches(db.name, q)) { childHit = true; break; }
        const objs = cs.objects.get(db.name);
        if (objs && objs.some((o) => matches(o.name, q))) { childHit = true; break; }
      }
    }
    if (!connHit && !childHit) continue;
    shown++;
    tree.append(renderConnNode(conn, cs, q, connHit));
  }
  if (!shown) {
    tree.append(h('div', { class: 'tree-empty', style: { padding: '18px 10px' } },
      S.connections.length ? '没有匹配的连接' : '还没有连接，点击 + 新建'));
    if (!S.connections.length) {
      tree.append(h('button', { class: 'btn sm', style: { margin: '0 12px' }, onClick: () => openConnectionDialog() }, '新建连接'));
    }
  }
  tree.scrollTop = scrollTop;
}

function renderConnNode(conn, cs, q, connHit) {
  const node = h('div', { class: 'tree-node' });
  const open = cs.expanded || (q && connHit);
  const caret = h('span', { class: 'node-caret' + (open ? ' open' : ''), html: icon('chevron-right') });
  const connected = !!(cs.connected || cs.sessionId);
  const checked = S.selectedConns.has(conn.id);
  const cb = S.multiSelect ? h('input', { type: 'checkbox', class: 'conn-check', checked }) : null;
  if (cb) {
    cb.addEventListener('click', (e) => e.stopPropagation());
    cb.addEventListener('change', () => toggleConnSelection(conn.id));
  }
  const row = h('div', { class: 'node-row ' + (connected ? 'conn-on' : 'conn-off') + (checked ? ' conn-checked' : ''), style: { '--depth': 0 }, title: `${conn.user}@${conn.host}:${conn.port}` },
    ...(cb ? [cb] : []),
    caret,
    h('span', { class: 'node-icon conn ' + (connected ? 'connected' : 'disconnected'), html: icon('database') }),
    h('span', { class: 'node-name' }, conn.name),
    h('span', { class: 'node-meta' }, cs.databases ? `${cs.databases.length} 个库` : (cs.server ? '已连接' : '')),
    h('span', { class: 'row-actions' },
      h('button', { class: 'btn ghost sm icon', title: '新建查询', html: icon('terminal'), onClick: (e) => { e.stopPropagation(); openQuery(conn.id, conn.database || '', ''); } }),
      h('button', { class: 'btn ghost sm icon', title: '编辑连接', html: icon('pencil'), onClick: (e) => { e.stopPropagation(); openConnectionDialog(conn); } }),
    ),
  );
  row.addEventListener('click', () => {
    if (S.multiSelect) toggleConnSelection(conn.id);
    else toggleConn(conn.id);
  });
  row.addEventListener('dblclick', () => { if (!S.multiSelect) openQuery(conn.id, conn.database || '', ''); });
  row.addEventListener('contextmenu', (e) => {
    e.preventDefault();
    contextMenu(e.clientX, e.clientY, [
      { label: '新建查询', icon: 'terminal', action: () => openQuery(conn.id, conn.database || '', '') },
      { label: '刷新', icon: 'refresh', action: () => {
        cs.databases = null; cs.objects.clear();
        for (const key of [...S.tableDetails.keys()]) if (key.startsWith(`${conn.id}|`)) S.tableDetails.delete(key);
        renderTree();
      } },
      { sep: true },
      { label: '编辑连接', icon: 'pencil', action: () => openConnectionDialog(conn) },
      { label: '删除连接', icon: 'trash', danger: true, action: () => deleteConnection(conn) },
    ]);
  });
  if (S.activeTabConn === conn.id) row.classList.add('selected');
  node.append(row);

  if (open) {
    if (cs.loading || cs.opening) {
      node.append(h('div', { class: 'tree-loading' }, h('span', { class: 'spinner' }), '正在连接…'));
    } else if (cs.error) {
      node.append(h('div', { class: 'tree-empty', style: { color: 'var(--red)' } }, cs.error));
    } else if (cs.databases) {
      for (const db of cs.databases) {
        if (q && !connHit && !matches(db.name, q) && !(cs.objects.get(db.name) || []).some((o) => matches(o.name, q))) continue;
        node.append(renderDbNode(conn, cs, db, q));
      }
      if (!cs.databases.length) node.append(h('div', { class: 'tree-empty' }, '没有数据库'));
    }
  }
  return node;
}

function renderDbNode(conn, cs, db, q) {
  const wrap = h('div', { class: 'tree-node' });
  const open = cs.openDbs.has(db.name) || (q && matches(db.name, q));
  const objects = cs.objects.get(db.name);
  const row = h('div', { class: 'node-row', style: { '--depth': 1 }, title: db.collation || db.name },
    h('span', { class: 'node-caret' + (open ? ' open' : ''), html: icon('chevron-right') }),
    h('span', { class: 'node-icon db', html: icon('database') }),
    h('span', { class: 'node-name' }, db.name),
    h('span', { class: 'node-meta' }, db.tables ? formatNumber(db.tables) : ''),
  );
  row.addEventListener('click', () => toggleDb(conn.id, db.name));
  row.addEventListener('contextmenu', (e) => {
    e.preventDefault();
    contextMenu(e.clientX, e.clientY, [
      { label: '新建查询', icon: 'terminal', action: () => openQuery(conn.id, db.name, '') },
      { label: '刷新库结构', icon: 'refresh', action: () => refreshDatabase(conn.id, db.name) },
      { label: '快速导出…', icon: 'download', action: () => openDatabaseExport(conn.id, db.name) },
      { label: '复制库名', icon: 'copy', action: () => copyText(db.name) },
    ]);
  });
  wrap.append(row);

  if (open && objects) {
    const tables = objects.filter((o) => o.type !== 'VIEW' && (!q || matches(o.name, q) || matches(db.name, q)));
    const views = objects.filter((o) => o.type === 'VIEW' && (!q || matches(o.name, q) || matches(db.name, q)));
    if (tables.length) {
      wrap.append(h('div', { class: 'tree-section', style: { '--depth': 2 } }, `表 · ${tables.length}`));
      for (const t of tables) wrap.append(renderTableNode(conn, db, t));
    }
    if (views.length) {
      wrap.append(h('div', { class: 'tree-section', style: { '--depth': 2 } }, `视图 · ${views.length}`));
      for (const v of views) wrap.append(renderTableNode(conn, db, v));
    }
    if (!tables.length && !views.length) wrap.append(h('div', { class: 'tree-empty', style: { '--depth': 2 } }, '没有对象'));
  } else if (open) {
    wrap.append(h('div', { class: 'tree-loading', style: { '--depth': 2 } }, h('span', { class: 'spinner' }), '加载对象…'));
  }
  return wrap;
}

function renderTableNode(conn, db, obj) {
  const isView = obj.type === 'VIEW';
  const row = h('div', { class: 'node-row', style: { '--depth': 2 }, title: `${obj.name}${obj.engine ? ' · ' + obj.engine : ''}` },
    h('span', { class: 'node-caret leaf' }),
    h('span', { class: 'node-icon ' + (isView ? 'view' : 'table'), html: icon(isView ? 'view' : 'table') }),
    h('span', { class: 'node-name' }, obj.name),
    h('span', { class: 'node-meta' }, obj.rows ? formatNumber(obj.rows) : ''),
  );
  row.addEventListener('click', () => openTable(conn.id, db.name, obj.name));
  row.addEventListener('contextmenu', (e) => {
    e.preventDefault();
    contextMenu(e.clientX, e.clientY, [
      { label: '打开数据', icon: 'table', action: () => openTable(conn.id, db.name, obj.name) },
      { label: '刷新表结构', icon: 'refresh', action: () => refreshTableDetail(conn.id, db.name, obj.name) },
      { label: '新建查询', icon: 'terminal', action: () => openQuery(conn.id, db.name, `SELECT * FROM ${ident(db.name)}.${ident(obj.name)} LIMIT 100;`) },
      { sep: true },
      { label: '导出 CSV', icon: 'download', action: () => exportTableFromTree(conn.id, db.name, obj.name) },
      { label: '快速备份到当前库…', icon: 'save', action: () => backupTableFromTree(conn.id, db.name, obj.name) },
      { sep: true },
      { label: '复制名称', icon: 'copy', action: () => copyText(obj.name) },
    ]);
  });
  return row;
}

async function toggleConn(connId) {
  const cs = connState(connId);
  cs.expanded = !cs.expanded;
  S.activeTabConn = connId;
  renderTree();
  if (!cs.expanded || cs.databases) return;
  cs.loading = true;
  cs.error = null;
  renderTree();
  try {
    const sid = await ensureTreeSession(connId);
    cs.databases = await apiGet('/api/databases?session=' + encodeURIComponent(sid));
    if (!cs.sessionId) cs.sessionId = sid;
  } catch (err) {
    cs.error = err.message;
    cs.expanded = false;
  } finally {
    cs.loading = false;
    renderTree();
  }
}

async function toggleDb(connId, dbName, forceLoad = false) {
  const cs = connState(connId);
  if (cs.openDbs.has(dbName)) {
    cs.openDbs.delete(dbName);
    renderTree();
    return;
  }
  cs.openDbs.add(dbName);
  cs.expanded = true;
  renderTree();
  if (cs.objects.has(dbName) && !forceLoad) return;
  try {
    const sid = await ensureTreeSession(connId);
    const objs = await apiGet(`/api/objects?session=${encodeURIComponent(sid)}&db=${encodeURIComponent(dbName)}`);
    cs.objects.set(dbName, objs);
  } catch (err) {
    toast(`加载 ${dbName} 失败：${err.message}`, 'err', 5000);
  }
  renderTree();
}

async function deleteConnection(conn) {
  const ok = await confirmDialog({
    title: '删除连接',
    message: `确定删除 <code>${escapeHtml(conn.name)}</code>？仅删除本地保存的连接配置，不会影响数据库。`,
    confirmText: '删除',
    danger: true,
  });
  if (!ok) return;
  try {
    await apiPost('/api/connections/delete', { id: conn.id });
    const cs = S.conns.get(conn.id);
    if (cs?.sessionId) apiPost('/api/session/close', { session: cs.sessionId }).catch(() => {});
    S.conns.delete(conn.id);
    await reloadConnections();
    renderTree();
    toast('连接已删除', 'ok');
  } catch (err) {
    toast(err.message, 'err', 5000);
  }
}

async function reloadConnections() {
  const state = await apiGet('/api/state');
  S.connections = state.connections || [];
  S.settings = Object.assign(S.settings, state.settings || {});
  const ids = new Set(S.connections.map((c) => c.id));
  for (const id of [...S.selectedConns]) if (!ids.has(id)) S.selectedConns.delete(id);
}

/* ------------------------------------------------------------------ */
/* tabs                                                               */
/* ------------------------------------------------------------------ */

function tabIcon(kind) {
  return { query: 'code', table: 'table', monitor: 'activity', welcome: 'bolt' }[kind] || 'file';
}

function activeTab() { return S.tabs.find((t) => t.id === S.activeId) || null; }

function addTab(kind, props = {}) {
  const tab = Object.assign({ id: 't' + (++S.seq), kind, title: '', el: document.createElement('div') }, props);
  tab.el.className = 'pane';
  tab.el.dataset.tab = tab.id;
  $('#workspace').append(tab.el);
  S.tabs.push(tab);
  activateTab(tab.id);
  return tab;
}

function activateTab(id) {
  S.activeId = id;
  for (const t of S.tabs) t.el.classList.toggle('active', t.id === id);
  renderTabs();
  const tab = activeTab();
  if (tab) {
    S.activeTabConn = tab.connId || null;
    updateStatusForTab(tab);
    if (tab.onActivate) setTimeout(() => tab.onActivate(), 0);
    renderTree();
  }
  $('#tabs').scrollTo({ left: 99999 });
}

function renderTabs() {
  const host = $('#tabs');
  host.innerHTML = '';
  for (const tab of S.tabs) {
    const close = h('button', { class: 'tab-close', title: '关闭 (⌘W)', html: icon('x') });
    close.addEventListener('click', (e) => { e.stopPropagation(); closeTab(tab.id); });
    const node = h('div', { class: 'tab' + (tab.id === S.activeId ? ' active' : ''), title: tab.title, role: 'tab' },
      h('span', { class: 'tab-ico', html: icon(tabIcon(tab.kind)) }),
      h('span', { class: 'tab-title' }, tab.title),
      close,
    );
    node.addEventListener('click', () => activateTab(tab.id));
    node.addEventListener('auxclick', (e) => { if (e.button === 1) closeTab(tab.id); });
    node.addEventListener('contextmenu', (e) => {
      e.preventDefault();
      contextMenu(e.clientX, e.clientY, [
        { label: '关闭', icon: 'x', action: () => closeTab(tab.id) },
        { label: '关闭其他标签', icon: 'x', action: () => closeOtherTabs(tab.id) },
      ]);
    });
    host.append(node);
  }
}

function closeOtherTabs(keepId) {
  for (const t of [...S.tabs]) if (t.id !== keepId) closeTab(t.id, true);
}

async function closeTab(id, silent = false) {
  const idx = S.tabs.findIndex((t) => t.id === id);
  if (idx < 0) return;
  const tab = S.tabs[idx];
  if (tab.running && tab.queryId && tab.sessionId) {
    apiPost('/api/query/cancel', { session: tab.sessionId, queryId: tab.queryId }).catch(() => {});
  }
  if (tab.sessionId) apiPost('/api/session/close', { session: tab.sessionId }).catch(() => {});
  if (tab.dispose) tab.dispose();
  tab.el.remove();
  S.tabs.splice(idx, 1);
  if (S.activeId === id) {
    const next = S.tabs[Math.min(idx, S.tabs.length - 1)];
    if (next) activateTab(next.id);
    else { S.activeId = null; openWelcome(); }
  }
  renderTabs();
  if (!silent) updateStatusForTab(activeTab());
}

/* ------------------------------------------------------------------ */
/* welcome                                                            */
/* ------------------------------------------------------------------ */

function openWelcome() {
  const existing = S.tabs.find((t) => t.kind === 'welcome');
  if (existing) {
    activateTab(existing.id);
    renderWelcome(existing);
    return existing;
  }
  const tab = addTab('welcome', { title: '欢迎' });
  renderWelcome(tab);
  return tab;
}

function renderWelcome(tab) {
  tab.el.innerHTML = '';
  const card = h('div', { class: 'welcome-card' });
  card.append(
    h('div', { class: 'welcome-hero' },
      h('div', { class: 'welcome-logo' }),
      h('div', {},
        h('h1', { class: 'welcome-title' }, 'NavMini'),
        h('p', { class: 'welcome-sub' }, '一个受某软件启发的轻量 MySQL 工作台 · Go + 原生 WebView'),
      ),
    ),
  );
  const quick = h('div', { class: 'welcome-panel' }, h('h3', {}, '快速连接'));
  if (!S.connections.length) {
    quick.append(h('div', { class: 'welcome-empty' }, '还没有保存任何连接。'));
    quick.append(h('button', { class: 'btn primary sm', style: { marginTop: '8px' }, onClick: () => openConnectionDialog() }, '新建连接'));
  } else {
    for (const conn of S.connections.slice(0, 6)) {
      const row = h('div', { class: 'recent-conn' },
        h('span', { class: 'node-icon', html: icon('database') }),
        h('span', { class: 'name' }, conn.name),
        h('span', { class: 'meta' }, `${conn.user}@${conn.host}:${conn.port}`),
      );
      row.addEventListener('click', () => openQuery(conn.id, conn.database || '', ''));
      quick.append(row);
    }
  }
  const shortcuts = h('div', { class: 'welcome-panel' }, h('h3', {}, '快捷键'));
  const rows = [
    ['运行全部 / 运行选中', '⌘ ↩ / ⇧⌘ ↩'],
    ['新建查询', '⌘ T'],
    ['新建连接', '⌘ N'],
    ['关闭标签页', '⌘ W'],
    ['自动补全', '⌃ Space'],
  ];
  for (const [label, keys] of rows) shortcuts.append(h('div', { class: 'shortcut-row' }, h('span', {}, label), h('span', { class: 'kbd' }, keys)));

  card.append(h('div', { class: 'welcome-grid' }, quick, shortcuts));
  card.append(h('p', { class: 'faint', style: { marginTop: '18px', fontSize: '11.5px', lineHeight: '1.7' } },
    '提示：单击左侧表名打开数据浏览器；在结果网格中右键可复制单元格/整行。SQL 编辑器支持 ⌘↩ 执行与表/列名补全。'));
  tab.el.append(h('div', { class: 'welcome' }, card));
}

/* ------------------------------------------------------------------ */
/* query tab                                                          */
/* ------------------------------------------------------------------ */

function openAIAssistantFromActive() {
  const tab = activeTab();
  const connId = tab?.connId || S.connections[0]?.id;
  if (!connId) { toast('请先选择连接', 'warn'); openConnectionDialog(); return; }
  const conn = connById(connId);
  openAssistant({
    connId,
    db: tab?.db || conn?.database || '',
    selectedTable: tab?.kind === 'table' ? tab.table : '',
  });
}

function askAIForRow(tab, row) {
  const r = tab.rowsData;
  if (!r || !r.rows[row]) return;
  const obj = {};
  for (let i = 0; i < r.columns.length; i++) obj[r.columns[i].name] = r.rows[row][i];
  openAssistant({ connId: tab.connId, db: tab.db, selectedTable: tab.table, selectedRow: obj });
}

function newQueryFromContext() {
  const tab = activeTab();
  const connId = tab?.connId || S.connections[0]?.id;
  if (!connId) { toast('请先新建一个连接', 'warn'); openConnectionDialog(); return; }
  const db = tab?.db || connById(connId)?.database || '';
  openQuery(connId, db, '');
}

function openQuery(connId, db = '', sql = '') {
  const conn = connById(connId);
  if (!conn) { toast('连接不存在', 'err'); return null; }
  const count = S.tabs.filter((t) => t.kind === 'query').length + 1;
  const tab = addTab('query', { connId, db: db || conn.database || '', title: `查询 ${count}` });
  buildQueryTab(tab, sql);
  return tab;
}

function buildQueryTab(tab, initialSQL) {
  const conn = connById(tab.connId);
  tab.sql = initialSQL || tab.sql || '';
  tab.results = [];
  tab.error = '';
  tab.activeResult = 0;
  tab.running = false;
  tab.maxRows = S.settings.maxRows;

  const root = h('div', { class: 'qt' });
  const dbSel = h('select', { class: 'sel', title: '当前数据库' }, h('option', { value: '' }, '（未选择数据库）'));
  const runBtn = h('button', { class: 'btn primary sm', html: icon('play') + '<span>运行</span>' });
  const runSelBtn = h('button', { class: 'btn sm', title: '运行选中 (⇧⌘↩)', html: icon('play') + '<span>选中</span>' });
  const stopBtn = h('button', { class: 'btn sm danger', disabled: true, html: icon('stop') + '<span>停止</span>' });
  const explainBtn = h('button', { class: 'btn sm', title: 'EXPLAIN 当前语句', html: icon('activity') + '<span>解释</span>' });
  const limitSel = h('select', { class: 'sel', title: '每次最多返回的行数' },
    ...[100, 500, 1000, 5000].map((n) => h('option', { value: String(n) }, `${n} 行`)));
  limitSel.value = String(tab.maxRows);

  const toolbar = h('div', { class: 'qt-toolbar' },
    h('span', { class: 'conn-chip' }, h('span', { class: 'dot' }), conn?.name || '连接'),
    dbSel,
    h('span', { class: 'spacer' }),
    runBtn, runSelBtn, stopBtn, explainBtn,
    h('button', { class: 'btn sm', onClick: () => saveCurrentQuery(tab) }, '保存'),
    h('button', { class: 'btn sm', onClick: () => openSavedQueries(tab.connId) }, '已保存'),
    h('span', { class: 'spacer' }),
    h('span', { class: 'faint', style: { fontSize: '11px' } }, '最大行数'), limitSel,
  );

  const editorHost = h('div', { class: 'editor-shell' });
  const splitter = h('div', { class: 'splitter', title: '拖动调整编辑区高度' });
  const resultArea = h('div', { class: 'result-area' });
  const resultTabs = h('div', { class: 'result-tabs' });
  const resultBody = h('div', { class: 'result-body' });
  resultArea.append(resultTabs, resultBody);

  root.append(toolbar, editorHost, splitter, resultArea);
  tab.el.append(root);

  tab.dbSel = dbSel;
  tab.resultTabsEl = resultTabs;
  tab.resultBodyEl = resultBody;
  tab.runBtn = runBtn;
  tab.stopBtn = stopBtn;

  tab.editor = new SQLEditor(editorHost, {
    value: tab.sql,
    onChange: (v) => { tab.sql = v; },
    onRun: (selectionOnly) => runQuery(tab, selectionOnly),
    complete: (word, value, pos) => completionsFor(tab, word, value, pos),
  });
  applyEditorFont(tab.editor);
  setTimeout(() => tab.editor.focus(), 40);

  dbSel.addEventListener('change', () => {
    tab.db = dbSel.value;
    updateStatusForTab(tab);
  });
  runBtn.addEventListener('click', () => runQuery(tab, false));
  runSelBtn.addEventListener('click', () => runQuery(tab, true));
  stopBtn.addEventListener('click', () => cancelQuery(tab));
  explainBtn.addEventListener('click', () => explainQuery(tab));
  limitSel.addEventListener('change', () => { tab.maxRows = Number(limitSel.value); });

  bindVerticalSplitter(splitter, editorHost, root);

  tab.populate = async () => {
    try {
      const sid = await ensureTabSession(tab);
      const dbs = await apiGet('/api/databases?session=' + encodeURIComponent(sid));
      tab.dbList = dbs;
      const current = tab.db || connById(tab.connId)?.database || '';
      dbSel.innerHTML = '';
      dbSel.append(h('option', { value: '' }, '（未选择数据库）'));
      for (const db of dbs) dbSel.append(h('option', { value: db.name }, db.name));
      if (current && dbs.some((d) => d.name === current)) dbSel.value = current;
      else if (dbs.length) { tab.db = dbs[0].name; dbSel.value = dbs[0].name; }
      else { tab.db = ''; }
      updateStatusForTab(tab);
    } catch (err) {
      toast(err.message, 'err', 6000);
    }
  };
  tab.populate();
  renderQueryResults(tab);
}

function applyEditorFont(editor) {
  const size = S.settings.fontSize || 13;
  const family = S.settings.fontFamily === 'serif'
    ? 'Georgia, "Times New Roman", serif'
    : S.settings.fontFamily === 'mono'
      ? 'ui-monospace, "SF Mono", Menlo, Consolas, monospace'
      : 'var(--mono)';
  editor.ta.style.fontSize = size + 'px';
  editor.pre.style.fontSize = size + 'px';
  editor.ta.style.fontFamily = family;
  editor.pre.style.fontFamily = family;
  editor.ta.style.lineHeight = '1.55';
  editor.pre.style.lineHeight = '1.55';
}

function setRunning(tab, running) {
  tab.running = running;
  tab.runBtn.disabled = running;
  tab.stopBtn.disabled = !running;
}

async function runQuery(tab, selectionOnly) {
  if (tab.running) return;
  const sql = selectionOnly ? tab.editor.getSelection().text : tab.editor.value;
  if (!sql.trim()) { toast('没有可执行的 SQL', 'warn', 2200); return; }
  await executeSQL(tab, sql);
}

async function explainQuery(tab) {
  if (tab.running) return;
  let sql = tab.editor.getSelection().text || tab.editor.value;
  sql = sql.trim();
  if (!sql) return;
  if (!/^\s*(select|with|insert|update|delete|replace)\b/i.test(sql)) {
    toast('EXPLAIN 仅适用于 SELECT / INSERT / UPDATE / DELETE', 'warn');
    return;
  }
  await executeSQL(tab, 'EXPLAIN ' + sql);
}

async function executeSQL(tab, sql) {
  setRunning(tab, true);
  const queryId = 'q' + Date.now().toString(36) + Math.random().toString(36).slice(2, 6);
  tab.queryId = queryId;
  setStatus(connById(tab.connId)?.name, tab.db, '执行中…', '');
  try {
    const res = await withTabSession(tab, (sid) => apiPost('/api/query', {
      session: sid, sql, database: tab.db || '', maxRows: tab.maxRows, queryId,
    }));
    tab.results = res.results || [];
    tab.error = res.error || '';
    tab.duration = res.duration;
    tab.activeResult = 0;
    renderQueryResults(tab);
    const rows = tab.results.reduce((a, r) => a + (r.rowCount || 0), 0);
    const affected = tab.results.reduce((a, r) => a + (r.affected || 0), 0);
    setStatus(connById(tab.connId)?.name, tab.db, `${formatMs(res.duration)} · ${rows ? rows + ' 行' : affected + ' 行受影响'}`, tab.error ? '有错误' : '');
    if (res.error) toast(res.error.split('\n')[0], 'err', 7000);
  } catch (err) {
    tab.results = [];
    tab.error = err.message;
    renderQueryResults(tab);
    setStatus(connById(tab.connId)?.name, tab.db, '执行失败', '');
    toast(err.message, 'err', 7000);
  } finally {
    setRunning(tab, false);
  }
}

async function cancelQuery(tab) {
  if (!tab.running || !tab.sessionId || !tab.queryId) return;
  try {
    await apiPost('/api/query/cancel', { session: tab.sessionId, queryId: tab.queryId });
    toast('已请求取消查询', 'info', 2200);
  } catch (err) {
    toast(err.message, 'err', 4000);
  }
}

function renderQueryResults(tab) {
  const tabsEl = tab.resultTabsEl;
  const bodyEl = tab.resultBodyEl;
  tabsEl.innerHTML = '';
  bodyEl.innerHTML = '';

  const entries = [];
  (tab.results || []).forEach((rs, i) => {
    if (rs.columns && rs.columns.length) entries.push({ kind: 'grid', rs, key: 'r' + i });
  });
  entries.push({ kind: 'msg', key: 'msg' });
  if (tab.activeResult == null || tab.activeResult >= entries.length) tab.activeResult = 0;
  tab.entries = entries;

  entries.forEach((entry, i) => {
    const btn = h('button', { class: 'result-tab' + (i === tab.activeResult ? ' active' : '') },
      entry.kind === 'grid' ? `结果 ${entry.rs.index}` : '消息',
      entry.kind === 'grid' ? h('span', { class: 'count' }, `${entry.rs.rowCount} 行`) : null,
    );
    btn.addEventListener('click', () => { tab.activeResult = i; renderQueryResults(tab); });
    tabsEl.append(btn);
  });
  tabsEl.append(h('span', { class: 'flex-1' }));

  if (tab.error) bodyEl.append(h('div', { class: 'error-banner' }, tab.error));
  const entry = entries[tab.activeResult];
  if (!entry) return;
  if (entry.kind === 'grid') {
    tabsEl.append(h('button', { class: 'btn sm', style: { marginLeft: '8px' }, title: 'Export Excel', html: icon('download') + '<span>Excel</span>', onClick: () => exportQueryXLSX(tab, entry) }));
    tabsEl.append(h('button', { class: 'btn sm', title: '查看当前单元格完整内容', html: icon('file') + '<span>详情</span>', onClick: () => {
      const sel = tab.queryGrid?.selected;
      if (!sel || sel.row < 0 || sel.col < 0) { toast('请先选择一个单元格', 'warn', 2400); return; }
      openCellDetail(tab, sel.row, sel.col);
    } }));
    const host = h('div', { style: { flex: '1', minHeight: '0' } });
    bodyEl.append(host);
    const grid = new Grid(host, { sortable: false, emptyText: '查询没有返回数据', onDetail: (row, col) => openCellDetail(tab, row, col) });
    tab.queryGrid = grid;
    grid.setData({ columns: entry.rs.columns, rows: entry.rs.rows, pk: [] });
    if (entry.rs.truncated) {
      bodyEl.append(h('div', { class: 'pager' }, h('span', {}, `已显示前 ${entry.rs.rowCount} 行（达到上限），请增加最大行数或使用 LIMIT。`)));
    }
  } else {
    bodyEl.append(renderMessages(tab));
  }
}

function renderMessages(tab) {
  const list = h('div', { class: 'message-list' });
  if (!tab.results.length && !tab.error) {
    list.append(h('div', { class: 'empty-state compact' }, h('span', { html: icon('terminal') }), h('h3', {}, '还没有执行结果'), h('p', {}, '按 ⌘↩ 运行 SQL，结果将显示在这里。')));
    return list;
  }
  tab.results.forEach((rs) => {
    const line = rs.columns?.length
      ? `${rs.rowCount} 行返回${rs.truncated ? '（截断）' : ''}`
      : (rs.message || `${rs.affected} 行受影响`);
    list.append(h('div', { class: 'message-row ok' },
      h('span', { class: 'idx' }, String(rs.index)),
      h('span', { class: 'msg', title: rs.statement }, firstLine(rs.statement) + ' — ' + line),
      h('span', { class: 'time' }, formatMs(rs.durationMs)),
    ));
  });
  if (tab.error) {
    list.append(h('div', { class: 'message-row err' }, h('span', { class: 'idx' }, '!'), h('span', { class: 'msg' }, tab.error)));
  }
  return list;
}

function firstLine(text) {
  if (!text) return '';
  const line = text.trim().split('\n')[0];
  return line.length > 110 ? line.slice(0, 110) + '…' : line;
}

async function completionsFor(tab, word, value, pos) {
  const items = [];
  const lower = word.toLowerCase();
  for (const kw of KEYWORDS) {
    if (kw.toLowerCase().startsWith(lower)) items.push({ label: kw, kind: 'keyword' });
  }
  const conn = connById(tab.connId);
  let objects = [];
  if (conn && tab.db) {
    const cs = connState(conn.id);
    if (!cs.objects.has(tab.db)) {
      try {
        const sid = await ensureTreeSession(conn.id);
        const list = await apiGet(`/api/objects?session=${encodeURIComponent(sid)}&db=${encodeURIComponent(tab.db)}`);
        cs.objects.set(tab.db, list);
      } catch { /* ignore */ }
    }
    objects = cs.objects.get(tab.db) || [];
    for (const o of objects) {
      if (o.name.toLowerCase().startsWith(lower)) items.push({ label: o.name, kind: 'table', detail: o.type === 'VIEW' ? 'view' : 'table' });
    }
  }
  for (const table of referencedTables(value)) {
    for (const col of await columnsFor(tab, table)) {
      if (col.name.toLowerCase().startsWith(lower)) items.push({ label: col.name, kind: 'column', detail: table });
    }
  }
  const seen = new Set();
  return items.filter((it) => {
    const k = it.kind + ':' + it.label;
    if (seen.has(k)) return false;
    seen.add(k);
    return true;
  });
}

function referencedTables(sql) {
  const out = new Set();
  const re = /\b(?:from|join|update|into)\s+`?([A-Za-z0-9_$]+)`?/gi;
  let m;
  while ((m = re.exec(sql)) !== null) out.add(m[1]);
  return [...out].slice(0, 4);
}

const columnCache = new Map();

async function columnsFor(tab, table) {
  const key = tab.connId + '/' + tab.db + '/' + table;
  if (columnCache.has(key)) return columnCache.get(key);
  try {
    const sid = await ensureTabSession(tab);
    const detail = await apiGet(`/api/table?session=${encodeURIComponent(sid)}&db=${encodeURIComponent(tab.db)}&table=${encodeURIComponent(table)}`);
    const cols = detail.columns || [];
    columnCache.set(key, cols);
    return cols;
  } catch {
    return [];
  }
}

function bindVerticalSplitter(splitter, editorShell, root) {
  splitter.addEventListener('mousedown', (e) => {
    e.preventDefault();
    const startY = e.clientY;
    const startH = editorShell.getBoundingClientRect().height;
    const onMove = (ev) => {
      const hNew = Math.max(90, Math.min(root.clientHeight - 140, startH + ev.clientY - startY));
      editorShell.style.height = hNew + 'px';
    };
    const onUp = () => {
      document.removeEventListener('mousemove', onMove);
      document.removeEventListener('mouseup', onUp);
    };
    document.addEventListener('mousemove', onMove);
    document.addEventListener('mouseup', onUp);
  });
}

function bindSidebarResize() {
  const grip = $('#sidebar-resizer');
  grip.addEventListener('mousedown', (e) => {
    e.preventDefault();
    const startX = e.clientX;
    const startW = $('#sidebar').getBoundingClientRect().width;
    const onMove = (ev) => {
      const w = Math.max(210, Math.min(460, startW + ev.clientX - startX));
      document.documentElement.style.setProperty('--sidebar-w', w + 'px');
    };
    const onUp = () => {
      document.removeEventListener('mousemove', onMove);
      document.removeEventListener('mouseup', onUp);
    };
    document.addEventListener('mousemove', onMove);
    document.addEventListener('mouseup', onUp);
  });
}

function bindShortcuts() {
  window.addEventListener('keydown', (e) => {
    const meta = e.metaKey || e.ctrlKey;
    if (!meta) return;
    const key = e.key.toLowerCase();
    if (key === 'enter') return;
    if (key === 't') { e.preventDefault(); newQueryFromContext(); }
    else if (key === 'n') { e.preventDefault(); openConnectionDialog(); }
    else if (key === 'w') { e.preventDefault(); const t = activeTab(); if (t) closeTab(t.id); }
    else if (key === 'k') { e.preventDefault(); openHistory(); }
    else if (key === ',') { e.preventDefault(); openSettings(); }
  });
}

function ident(name) {
  return '`' + String(name).replaceAll('`', '``') + '`';
}

/* ------------------------------------------------------------------ */
/* table tab                                                          */
/* ------------------------------------------------------------------ */

function openTable(connId, db, table) {
  const existing = S.tabs.find((t) => t.kind === 'table' && t.connId === connId && t.db === db && t.table === table);
  if (existing) { activateTab(existing.id); return existing; }
  const tab = addTab('table', { connId, db, table, title: table });
  buildTableTab(tab);
  return tab;
}

function buildTableTab(tab) {
  tab.view = 'data';
  tab.page = 0;
  tab.pageSize = S.settings.pageSize;
  tab.order = [];
  tab.filters = [];
  tab.detail = null;
  tab.rowsData = null;
  tab.loadingRows = false;
  tab.rowsError = '';
  tab.loadSeq = 0;
  tab.timestampDisplay = false;
  tab.filterOpen = true;
  tab.sortCleared = false;

  const root = h('div', { class: 'tt' });
  const title = h('div', { class: 'tt-title' },
    h('span', { class: 'name' }, tab.table),
    h('span', { class: 'path' }, tab.db),
  );
  const badges = h('div', { class: 'tt-badges' });
  const actions = h('div', { class: 'tt-actions' });
  const head = h('div', { class: 'tt-head' }, title, badges, actions);
  const subtabs = h('div', { class: 'tt-subtabs' });
  const content = h('div', { class: 'tt-content' });
  root.append(head, subtabs, content);
  tab.el.append(root);
  tab.badgesEl = badges;
  tab.actionsEl = actions;
  tab.contentEl = content;
  tab.subtabsEl = subtabs;

  for (const [key, label] of [['data', '数据'], ['structure', '结构'], ['ddl', 'DDL']]) {
    const b = h('button', { class: 'tt-subtab' + (key === tab.view ? ' active' : '') }, label);
    b.dataset.view = key;
    b.addEventListener('click', () => { tab.view = key; renderSubtabs(tab); renderTableContent(tab); });
    subtabs.append(b);
  }

  renderTableActions(tab);
  renderTableContent(tab);
  loadTableDetail(tab);
  loadTableRows(tab);
}

function renderSubtabs(tab) {
  tab.subtabsEl.querySelectorAll('.tt-subtab').forEach((b) => b.classList.toggle('active', b.dataset.view === tab.view));
}

function showDetailPanel(title, text, meta = '') {
  document.getElementById('cell-detail')?.remove();
  const body = h('pre', { class: 'detail-body' }, text || '');
  const head = h('div', { class: 'detail-head' },
    h('span', { class: 'title' }, title),
    h('span', { class: 'meta' }, meta),
    h('span', { class: 'flex-1' }),
    h('button', { class: 'btn ghost sm icon', title: '复制', html: icon('copy'), onClick: () => copyText(text || '') }),
    h('button', { class: 'btn ghost sm icon', title: '关闭', html: icon('x'), onClick: () => panel.remove() }),
  );
  const resizer = h('div', { class: 'detail-resizer' });
  const panel = h('div', { class: 'detail-panel', id: 'cell-detail' }, resizer, head, body);
  document.body.append(panel);
  resizer.addEventListener('mousedown', (e) => {
    e.preventDefault();
    const move = (ev) => { panel.style.height = Math.max(140, Math.min(window.innerHeight * 0.75, window.innerHeight - ev.clientY)) + 'px'; };
    const up = () => { window.removeEventListener('mousemove', move); window.removeEventListener('mouseup', up); };
    window.addEventListener('mousemove', move);
    window.addEventListener('mouseup', up);
  });
}

async function openCellDetail(tab, row, col) {
  const grid = tab.kind === 'query' ? tab.queryGrid : tab.grid;
  const columns = grid?.columns || [];
  const values = grid?.rows?.[row];
  if (!values || !columns[col]) return;
  const column = columns[col].name;
  let text = values[col] === null || values[col] === undefined ? '' : String(values[col]);
  let meta = '';
  const r = tab.rowsData;
  if (tab.kind === 'table' && r && r.primaryKey?.length) {
    const pk = {};
    for (const key of r.primaryKey) {
      const idx = r.columns.findIndex((c) => c.name === key);
      pk[key] = r.rows[row]?.[idx] === null ? null : String(r.rows[row]?.[idx]);
    }
    try {
      const res = await apiPost('/api/cell', { session: tab.sessionId, database: tab.db, table: tab.table, column, pk });
      text = res.null ? '' : res.value;
      meta = res.bytes ? `${formatBytes(res.bytes)} · full value` : '';
    } catch (err) {
      meta = `详情读取失败：${err.message}（显示当前行值）`;
    }
  }
  try {
    if (text.trim().startsWith('{') || text.trim().startsWith('[')) {
      text = JSON.stringify(JSON.parse(text), null, 2);
    }
  } catch { /* keep raw text */ }
  const title = `${tab.table || 'query'}.${column} · row ${row + 1}`;
  showDetailPanel(title, text || '(NULL)', meta);
}

function renderTableActions(tab) {
  const host = tab.actionsEl;
  host.innerHTML = '';
  const isView = tab.detail?.type === 'VIEW' || tab.rowsData?.type === 'VIEW';
  const refresh = h('button', { class: 'btn ghost sm icon', title: '刷新', html: icon('refresh'), onClick: () => {
    S.tableDetails.delete(tableCacheKey(tab.connId, tab.db, tab.table));
    loadTableDetail(tab); loadTableRows(tab);
  } });
  const exportBtn = h('button', { class: 'btn sm', title: '导出 CSV', html: icon('download') + '<span>CSV</span>', onClick: () => exportTable(tab) });
  const xlsxBtn = h('button', { class: 'btn sm', title: '导出 Excel', html: icon('download') + '<span>Excel</span>', onClick: () => exportTableXLSX(tab) });
  const tsBtn = h('button', { class: 'btn sm', title: '时间戳转可读时间（不修改原始数据）', html: icon('clock') + '<span>时间</span>', onClick: () => {
    tab.timestampDisplay = !tab.timestampDisplay;
    tsBtn.classList.toggle('primary', !!tab.timestampDisplay);
    renderTableContent(tab);
  } });
  tsBtn.classList.toggle('primary', !!tab.timestampDisplay);
  const detailBtn = h('button', { class: 'btn sm', title: '查看当前单元格完整内容', html: icon('file') + '<span>详情</span>', onClick: () => {
    const sel = tab.grid?.selected;
    if (!sel || sel.row < 0 || sel.col < 0) { toast('请先选择一个单元格', 'warn', 2400); return; }
    openCellDetail(tab, sel.row, sel.col);
  } });
  const whereBtn = h('button', { class: 'btn sm', title: 'WHERE 条件（字段可手动输入）', html: icon('filter') + `<span>WHERE${tab.filters?.length ? ' ' + tab.filters.length : ''}</span>`, onClick: () => {
    tab.filterOpen = !tab.filterOpen;
    renderTableActions(tab);
    renderTableContent(tab);
  } });
  whereBtn.classList.toggle('primary', !!tab.filterOpen);
  host.append(refresh, whereBtn, exportBtn, xlsxBtn, tsBtn, detailBtn);
  if (!isView) {
    host.append(
      h('button', { class: 'btn sm', title: '插入行', html: icon('plus') + '<span>插入</span>', onClick: () => addRowDialog(tab) }),
      h('button', { class: 'btn sm', title: '根据选中行生成 SQL', html: icon('code') + '<span>SQL</span>', onClick: () => {
        const row = tab.grid?.getSelectedRow();
        if (row < 0) { toast('请先选择一行', 'warn'); return; }
        generateRowSQL(tab, row, 'insert');
      } }),
      h('button', { class: 'btn sm', title: '导入 Excel', html: icon('upload') + '<span>导入</span>', onClick: () => openImportWizard(tab) }),
      h('button', { class: 'btn sm', title: '删除选中行', html: icon('trash'), onClick: () => deleteSelectedRow(tab) }),
    );
  }
}

async function loadTableDetail(tab) {
  const key = tableCacheKey(tab.connId, tab.db, tab.table);
  if (S.tableDetails.has(key)) {
    tab.detail = S.tableDetails.get(key);
    renderTableHead(tab);
    renderTableActions(tab);
    renderTableContent(tab);
    return;
  }
  try {
    const detail = await withTabSession(tab, (sid) => apiGet(`/api/table?session=${encodeURIComponent(sid)}&db=${encodeURIComponent(tab.db)}&table=${encodeURIComponent(tab.table)}`));
    S.tableDetails.set(key, detail);
    tab.detail = detail;
    renderTableHead(tab);
    renderTableActions(tab);
    renderTableContent(tab);
  } catch (err) {
    toast(err.message, 'err', 6000);
  }
}

function renderTableHead(tab) {
  const d = tab.detail;
  const r = tab.rowsData;
  const type = d?.type || r?.type || 'TABLE';
  const engine = d?.engine || r?.engine || '';
  const rows = d?.rows ?? null;
  const comment = d?.comment || r?.comment || '';
  tab.badgesEl.innerHTML = '';
  tab.badgesEl.append(h('span', { class: 'badge ' + (type === 'VIEW' ? 'view' : 'table') }, type === 'VIEW' ? 'VIEW' : 'TABLE'));
  if (engine) tab.badgesEl.append(h('span', { class: 'badge engine' }, engine));
  if (rows) tab.badgesEl.append(h('span', { class: 'badge engine' }, '~' + formatNumber(rows) + ' 行'));
  if (comment) tab.badgesEl.append(h('span', { class: 'badge engine', title: comment }, comment.length > 18 ? comment.slice(0, 18) + '…' : comment));
}

async function loadTableRows(tab) {
  const seq = ++tab.loadSeq;
  tab.loadingRows = true;
  tab.rowsError = '';
  if (!tab.rowsData) renderTableContent(tab);
  try {
    const body = {
      database: tab.db,
      table: tab.table,
      limit: tab.pageSize,
      offset: tab.page * tab.pageSize,
      orderBy: tab.order,
      filters: tab.filters,
      count: tab.page === 0,
      defaultSort: !!S.settings.defaultSort && !(tab.order?.length) && !tab.sortCleared,
    };
    const res = await withTabSession(tab, (sid) => apiPost('/api/rows', { ...body, session: sid }));
    if (seq !== tab.loadSeq) return;
    tab.rowsData = res;
    if (!tab.order?.length && !tab.sortCleared && S.settings.defaultSort && res.primaryKey?.length) {
      tab.order = res.primaryKey.map((c) => ({ column: c, desc: true }));
    }
    if (res.total !== null && res.total !== undefined) tab.total = res.total;
    renderTableHead(tab);
    renderTableActions(tab);
    setStatus(connById(tab.connId)?.name, tab.db, `${formatMs(res.durationMs)} · ${formatNumber(res.rows.length)} 行`,
      tab.total !== undefined && tab.total !== null ? `共 ${formatNumber(tab.total)} 行` : '');
  } catch (err) {
    if (seq !== tab.loadSeq) return;
    tab.rowsError = err.message;
    toast(err.message, 'err', 6000);
  } finally {
    if (seq === tab.loadSeq) {
      tab.loadingRows = false;
      renderTableContent(tab);
    }
  }
}

function renderTableContent(tab) {
  tab.contentEl.innerHTML = '';
  if (tab.view === 'data') renderTableData(tab);
  else if (tab.view === 'structure') renderTableStructure(tab);
  else renderTableDDL(tab);
}

function renderTableData(tab) {
  const r = tab.rowsData;
  if (tab.loadingRows && !r) {
    tab.contentEl.append(h('div', { class: 'loading-row' }, h('span', { class: 'spinner' }), '加载数据…'));
    return;
  }
  if (!r) {
    tab.contentEl.append(h('div', { class: 'empty-state compact' },
      h('span', { html: icon('table') }),
      h('h3', {}, tab.rowsError ? '加载失败' : '没有数据'),
      h('p', {}, tab.rowsError || '这张表暂时没有可显示的行。')));
    return;
  }
  if (tab.filterOpen) tab.contentEl.append(h('div', { class: 'filter-wrap' }, buildFilterEditor(tab)));
  const qline = buildQueryLine(tab);
  if (qline) tab.contentEl.append(qline);

  const host = h('div', { style: { flex: '1', minHeight: '0' } });
  tab.contentEl.append(host);
  const editable = r.primaryKey && r.primaryKey.length > 0 && r.type !== 'VIEW';
  const grid = new Grid(host, {
    sortable: true,
    emptyText: '没有行',
    onSort: (col, dir) => {
      tab.order = dir ? [{ column: col, desc: dir === 'desc' }] : [];
      tab.sortCleared = !dir;
      tab.page = 0;
      loadTableRows(tab);
    },
    onEdit: (row, col, value) => updateCell(tab, row, col, value),
    onDeleteRow: (row) => deleteRow(tab, row),
    onGenerateSQL: (row, kind) => generateRowSQL(tab, row, kind),
    onAskAI: (row) => askAIForRow(tab, row),
    onDetail: (row, col) => openCellDetail(tab, row, col),
    canEdit: () => editable,
  });
  tab.grid = grid;
  const comments = new Map((tab.detail?.columns || []).map((c) => [c.name, c.comment || '']));
  const columns = r.columns.map((c) => ({ ...c, comment: comments.get(c.name) || c.comment || '' }));
  grid.setData({
    columns,
    rows: r.rows,
    pk: r.primaryKey || [],
    sort: tab.order[0] ? { column: tab.order[0].column, desc: tab.order[0].desc } : null,
    editable,
    emptyText: '没有行',
    timestampDisplay: !!tab.timestampDisplay,
  });
  tab.contentEl.append(buildPager(tab));
}

function mysqlQuote(v) {
  return "'" + String(v).replaceAll('\\', '\\\\').replaceAll("'", "\\'") + "'";
}

function effectiveTableSQL(tab) {
  const parts = [];
  const f = tab.filters || [];
  for (let i = 0; i < f.length; i++) {
    const x = f[i];
    if (!x.column) continue;
    const col = ident(x.column);
    let cond = '';
    const op = x.op;
    if (op === 'is null' || op === 'is not null') {
      cond = `${col} ${op.toUpperCase()}`;
    } else if (op === 'in' || op === 'not in') {
      const vals = String(x.value || '').split(',').map((v) => v.trim()).filter(Boolean).map(mysqlQuote);
      if (!vals.length) continue;
      cond = `${col} ${op.toUpperCase()} (${vals.join(', ')})`;
    } else if (op === 'between' || op === 'not between') {
      if (!x.value || !x.value2) continue;
      cond = `${col} ${op.toUpperCase()} ${mysqlQuote(x.value)} AND ${mysqlQuote(x.value2)}`;
    } else if (op === 'contains' || op === 'starts' || op === 'ends') {
      const raw = String(x.value || '');
      if (!raw) continue;
      const val = op === 'contains' ? `%${raw}%` : op === 'starts' ? `${raw}%` : `%${raw}`;
      cond = `${col} LIKE ${mysqlQuote(val)}`;
    } else {
      if (String(x.value || '').trim() === '') continue;
      cond = `${col} ${op} ${mysqlQuote(x.value)}`;
    }
    parts.push((i > 0 ? (x.connector === 'or' ? ' OR ' : ' AND ') : '') + cond);
  }
  let sql = `SELECT * FROM ${ident(tab.db)}.${ident(tab.table)}`;
  if (parts.length) sql += ' WHERE ' + parts.join('');
  if (tab.order?.length) sql += ' ORDER BY ' + tab.order.map((o) => `${ident(o.column)} ${o.desc ? 'DESC' : 'ASC'}`).join(', ');
  sql += ` LIMIT ${tab.pageSize} OFFSET ${tab.page * tab.pageSize}`;
  return sql;
}

function buildQueryLine(tab) {
  if (!tab.db || !tab.table) return null;
  const sql = effectiveTableSQL(tab);
  return h('div', { class: 'query-line' },
    h('span', { class: 'ql-label' }, 'SQL'),
    h('span', { class: 'ql-text', title: sql }, sql),
    h('button', { class: 'btn ghost sm icon', title: '复制查询', html: icon('copy'), onClick: () => copyText(sql) }),
  );
}

function buildFilterEditor(tab) {
  const cols = (tab.detail?.columns || tab.rowsData?.columns || []).map((c) => c.name);
  if (!cols.length) return h('div', { class: 'filter-editor' }, h('div', { class: 'faint fb-empty' }, '表结构尚未加载'));
  const draft = {
    filters: (tab.filters || []).map((f) => ({ ...f })),
    order: (tab.order || []).map((o) => ({ ...o })),
  };
  const condBox = h('div', { class: 'fb-list' });
  const sortBox = h('div', { class: 'fb-list' });
  const ops = [['=', '='], ['!=', '≠'], ['>', '>'], ['>=', '≥'], ['<', '<'], ['<=', '≤'],
    ['contains', '包含'], ['starts', '开头是'], ['ends', '结尾是'],
    ['in', 'IN（逗号分隔）'], ['not in', 'NOT IN'], ['between', 'BETWEEN'], ['not between', 'NOT BETWEEN'],
    ['is null', '为空'], ['is not null', '不为空']];
  const needsValue = (op) => !['is null', 'is not null'].includes(op);

  const renderConditions = () => {
    condBox.innerHTML = '';
    if (!draft.filters.length) {
      condBox.append(h('div', { class: 'faint fb-empty' }, '还没有筛选条件，点击「添加条件」，或从表头拖入字段。'));
      return;
    }
    draft.filters.forEach((f, i) => {
      const colSel = h('input', { class: 'fb-select fb-col-input', title: '字段（可输入搜索）', list: 'filter-fields-' + tab.id, value: f.column, placeholder: '字段', spellcheck: 'false' });
      colSel.addEventListener('input', () => { f.column = colSel.value.trim(); });
      const opSel = h('select', { class: 'fb-select', title: '运算符' }, ...ops.map(([v, l]) => h('option', { value: v }, l)));
      opSel.value = f.op;
      const connector = h('select', { class: 'fb-select fb-connector', title: '与上一条件的连接' },
        h('option', { value: 'and' }, 'AND'), h('option', { value: 'or' }, 'OR'));
      connector.value = f.connector === 'or' ? 'or' : 'and';
      connector.addEventListener('change', () => { f.connector = connector.value; });
      const value1 = h('input', { class: 'fb-input', placeholder: '值', spellcheck: 'false' });
      value1.value = f.value || '';
      value1.addEventListener('input', () => { f.value = value1.value; });
      const value2 = h('input', { class: 'fb-input', placeholder: '第二个值', spellcheck: 'false' });
      value2.value = f.value2 || '';
      value2.addEventListener('input', () => { f.value2 = value2.value; });
      const values = h('div', { class: 'fb-values' }, value1, value2);
      const syncInputs = () => {
        value1.disabled = !needsValue(opSel.value);
        value2.classList.toggle('hidden', !['between', 'not between'].includes(opSel.value));
        value1.placeholder = ['in', 'not in'].includes(opSel.value) ? '值1, 值2, …' : '值';
      };
      opSel.addEventListener('change', () => { f.op = opSel.value; syncInputs(); });
      syncInputs();
      const rm = h('button', { class: 'btn ghost sm icon', title: '删除条件', html: icon('x') });
      rm.addEventListener('click', () => { draft.filters.splice(i, 1); renderConditions(); });
      condBox.append(h('div', { class: 'fb-row' },
        i > 0 ? connector : h('span', { class: 'fb-connector faint' }, 'WHERE'),
        colSel, opSel, values, rm));
    });
  };

  const renderSorts = () => {
    sortBox.innerHTML = '';
    draft.order.forEach((o, i) => {
      const colSel = h('select', { class: 'fb-select' }, ...cols.map((c) => h('option', { value: c }, c)));
      colSel.value = o.column;
      colSel.addEventListener('change', () => { o.column = colSel.value; });
      const dir = h('select', { class: 'fb-select' }, h('option', { value: 'asc' }, '升序 ASC'), h('option', { value: 'desc' }, '降序 DESC'));
      dir.value = o.desc ? 'desc' : 'asc';
      dir.addEventListener('change', () => { o.desc = dir.value === 'desc'; });
      const up = h('button', { class: 'btn ghost sm icon', title: '上移', html: icon('arrowup'), disabled: i === 0 });
      up.addEventListener('click', () => { [draft.order[i - 1], draft.order[i]] = [draft.order[i], draft.order[i - 1]]; renderSorts(); });
      const rm = h('button', { class: 'btn ghost sm icon', title: '删除排序', html: icon('x') });
      rm.addEventListener('click', () => { draft.order.splice(i, 1); renderSorts(); });
      sortBox.append(h('div', { class: 'fb-row' }, h('span', { class: 'fb-connector faint' }, i === 0 ? 'ORDER BY' : '然后'), colSel, dir, up, rm));
    });
    if (!draft.order.length) sortBox.append(h('div', { class: 'faint fb-empty' }, '未设置排序；默认按主键升序分页。'));
  };

  const addCond = h('button', { class: 'btn sm', html: icon('plus') + '<span>添加条件</span>' });
  addCond.addEventListener('click', () => { draft.filters.push({ column: cols[0], op: '=', value: '', value2: '', connector: 'and' }); renderConditions(); });
  const addSort = h('button', { class: 'btn sm', html: icon('plus') + '<span>添加排序</span>' });
  addSort.addEventListener('click', () => {
    if (draft.order.length >= 3) { toast('最多 3 个排序字段', 'warn', 2000); return; }
    draft.order.push({ column: cols[0], desc: false }); renderSorts();
  });
  const close = () => { tab.filterOpen = false; renderTableActions(tab); renderTableContent(tab); };
  const reset = h('button', { class: 'btn', onClick: () => { draft.filters = []; draft.order = []; renderConditions(); renderSorts(); } }, '重置');
  const cancel = h('button', { class: 'btn', onClick: close }, '取消');
  const apply = h('button', { class: 'btn primary', onClick: () => {
    const filters = [];
    for (const f of draft.filters) {
      f.column = String(f.column || '').trim();
      if (!f.column) continue;
      if (!cols.includes(f.column)) { toast(`没有字段「${f.column}」`, 'warn', 2400); return; }
      const value = String(f.value || '').trim();
      const value2 = String(f.value2 || '').trim();
      if (f.op === 'is null' || f.op === 'is not null') {
        filters.push({ column: f.column, op: f.op, value: '', value2: '', connector: filters.length ? (f.connector === 'or' ? 'or' : 'and') : 'and' });
        continue;
      }
      if (['in', 'not in'].includes(f.op)) {
        if (value.split(',').every((v) => !v.trim())) continue;
      } else if (['between', 'not between'].includes(f.op)) {
        if (!value && !value2) continue;
        if (!value || !value2) { toast(`条件「${f.column}」需要两个值`, 'warn', 2400); return; }
      } else if (!value) continue;
      filters.push({ column: f.column, op: f.op, value: f.value || '', value2: f.value2 || '', connector: filters.length ? (f.connector === 'or' ? 'or' : 'and') : 'and' });
    }
    tab.filters = filters;
    tab.order = draft.order.filter((o) => o.column);
    tab.sortCleared = tab.order.length === 0;
    tab.page = 0;
    tab.filterOpen = false;
    renderTableActions(tab);
    renderTableContent(tab);
    loadTableRows(tab);
  } }, '应用');

  const editor = h('div', { class: 'filter-editor' },
    h('datalist', { id: 'filter-fields-' + tab.id }, ...cols.map((c) => h('option', { value: c }))),
    h('div', { class: 'fb-merged' },
      h('div', { class: 'fb-col' }, h('div', { class: 'fb-title' }, '筛选条件 / WHERE', h('span', { class: 'spacer' }), addCond), condBox),
      h('div', { class: 'fb-col' }, h('div', { class: 'fb-title' }, '排序 / ORDER BY', h('span', { class: 'spacer' }), addSort), sortBox),
    ),
    h('div', { class: 'fb-footer' }, reset, h('span', { class: 'spacer' }), cancel, apply),
  );
  renderConditions();
  renderSorts();
  return editor;
}

function opLabel(op) {
  return { contains: '包含', starts: '开头是', ends: '结尾是', 'is null': '为空', 'is not null': '不为空', '!=': '≠', '>=': '≥', '<=': '≤' }[op] || op;
}

function buildPager(tab) {
  const r = tab.rowsData;
  const offset = tab.page * tab.pageSize;
  const shown = r?.rows.length || 0;
  const pager = h('div', { class: 'pager' });
  const prev = h('button', { class: 'btn ghost sm icon', title: '上一页', html: icon('chevron-left'), disabled: tab.page === 0, onClick: () => { tab.page = Math.max(0, tab.page - 1); loadTableRows(tab); } });
  const next = h('button', { class: 'btn ghost sm icon', title: '下一页', html: icon('chevron-right'), disabled: !(r?.truncated), onClick: () => { tab.page++; loadTableRows(tab); } });
  const sizeSel = h('select', { title: '每页行数' }, ...[50, 100, 200, 500, 1000].map((n) => h('option', { value: String(n) }, `${n} 行/页`)));
  sizeSel.value = String(tab.pageSize);
  sizeSel.addEventListener('change', () => { tab.pageSize = Number(sizeSel.value); tab.page = 0; loadTableRows(tab); });
  pager.append(prev, next);
  pager.append(h('span', {}, `${shown ? formatNumber(offset + 1) : 0}–${formatNumber(offset + shown)}${tab.total !== undefined && tab.total !== null ? ' / ' + formatNumber(tab.total) : ''}`));
  pager.append(h('span', { class: 'spacer' }));
  if (tab.filters.length) pager.append(h('span', { class: 'faint' }, `${tab.filters.length} 个筛选`));
  pager.append(h('span', { class: 'faint' }, formatMs(r?.durationMs) + ' · '), sizeSel);
  return pager;
}

async function updateCell(tab, row, col, value) {
  const r = tab.rowsData;
  const rowValues = r.rows[row];
  const pk = {};
  for (const key of r.primaryKey || []) {
    const idx = r.columns.findIndex((c) => c.name === key);
    pk[key] = rowValues[idx] === null ? null : String(rowValues[idx]);
  }
  const column = r.columns[col].name;
  const res = await apiPost('/api/rows/update', {
    session: tab.sessionId,
    database: tab.db,
    table: tab.table,
    pk,
    changes: { [column]: value },
  });
  if (!res.affected) throw new Error('没有行被更新（数据可能已被修改）');
  toast('已更新 1 行', 'ok', 1400);
}

async function deleteRow(tab, row) {
  const r = tab.rowsData;
  const rowValues = r.rows[row];
  const pk = {};
  for (const key of r.primaryKey || []) {
    const idx = r.columns.findIndex((c) => c.name === key);
    pk[key] = rowValues[idx] === null ? null : String(rowValues[idx]);
  }
  const pkText = Object.entries(pk).map(([k, v]) => `${k}=${v === null ? 'NULL' : v}`).join(', ');
  const ok = await confirmDialog({
    title: '删除行',
    message: `确定删除 <code>${escapeHtml(tab.table)}</code> 中主键为 <code>${escapeHtml(pkText)}</code> 的行？此操作不可撤销。`,
    confirmText: '删除',
    danger: true,
  });
  if (!ok) return;
  try {
    const res = await apiPost('/api/rows/delete', { session: tab.sessionId, database: tab.db, table: tab.table, pk });
    toast(`已删除 ${res.affected} 行`, 'ok');
    if (tab.rowsData.rows.length === 1 && tab.page > 0) tab.page--;
    loadTableRows(tab);
  } catch (err) {
    toast(err.message, 'err', 6000);
  }
}

async function deleteSelectedRow(tab) {
  if (!tab.grid) { toast('请先在数据页选择一行', 'warn'); return; }
  const row = tab.grid.getSelectedRow();
  if (row < 0) { toast('请先选择一行', 'warn'); return; }
  await deleteRow(tab, row);
}

async function generateRowSQL(tab, row, kind) {
  const r = tab.rowsData;
  if (!r || !r.rows || !r.rows[row]) { toast('没有可用的行数据', 'err'); return; }
  const values = {};
  for (let i = 0; i < r.columns.length; i++) {
    const v = r.rows[row][i];
    values[r.columns[i].name] = v === null ? null : String(v);
  }
  const pk = {};
  for (const key of r.primaryKey || []) {
    const idx = r.columns.findIndex((c) => c.name === key);
    if (idx >= 0) {
      const v = r.rows[row][idx];
      pk[key] = v === null ? null : String(v);
    }
  }
  if ((kind === 'update' || kind === 'delete') && !Object.keys(pk).length) {
    toast('该表没有主键，无法生成安全的 UPDATE / DELETE', 'err', 4000);
    return;
  }
  try {
    await ensureTabSession(tab);
    const res = await apiPost('/api/rows/sql', {
      session: tab.sessionId, database: tab.db, table: tab.table, kind, values, pk,
    });
    showSQLPreview(tab, res.sql, kind);
  } catch (err) {
    toast(err.message, 'err', 6000);
  }
}

function showSQLPreview(tab, sql, kind) {
  const body = h('div', { class: 'sql-preview' },
    h('div', { class: 'faint', style: { fontSize: '11.5px', marginBottom: '8px' } },
      `${kind.toUpperCase()} · ${tab.db}.${tab.table} · 由当前行生成，请核对后再执行`),
    h('pre', { class: 'ddl' }, sql));
  const m = openModal({ title: '生成的 SQL', subtitle: `${tab.db}.${tab.table}`, body, wide: true });
  const copy = h('button', { class: 'btn sm', html: icon('copy') + '<span>复制</span>' });
  copy.addEventListener('click', () => copyText(sql));
  const open = h('button', { class: 'btn primary sm', html: icon('terminal') + '<span>在查询中打开</span>' });
  open.addEventListener('click', () => { m.close(); openQuery(tab.connId, tab.db, sql); });
  const close = h('button', { class: 'btn sm' }, '关闭');
  close.addEventListener('click', () => m.close());
  m.footer.append(h('span', { class: 'spacer' }), close, copy, open);
}

async function withConnSession(connId, fn) {
  const cs = connState(connId);
  if (cs.sessionId) return fn(cs.sessionId);
  const v = await openSession(connId);
  try {
    return await fn(v.id);
  } finally {
    apiPost('/api/session/close', { session: v.id }).catch(() => {});
  }
}

async function exportTableFromTreeXLSX(connId, db, table) {
  try {
    await withConnSession(connId, async (sid) => {
      xlsxExportDialog({
        title: '导出 Excel',
        sql: `SELECT * FROM \`${db}\`.\`${table}\``,
        onConfirm: async (includeSQL) => {
          try {
            await postXLSX({ kind: 'table', session: sid, database: db, table, includeSQL }, `${db}.${table}.xlsx`);
            toast('已导出 Excel', 'ok');
          } catch (err) { toast(err.message, 'err', 6000); }
        },
      });
    });
  } catch (err) { toast(err.message, 'err', 6000); }
}

async function exportTableFromTree(connId, db, table) {
  try {
    await withConnSession(connId, async (sid) => {
      const res = await fetch('/api/export', {
        method: 'POST',
        headers: { 'X-NavMini-Token': TOKEN, 'Content-Type': 'application/json' },
        body: JSON.stringify({ session: sid, database: db, table, orderBy: [], filters: [] }),
      });
      if (!res.ok) {
        let msg = res.statusText;
        try { msg = (await res.json()).error || msg; } catch { /* ignore */ }
        throw new Error(msg);
      }
      const blob = await res.blob();
      downloadBlob(blob, `${db}.${table}.csv`);
    });
    toast('已导出 CSV', 'ok');
  } catch (err) {
    toast(err.message, 'err', 6000);
  }
}

async function openDatabaseExport(connId, dbName) {
  const conn = connById(connId);
  if (!conn) { toast('连接不存在', 'err'); return; }
  let objects = [];
  try {
    const sid = await ensureTreeSession(connId);
    objects = await apiGet(`/api/objects?session=${encodeURIComponent(sid)}&db=${encodeURIComponent(dbName)}`);
  } catch (err) { toast(err.message, 'err', 6000); return; }
  const tables = (objects || []).filter((o) => o.type !== 'VIEW');
  const mode = h('select', { class: 'input' },
    h('option', { value: 'structure' }, '仅结构'),
    h('option', { value: 'full' }, '结构 + 全部数据'),
    h('option', { value: 'custom' }, '自定义：全部结构，指定表数据'));
  const custom = h('div', { class: 'export-table-list hidden' });
  for (const t of tables) {
    const cb = h('input', { type: 'checkbox', checked: true });
    cb.value = t.name;
    custom.append(h('label', { class: 'check' }, cb, h('span', {}, t.name)));
  }
  mode.addEventListener('change', () => custom.classList.toggle('hidden', mode.value !== 'custom'));
  const m = openModal({ title: '快速导出数据库', subtitle: dbName, body: h('div', { class: 'form-grid' },
    field('导出内容', mode),
    h('div', { class: 'field wide' }, h('label', {}, '指定表数据（仅自定义模式）'), custom),
  ), wide: true });
  const run = h('button', { class: 'btn primary' }, '导出 .sql');
  run.addEventListener('click', async () => {
    const dataTables = [...custom.querySelectorAll('input:checked')].map((x) => x.value);
    run.disabled = true;
    try {
      const sid = await ensureTreeSession(connId);
      const res = await fetch('/api/database/export', {
        method: 'POST',
        headers: { 'X-NavMini-Token': TOKEN, 'Content-Type': 'application/json' },
        body: JSON.stringify({ session: sid, database: dbName, mode: mode.value, dataTables }),
      });
      if (!res.ok) {
        let msg = res.statusText;
        try { msg = (await res.json()).error || msg; } catch { /* ignore */ }
        throw new Error(msg);
      }
      downloadBlob(await res.blob(), `${dbName}-${mode.value}.sql`);
      toast('已导出 SQL', 'ok');
      m.close();
    } catch (err) { toast(err.message, 'err', 7000); }
    finally { run.disabled = false; }
  });
  m.footer.append(h('span', { class: 'spacer' }), h('button', { class: 'btn', onClick: () => m.close() }, '取消'), run);
}

function backupTableFromTree(connId, db, table) {
  const stamp = new Date().toISOString().replace(/[-:T]/g, '').slice(0, 14);
  const target = h('input', { class: 'input', value: `${table}_bak_${stamp}`, spellcheck: 'false' });
  const mode = h('select', { class: 'input' },
    h('option', { value: 'full' }, '结构 + 数据（默认）'),
    h('option', { value: 'structure' }, '仅结构'),
    h('option', { value: 'data' }, '仅数据（目标表需已存在）'));
  const note = h('p', { class: 'faint', style: { fontSize: '11.5px', lineHeight: '1.7', margin: '4px 0 0' } },
    '使用 CREATE TABLE … LIKE 保留索引与引擎；MySQL 不会复制外键定义。目标表已存在时会拒绝覆盖。');
  const form = h('div', {},
    h('div', { class: 'field' }, h('label', {}, '目标表名'), target),
    h('div', { class: 'field' }, h('label', {}, '备份内容'), mode),
    note);
  const m = openModal({ title: '快速备份到当前库', subtitle: `${db}.${table}`, body: form });
  const cancel = h('button', { class: 'btn' }, '取消');
  cancel.addEventListener('click', () => m.close());
  const run = h('button', { class: 'btn primary' }, '开始备份');
  run.addEventListener('click', async () => {
    const name = target.value.trim();
    if (!/^[A-Za-z0-9_]{1,64}$/.test(name)) { toast('表名只能包含字母、数字、下划线（≤64）', 'warn'); return; }
    if (name === table) { toast('目标表不能与源表同名', 'warn'); return; }
    run.disabled = true;
    run.textContent = '备份中…';
    try {
      const res = await withConnSession(connId, (sid) => apiPost('/api/table/backup', {
        session: sid, database: db, table, target: name, mode: mode.value,
      }));
      toast(`备份完成：${name} · ${formatNumber(res.rows)} 行`, 'ok', 4000);
      m.close();
      refreshDbObjects(connId, db);
    } catch (err) {
      toast(err.message, 'err', 7000);
      run.disabled = false;
      run.textContent = '开始备份';
    }
  });
  m.footer.append(h('span', { class: 'spacer' }), cancel, run);
}

async function refreshDbObjects(connId, db) {
  const cs = connState(connId);
  cs.objects.delete(db);
  if (!cs.openDbs.has(db)) { renderTree(); return; }
  try {
    const sid = await ensureTreeSession(connId);
    const objs = await apiGet(`/api/objects?session=${encodeURIComponent(sid)}&db=${encodeURIComponent(db)}`);
    cs.objects.set(db, objs);
  } catch (err) {
    toast(err.message, 'err', 5000);
  }
  renderTree();
}

async function addRowDialog(tab) {
  if (!tab.detail) {
    try { await loadTableDetail(tab); } catch { /* handled */ }
  }
  if (!tab.detail) { toast('表结构尚未加载', 'err'); return; }
  const cols = tab.detail.columns.filter((c) => !c.autoIncrement && !c.generated);
  const form = h('div', { class: 'form-grid' });
  const inputs = new Map();
  for (const c of cols) {
    const input = h('input', { class: 'input', value: c.default ?? '', placeholder: c.nullable ? 'NULL' : '', spellcheck: 'false' });
    inputs.set(c.name, input);
    form.append(h('div', { class: 'field' + (c.dataType.includes('text') || c.dataType.includes('json') ? ' span2' : '') },
      h('label', { title: c.columnType }, c.name + ' · ' + c.dataType),
      input,
      h('span', { class: 'hint' }, (c.nullable ? 'NULL 允许' : 'NOT NULL') + (c.default !== null && c.default !== undefined ? ' · 默认 ' + c.default : '')),
    ));
  }
  const m = openModal({ title: '插入行', subtitle: tab.db + '.' + tab.table, body: form, wide: true });
  const save = h('button', { class: 'btn primary' }, '插入');
  const cancel = h('button', { class: 'btn' }, '取消');
  cancel.addEventListener('click', () => m.close());
  save.addEventListener('click', async () => {
    const values = {};
    for (const [name, input] of inputs) {
      const col = cols.find((c) => c.name === name);
      const raw = input.value;
      if (raw === '' && col.default !== null && col.default !== undefined) continue;
      values[name] = raw === '' && col.nullable ? null : raw;
    }
    try {
      save.disabled = true;
      const res = await apiPost('/api/rows/insert', { session: tab.sessionId, database: tab.db, table: tab.table, values });
      toast('已插入 1 行' + (res.insertId ? '（id=' + res.insertId + '）' : ''), 'ok');
      m.close();
      loadTableRows(tab);
    } catch (err) {
      toast(err.message, 'err', 6000);
      save.disabled = false;
    }
  });
  m.footer.append(h('span', { class: 'spacer' }), cancel, save);
  setTimeout(() => form.querySelector('input')?.focus(), 50);
}

function xlsxExportDialog({ title, sql, onConfirm }) {
  const include = h('input', { type: 'checkbox' });
  const body = h('div', {},
    h('p', { class: 'modal-msg' }, 'The first sheet contains column headers and data. DATETIME/TIMESTAMP/DATE/TIME values are written as text.'),
    sql ? h('pre', { class: 'sql-preview' }, sql) : null,
    h('label', { class: 'check', style: { marginTop: '12px' } }, include, h('span', {}, 'Also export the query SQL to a separate sheet')),
  );
  const m = openModal({ title, body, wide: true });
  const run = h('button', { class: 'btn primary' }, 'Export');
  run.addEventListener('click', () => { m.close(); onConfirm(include.checked); });
  m.footer.append(h('span', { class: 'spacer' }), h('button', { class: 'btn', onClick: () => m.close() }, '取消'), run);
}

async function postXLSX(payload, filename) {
  const res = await fetch('/api/export/xlsx', {
    method: 'POST',
    headers: { 'X-NavMini-Token': TOKEN, 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  });
  if (!res.ok) {
    let msg = res.statusText;
    try { msg = (await res.json()).error || msg; } catch { /* ignore */ }
    throw new Error(msg);
  }
  downloadBlob(await res.blob(), filename);
}

async function exportTableXLSX(tab) {
  const sid = await ensureTabSession(tab);
  xlsxExportDialog({
    title: '导出 Excel',
    sql: `SELECT * FROM \`${tab.db}\`.\`${tab.table}\``,
    onConfirm: async (includeSQL) => {
      try {
        await postXLSX({
          kind: 'table', session: sid, database: tab.db, table: tab.table,
          orderBy: tab.order, filters: tab.filters, includeSQL,
        }, `${tab.db}.${tab.table}.xlsx`);
        toast('已导出 Excel', 'ok');
      } catch (err) { toast(err.message, 'err', 6000); }
    },
  });
}

function exportQueryXLSX(tab, entry) {
  if (!entry || !entry.rs) return;
  const rs = entry.rs;
  if (!rs.columns?.length) { toast('没有可导出的数据', 'warn'); return; }
  xlsxExportDialog({
    title: '导出 Excel',
    sql: rs.statement || tab.sql,
    onConfirm: async (includeSQL) => {
      try {
        await postXLSX({
          kind: 'result', columns: rs.columns, rows: rs.rows, sql: rs.statement || tab.sql, includeSQL,
        }, `query-${rs.index || 1}.xlsx`);
        toast('已导出 Excel', 'ok');
      } catch (err) { toast(err.message, 'err', 6000); }
    },
  });
}

async function openImportWizard(tab, preset = null) {
  if (!tab || tab.kind !== 'table') return;
  let upload = null;
  let preview = null;
  let mapping = {};
  let headerRow = 1;
  let sheet = '';
  let busy = false;

  const fileInput = h('input', { type: 'file', accept: '.xlsx,.xlsm', style: { display: 'none' } });
  const fileBtn = h('button', { class: 'btn sm' }, '选择 Excel 文件');
  const fileName = h('span', { class: 'faint', style: { fontSize: '11px' } }, 'No file selected');
  const sheetSel = h('select', { class: 'input', disabled: true });
  const headerInput = h('input', { class: 'input', type: 'number', min: 1, value: 1, style: { width: '90px' } });
  const loadBtn = h('button', { class: 'btn sm', disabled: true }, '加载表头 / 预览');
  const status = h('div', { class: 'faint', style: { fontSize: '11px', padding: '4px 0' } });
  const mappingHost = h('div', { class: 'import-mapping' });
  const importBtn = h('button', { class: 'btn primary', disabled: true }, '开始导入');

  const m = openModal({ title: '导入 Excel', subtitle: `${tab.db}.${tab.table}`, body: h('div', {},
    h('div', { class: 'row gap-8', style: { marginBottom: '10px' } }, fileBtn, fileName, fileInput),
    h('div', { class: 'form-grid' },
      field('工作表', sheetSel),
      field('表头行', headerInput),
    ),
    h('div', { class: 'row gap-8', style: { margin: '4px 0 10px' } }, loadBtn, status),
    mappingHost,
  ), wide: true });
  m.footer.append(h('span', { class: 'spacer' }), h('button', { class: 'btn', onClick: () => m.close() }, '取消'), importBtn);

  const renderMapping = () => {
    mappingHost.innerHTML = '';
    if (!preview) return;
    if (!preview.headers.length) {
      mappingHost.append(h('div', { class: 'empty-state compact' }, h('h3', {}, '未读取到表头')));
      return;
    }
    const cols = preview.columns || [];
    const table = h('table', { class: 'mtable import-table' },
      h('thead', {}, h('tr', {},
        h('th', {}, '源表头'), h('th', {}, '预览'), h('th', {}, '目标列'),
      )));
    const tbody = h('tbody');
    preview.headers.forEach((header, i) => {
      const sample = (preview.preview || []).map((r) => r[i]).filter((v) => v !== undefined && v !== '').slice(0, 2).join(' / ');
      const sel = h('select', { class: 'input' }, h('option', { value: '' }, '— 跳过 —'));
      for (const c of cols) {
        const opt = h('option', { value: c.name }, `${c.name}  ·  ${c.columnType || c.dataType || ''}${c.nullable ? '' : ' · required'}`);
        if (c.generated) opt.disabled = true;
        sel.append(opt);
      }
      sel.value = mapping[String(i)] || '';
      sel.addEventListener('change', () => {
        if (sel.value) mapping[String(i)] = sel.value;
        else delete mapping[String(i)];
        importBtn.disabled = busy || !Object.keys(mapping).length;
      });
      tbody.append(h('tr', {},
        h('td', {}, h('span', { class: 'mono' }, header || `Column ${i + 1}`)),
        h('td', { class: 'faint', style: { maxWidth: '240px', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }, title: sample }, sample || '—'),
        h('td', {}, sel),
      ));
    });
    table.append(tbody);
    mappingHost.append(table);
    importBtn.disabled = busy || !Object.keys(mapping).length;
  };

  const loadPreview = async () => {
    if (!upload || !sheet) return;
    busy = true; loadBtn.disabled = true; status.textContent = '读取中…';
    try {
      const sid = await ensureTabSession(tab);
      preview = await apiPost('/api/import/preview', {
        uploadId: upload.uploadId, sheet, headerRow: Number(headerInput.value) || 1,
        session: sid, database: tab.db, table: tab.table,
      });
      mapping = Object.assign({}, preview.mapping || {});
      renderMapping();
      status.textContent = `${preview.rowCount || 0} rows · ${preview.headers.length} columns`;
    } catch (err) { status.textContent = ''; toast(err.message, 'err', 6000); }
    finally { busy = false; loadBtn.disabled = false; importBtn.disabled = !Object.keys(mapping).length; }
  };

  fileBtn.addEventListener('click', () => fileInput.click());
  fileInput.addEventListener('change', async () => {
    const file = fileInput.files?.[0];
    if (!file) return;
    fileName.textContent = `${file.name} · ${formatBytes(file.size)}`;
    const fd = new FormData();
    fd.append('file', file);
    try {
      const res = await fetch('/api/import/upload', { method: 'POST', headers: { 'X-NavMini-Token': TOKEN }, body: fd });
      const data = await res.json().catch(() => ({}));
      if (!res.ok) throw new Error(data.error || `HTTP ${res.status}`);
      upload = data;
      sheetSel.innerHTML = '';
      for (const sh of data.sheets || []) {
        sheetSel.append(h('option', { value: sh.name }, `${sh.name}${sh.rows ? ` (${sh.rows} rows)` : ''}`));
      }
      sheet = data.sheets?.[0]?.name || '';
      sheetSel.disabled = false;
      loadBtn.disabled = false;
      await loadPreview();
    } catch (err) { toast(err.message, 'err', 7000); }
  });
  sheetSel.addEventListener('change', () => { sheet = sheetSel.value; preview = null; mapping = {}; renderMapping(); });
  loadBtn.addEventListener('click', loadPreview);
  importBtn.addEventListener('click', async () => {
    if (busy || !upload || !Object.keys(mapping).length) return;
    busy = true; importBtn.disabled = true; const old = importBtn.textContent; importBtn.textContent = '导入中…';
    try {
      const sid = await ensureTabSession(tab);
      const res = await apiPost('/api/import/execute', {
        uploadId: upload.uploadId, sheet, headerRow: Number(headerInput.value) || 1,
        session: sid, database: tab.db, table: tab.table, mapping,
      });
      toast(`已导入 ${res.inserted} 行（扫描 ${res.scanned}，${formatMs(res.durationMs)}）`, 'ok', 6000);
      m.close();
      loadTableRows(tab);
    } catch (err) { toast(err.message, 'err', 8000); }
    finally { busy = false; importBtn.disabled = false; importBtn.textContent = old; }
  });

  if (preset && preset.uploadId) {
    upload = { uploadId: preset.uploadId, filename: preset.name || 'attachment.xlsx', sheets: preset.sheets || [] };
    fileName.textContent = preset.name || 'attachment';
    sheetSel.innerHTML = '';
    for (const sh of upload.sheets) {
      sheetSel.append(h('option', { value: sh.name }, `${sh.name}${sh.rows ? ` (${sh.rows} rows)` : ''}`));
    }
    sheet = preset.sheet || upload.sheets[0]?.name || '';
    if (sheet) sheetSel.value = sheet;
    sheetSel.disabled = false;
    loadBtn.disabled = false;
    setTimeout(loadPreview, 30);
  }
}

async function exportTable(tab) {
  try {
    const sid = await ensureTabSession(tab);
    const res = await fetch('/api/export', {
      method: 'POST',
      headers: { 'X-NavMini-Token': TOKEN, 'Content-Type': 'application/json' },
      body: JSON.stringify({
        session: sid, database: tab.db, table: tab.table,
        orderBy: tab.order, filters: tab.filters,
      }),
    });
    if (!res.ok) {
      let msg = res.statusText;
      try { msg = (await res.json()).error || msg; } catch { /* ignore */ }
      throw new Error(msg);
    }
    const blob = await res.blob();
    downloadBlob(blob, `${tab.db}.${tab.table}.csv`);
    toast('已导出 CSV', 'ok');
  } catch (err) {
    toast(err.message, 'err', 6000);
  }
}

function renderTableStructure(tab) {
  const d = tab.detail;
  if (!d) {
    tab.contentEl.append(h('div', { class: 'loading-row' }, h('span', { class: 'spinner' }), '加载表结构…'));
    return;
  }
  const wrap = h('div', { class: 'struct' });
  const info = h('div', { class: 'kv-grid' });
  const kv = (k, v) => h('div', { class: 'kv-row' }, h('span', { class: 'k' }, k), h('span', { class: 'v' }, v ?? '—'));
  info.append(
    kv('数据库', d.database), kv('类型', d.type), kv('引擎', d.engine || '—'),
    kv('估算行数', formatNumber(d.rows)), kv('数据长度', formatBytes(d.dataLength)),
    kv('索引长度', formatBytes(d.indexLength)), kv('主键', (d.primaryKey || []).join(', ') || '—'),
    kv('注释', d.comment || '—'),
  );
  wrap.append(h('h4', {}, '表信息'), info);

  const colTable = h('table', { class: 'struct-table' });
  colTable.innerHTML = `<thead><tr>
    <th>#</th><th>列名</th><th>类型</th><th>NULL</th><th>默认</th><th>键</th><th>额外</th><th>注释</th>
  </tr></thead><tbody>${d.columns.map((c) => `<tr>
    <td class="dim">${c.position}</td>
    <td class="col-name">${escapeHtml(c.name)}</td>
    <td class="type">${escapeHtml(c.columnType)}</td>
    <td>${c.nullable ? 'YES' : '<span class="dim">NO</span>'}</td>
    <td class="dim">${c.default === null || c.default === undefined ? '<i>NULL</i>' : escapeHtml(String(c.default))}</td>
    <td>${c.primaryKey ? '<span class="pill pk">PK</span>' : ''}${c.key && !c.primaryKey ? escapeHtml(c.key) : ''}</td>
    <td class="dim">${escapeHtml(c.extra || '')}</td>
    <td class="dim">${escapeHtml(c.comment || '')}</td>
  </tr>`).join('')}</tbody>`;
  wrap.append(h('h4', {}, `列 · ${d.columns.length}`), colTable);

  if (d.indexes?.length) {
    const idxTable = h('table', { class: 'struct-table' });
    idxTable.innerHTML = `<thead><tr><th>索引</th><th>唯一</th><th>类型</th><th>列</th></tr></thead><tbody>${d.indexes.map((i) => `<tr>
      <td class="col-name">${escapeHtml(i.name)}</td><td>${i.unique ? '是' : '否'}</td>
      <td class="dim">${escapeHtml(i.type || '')}</td><td class="mono">${escapeHtml(i.columns.join(', '))}</td>
    </tr>`).join('')}</tbody>`;
    wrap.append(h('h4', {}, `索引 · ${d.indexes.length}`), idxTable);
  }

  if (d.foreignKeys?.length) {
    const fkTable = h('table', { class: 'struct-table' });
    fkTable.innerHTML = `<thead><tr><th>外键</th><th>列</th><th>引用</th><th>更新</th><th>删除</th></tr></thead><tbody>${d.foreignKeys.map((f) => `<tr>
      <td class="col-name">${escapeHtml(f.name)}</td><td>${escapeHtml(f.column)}</td>
      <td class="mono">${escapeHtml(f.refSchema + '.' + f.refTable + '.' + f.refColumn)}</td>
      <td class="dim">${escapeHtml(f.onUpdate || '')}</td><td class="dim">${escapeHtml(f.onDelete || '')}</td>
    </tr>`).join('')}</tbody>`;
    wrap.append(h('h4', {}, `外键 · ${d.foreignKeys.length}`), fkTable);
  }
  tab.contentEl.append(wrap);
}

function renderTableDDL(tab) {
  const d = tab.detail;
  if (!d) {
    tab.contentEl.append(h('div', { class: 'loading-row' }, h('span', { class: 'spinner' }), '加载 DDL…'));
    return;
  }
  if (!d.ddl) {
    tab.contentEl.append(h('div', { class: 'empty-state compact' }, h('h3', {}, '无法读取 DDL'), h('p', {}, '当前账号可能没有 SHOW CREATE 权限。')));
    return;
  }
  const wrap = h('div', { class: 'ddl-wrap' });
  const actions = h('div', { class: 'ddl-actions' },
    h('button', { class: 'btn sm', html: icon('copy') + '<span>复制</span>', onClick: () => copyText(d.ddl) }),
    h('button', { class: 'btn sm', html: icon('download') + '<span>下载 .sql</span>', onClick: () => downloadText(d.ddl, `${tab.db}.${tab.table}.sql`, 'text/plain;charset=utf-8') }),
    h('button', { class: 'btn sm', html: icon('terminal') + '<span>在查询中打开</span>', onClick: () => openQuery(tab.connId, tab.db, d.ddl) }),
  );
  wrap.append(actions, h('pre', { class: 'ddl' }, d.ddl));
  tab.contentEl.append(wrap);
}

/* ------------------------------------------------------------------ */
/* monitor                                                            */
/* ------------------------------------------------------------------ */

function openMonitor() {
  const tab0 = activeTab();
  const connId = tab0?.connId || S.connections[0]?.id;
  if (!connId) { toast('请先新建一个连接', 'warn'); openConnectionDialog(); return; }
  const existing = S.tabs.find((t) => t.kind === 'monitor' && t.connId === connId);
  if (existing) { activateTab(existing.id); return existing; }
  const tab = addTab('monitor', { connId, title: '服务器监控' });
  buildMonitorTab(tab);
  return tab;
}

function buildMonitorTab(tab) {
  const conn = connById(tab.connId);
  const root = h('div', { class: 'monitor' });
  const auto = h('input', { type: 'checkbox' });
  const refreshBtn = h('button', { class: 'btn sm', html: icon('refresh') + '<span>刷新</span>' });
  const cards = h('div', { class: 'monitor-cards' });
  const procPanel = h('div', { class: 'panel' });
  const statusPanel = h('div', { class: 'panel' });
  const varPanel = h('div', { class: 'panel' });
  root.append(
    h('div', { class: 'monitor-head' },
      h('h2', {}, '服务器监控'),
      h('span', { class: 'conn-chip' }, h('span', { class: 'dot' }), conn?.name || 'MySQL'),
      h('span', { class: 'flex-1' }),
      h('label', { class: 'faint', style: { display: 'flex', alignItems: 'center', gap: '6px', fontSize: '11.5px' } }, auto, '每 5 秒自动刷新'),
      refreshBtn,
    ),
    cards, procPanel, statusPanel, varPanel,
  );
  tab.el.append(root);

  const load = async () => {
    try {
      const sid = await ensureTabSession(tab);
      const [list, status] = await Promise.all([
        apiGet('/api/processlist?session=' + encodeURIComponent(sid)),
        apiGet('/api/status?session=' + encodeURIComponent(sid)),
      ]);
      renderMonitor(tab, list, status, cards, procPanel, statusPanel, varPanel);
    } catch (err) {
      toast(err.message, 'err', 6000);
    }
  };
  refreshBtn.addEventListener('click', load);
  let timer = null;
  const setTimer = () => {
    clearInterval(timer);
    timer = auto.checked ? setInterval(load, 5000) : null;
  };
  auto.addEventListener('change', setTimer);
  tab.dispose = () => clearInterval(timer);
  load();
}

function renderMonitor(tab, list, status, cards, procPanel, statusPanel, varPanel) {
  const map = Object.fromEntries((status.status || []).map((s) => [s.name, s.value]));
  const num = (k) => Number(map[k] || 0);
  const qps = status.uptime ? (status.queries / status.uptime) : 0;
  const hit = num('Innodb_buffer_pool_read_requests') > 0
    ? (1 - num('Innodb_buffer_pool_reads') / num('Innodb_buffer_pool_read_requests')) * 100
    : 0;
  const uptime = formatUptime(status.uptime);
  cards.innerHTML = '';
  const metric = (label, value, sub) => h('div', { class: 'metric' }, h('div', { class: 'label' }, label), h('div', { class: 'value' }, value), sub ? h('div', { class: 'sub' }, sub) : null);
  cards.append(
    metric('连接数', formatNumber(num('Threads_connected')), `运行中 ${formatNumber(num('Threads_running'))}`),
    metric('查询/秒', qps.toFixed(1), `累计 ${formatNumber(status.queries)}`),
    metric('运行时间', uptime, `版本 ${tab.server?.version || '—'}`),
    metric('慢查询', formatNumber(num('Slow_queries')), 'Slow_queries'),
    metric('缓冲池命中', hit.toFixed(1) + '%', 'InnoDB buffer pool'),
    metric('流量', formatBytes(num('Bytes_received') + num('Bytes_sent')), `接收 ${formatBytes(num('Bytes_received'))}`),
  );

  procPanel.innerHTML = '';
  const phead = h('div', { class: 'panel-head' }, `进程列表 · ${list.length}`, h('span', { class: 'spacer' }));
  procPanel.append(phead);
  const pt = h('table', { class: 'mtable' });
  pt.innerHTML = `<thead><tr><th>Id</th><th>用户</th><th>主机</th><th>数据库</th><th>命令</th><th class="num">时间</th><th>状态</th><th>SQL</th><th></th></tr></thead>
    <tbody>${list.map((p) => `<tr>
      <td class="mono">${p.id}</td><td>${escapeHtml(p.user)}</td><td class="mono">${escapeHtml(p.host)}</td>
      <td>${escapeHtml(p.db || '')}</td><td>${escapeHtml(p.command)}</td>
      <td class="num">${p.time}s</td><td class="dim">${escapeHtml(p.state || '')}</td>
      <td class="sql" title="${escapeHtml(p.info || '')}">${escapeHtml(p.info || '')}</td>
      <td><button class="btn ghost sm" data-kill="${p.id}" title="终止连接">终止</button></td>
    </tr>`).join('')}</tbody>`;
  pt.querySelectorAll('[data-kill]').forEach((btn) => {
    btn.addEventListener('click', async () => {
      const id = Number(btn.dataset.kill);
      const ok = await confirmDialog({ title: '终止连接', message: `确定终止连接 <code>${id}</code>？`, confirmText: '终止', danger: true });
      if (!ok) return;
      try {
        await apiPost('/api/processlist/kill', { session: tab.sessionId, id });
        toast('已发送 KILL', 'ok');
        buildMonitorTabRefresh(tab);
      } catch (err) { toast(err.message, 'err', 5000); }
    });
  });
  procPanel.append(pt);

  const vars = Object.fromEntries((status.variables || []).map((v) => [v.name, v.value]));
  statusPanel.innerHTML = '';
  statusPanel.append(h('div', { class: 'panel-head' }, '状态变量', h('span', { class: 'spacer' }), h('span', {}, 'GLOBAL STATUS')));
  const sg = h('div', { class: 'kv-grid', style: { padding: '10px 12px' } });
  for (const s of status.status || []) sg.append(h('div', { class: 'kv-row' }, h('span', { class: 'k' }, s.name), h('span', { class: 'v' }, s.value)));
  statusPanel.append(sg);

  varPanel.innerHTML = '';
  varPanel.append(h('div', { class: 'panel-head' }, '服务器变量', h('span', { class: 'spacer' }), h('span', {}, 'SHOW VARIABLES')));
  const vg = h('div', { class: 'kv-grid', style: { padding: '10px 12px' } });
  for (const v of status.variables || []) vg.append(h('div', { class: 'kv-row' }, h('span', { class: 'k' }, v.name), h('span', { class: 'v', title: v.value }, v.value)));
  varPanel.append(vg);
}

function buildMonitorTabRefresh(tab) {
  // Reload only the process list by triggering a click on the refresh button.
  const root = tab.el.querySelector('.monitor');
  const btn = root?.querySelector('.monitor-head .btn');
  if (btn) btn.click();
}

function formatUptime(sec) {
  sec = Number(sec) || 0;
  const d = Math.floor(sec / 86400);
  const hh = Math.floor((sec % 86400) / 3600);
  const mm = Math.floor((sec % 3600) / 60);
  if (d > 0) return `${d} 天 ${hh} 时`;
  if (hh > 0) return `${hh} 时 ${mm} 分`;
  return `${mm} 分`;
}

/* ------------------------------------------------------------------ */
/* connection dialog                                                  */
/* ------------------------------------------------------------------ */

async function openExportConnections() {
  if (!S.connections.length) { toast('还没有可导出的连接', 'warn', 2500); return; }
  const selected = new Set(S.connections.map((c) => c.id));
  const list = h('div', { class: 'export-conn-list' });
  for (const c of S.connections) {
    const cb = h('input', { type: 'checkbox', checked: true });
    cb.value = c.id;
    cb.addEventListener('change', () => { if (cb.checked) selected.add(c.id); else selected.delete(c.id); });
    list.append(h('label', { class: 'check' }, cb, h('span', {}, `${c.name} · ${c.user}@${c.host}:${c.port}`)));
  }
  const include = h('input', { type: 'checkbox' });
  const m = openModal({ title: '导出连接 (.ncx)', subtitle: 'NavMini / MySQL', body: h('div', {},
    h('p', { class: 'modal-msg' }, '选择要导出的连接。导出文件可用于其他兼容客户端；默认不包含密码。'),
    list,
    h('label', { class: 'check', style: { marginTop: '12px' } }, include, h('span', {}, '包含密码（AES-128-CBC 加密）')),
    h('p', { class: 'hint' }, '包含密码的文件应按敏感凭据保管。'),
  ), wide: true });
  const run = h('button', { class: 'btn primary' }, '导出 .ncx');
  run.addEventListener('click', async () => {
    if (!selected.size) { toast('至少选择一个连接', 'warn'); return; }
    run.disabled = true;
    try {
      const res = await fetch('/api/connections/export-ncx', {
        method: 'POST',
        headers: { 'X-NavMini-Token': TOKEN, 'Content-Type': 'application/json' },
        body: JSON.stringify({ ids: [...selected], includePassword: include.checked }),
      });
      if (!res.ok) {
        let msg = res.statusText;
        try { msg = (await res.json()).error || msg; } catch { /* ignore */ }
        throw new Error(msg);
      }
      downloadBlob(await res.blob(), 'navmini-connections.ncx');
      toast('已导出 .ncx', 'ok');
      m.close();
    } catch (err) { toast(err.message, 'err', 7000); }
    finally { run.disabled = false; }
  });
  m.footer.append(h('span', { class: 'spacer' }), h('button', { class: 'btn', onClick: () => m.close() }, '取消'), run);
}

function openConnectionDialog(conn = null) {
  const editing = !!conn;
  const draft = conn ? JSON.parse(JSON.stringify(conn)) : {
    id: '', name: '', group: '', color: '#33c9a4', host: '127.0.0.1', port: 3306, user: 'root',
    database: '', charset: 'utf8mb4', sslMode: 'preferred', timeout: 10, remark: '',
    ssh: { enabled: false, host: '', port: 22, user: '', auth: 'password', keyPath: '', hasPassword: false },
  };
  draft.ssh = draft.ssh || { enabled: false, port: 22, auth: 'password', keyPath: '' };

  const name = h('input', { class: 'input', value: draft.name, placeholder: '例如：本地开发库' });
  const group = h('input', { class: 'input', value: draft.group, placeholder: '可选分组' });
  const host = h('input', { class: 'input', value: draft.host });
  const port = h('input', { class: 'input', type: 'number', value: draft.port });
  const user = h('input', { class: 'input', value: draft.user });
  const password = h('input', { class: 'input', type: 'password', placeholder: draft.hasPassword ? '已保存，留空保持不变' : '密码' });
  const database = h('input', { class: 'input', value: draft.database, placeholder: '可选，默认数据库' });
  const charset = h('input', { class: 'input', value: draft.charset || 'utf8mb4' });
  const sslMode = h('select', { class: 'input' },
    h('option', { value: 'preferred' }, 'Preferred（优先 TLS）'),
    h('option', { value: 'required' }, 'Required（强制 TLS）'),
    h('option', { value: 'skip-verify' }, 'Skip Verify（不校验证书）'),
    h('option', { value: 'disabled' }, 'Disabled（不使用 TLS）'));
  sslMode.value = draft.sslMode || 'preferred';
  const timeout = h('input', { class: 'input', type: 'number', value: draft.timeout || 10 });
  const remark = h('textarea', { class: 'input', rows: 2, placeholder: '备注' });
  remark.value = draft.remark || '';

  const form = h('div');
  form.append(h('div', { class: 'form-grid' },
    field('连接名称', name),
    field('分组', group),
    field('主机', host),
    field('端口', port),
    field('用户名', user),
    field('密码', password),
    field('默认数据库', database),
    field('字符集', charset),
    field('SSL 模式', sslMode),
    field('连接超时（秒）', timeout),
    h('div', { class: 'field span2' }, h('label', {}, '备注'), remark),
  ));

  // color row
  const colors = ['#33c9a4', '#6aa9ff', '#f0a35e', '#c792ea', '#f06a6a', '#8bd49c'];
  let chosenColor = draft.color || colors[0];
  const swatches = h('div', { class: 'color-swatches' });
  for (const c of colors) {
    const sw = h('span', { class: 'color-swatch' + (c === chosenColor ? ' active' : ''), style: { background: c }, title: c });
    sw.addEventListener('click', () => {
      chosenColor = c;
      swatches.querySelectorAll('.color-swatch').forEach((el) => el.classList.toggle('active', el === sw));
    });
    swatches.append(sw);
  }
  form.append(h('div', { class: 'field' }, h('label', {}, '标识颜色'), swatches));

  // SSH
  const sshEnabled = h('input', { type: 'checkbox', checked: !!draft.ssh.enabled });
  const sshBody = h('div');
  const sshHost = h('input', { class: 'input', value: draft.ssh.host || '', placeholder: 'ssh.example.com' });
  const sshPort = h('input', { class: 'input', type: 'number', value: draft.ssh.port || 22 });
  const sshUser = h('input', { class: 'input', value: draft.ssh.user || '' });
  const sshAuth = h('select', { class: 'input' },
    h('option', { value: 'password' }, '密码'),
    h('option', { value: 'key' }, '私钥文件'),
    h('option', { value: 'agent' }, 'ssh-agent'));
  sshAuth.value = draft.ssh.auth || 'password';
  const sshPassword = h('input', { class: 'input', type: 'password', placeholder: draft.ssh.hasPassword ? '已保存，留空保持不变' : 'SSH 密码 / 私钥口令' });
  const sshKey = h('input', { class: 'input', value: draft.ssh.keyPath || '', placeholder: '~/.ssh/id_ed25519' });
  const sshPassField = h('div', { class: 'field' }, h('label', {}, 'SSH 密码'), sshPassword);
  const sshKeyField = h('div', { class: 'field' }, h('label', {}, '私钥路径'), sshKey);
  const applyAuth = () => {
    sshPassField.classList.toggle('hidden', sshAuth.value === 'agent' || sshAuth.value === 'key');
    sshKeyField.classList.toggle('hidden', sshAuth.value !== 'key');
  };
  sshAuth.addEventListener('change', applyAuth);
  applyAuth();
  sshBody.append(h('div', { class: 'form-grid' },
    field('SSH 主机', sshHost),
    field('SSH 端口', sshPort),
    field('SSH 用户', sshUser),
    field('认证方式', sshAuth),
    sshPassField,
    sshKeyField,
  ));
  sshBody.append(h('p', { class: 'hint', style: { marginTop: '0' } }, '通过本地端口转发连接 MySQL；SSH 主机密钥在首次连接时自动信任。'));
  const sshSection = h('div', { class: 'section' },
    h('div', { class: 'section-title' },
      h('span', {}, 'SSH 隧道'),
      h('span', { class: 'spacer' }),
      h('label', { class: 'toggle' }, h('span', { class: 'switch' }, sshEnabled, h('span')), '启用'),
    ),
    sshBody,
  );
  const syncSSH = () => sshBody.classList.toggle('hidden', !sshEnabled.checked);
  sshEnabled.addEventListener('change', syncSSH);
  syncSSH();
  form.append(sshSection);

  const testResult = h('div', { class: 'test-result' });
  form.append(testResult);

  const m = openModal({ title: editing ? '编辑连接' : '新建连接', subtitle: 'MySQL', body: form, wide: true });
  const collect = () => {
    const payload = {
      id: draft.id || '', name: name.value.trim(), group: group.value.trim(), color: chosenColor,
      host: host.value.trim(), port: Number(port.value) || 3306, user: user.value.trim(),
      database: database.value.trim(), charset: charset.value.trim() || 'utf8mb4',
      sslMode: sslMode.value, timeout: Number(timeout.value) || 10, remark: remark.value,
      ssh: {
        enabled: sshEnabled.checked, host: sshHost.value.trim(), port: Number(sshPort.value) || 22,
        user: sshUser.value.trim(), auth: sshAuth.value, keyPath: sshKey.value.trim(),
        hasPassword: !!draft.ssh.hasPassword,
      },
    };
    if (password.value) payload.password = password.value;
    if (sshPassword.value) payload.sshPassword = sshPassword.value;
    return payload;
  };

  const testBtn = h('button', { class: 'btn' }, '测试连接');
  const cancel = h('button', { class: 'btn' }, '取消');
  const save = h('button', { class: 'btn primary' }, editing ? '保存' : '创建');
  cancel.addEventListener('click', () => m.close());
  testBtn.addEventListener('click', async () => {
    const payload = collect();
    if (!payload.host || !payload.user) { toast('请填写主机与用户名', 'warn'); return; }
    testBtn.disabled = true;
    testResult.className = 'test-result show';
    testResult.textContent = '正在连接…';
    try {
      const res = await apiPost('/api/connections/test', payload);
      testResult.className = 'test-result show ok';
      testResult.innerHTML = `<b>连接成功</b> · MySQL ${escapeHtml(res.server?.version || '')} (${escapeHtml(res.server?.edition || '')}) · ${res.latency} ms · ${escapeHtml(res.server?.hostname || '')}`;
    } catch (err) {
      testResult.className = 'test-result show err';
      testResult.textContent = '连接失败：' + err.message;
    } finally {
      testBtn.disabled = false;
    }
  });
  save.addEventListener('click', async () => {
    const payload = collect();
    if (!payload.host) { toast('请填写主机', 'warn'); return; }
    if (!payload.user) { toast('请填写用户名', 'warn'); return; }
    try {
      save.disabled = true;
      const saved = await apiPost('/api/connections/save', payload);
      S.connections = S.connections.filter((c) => c.id !== saved.id);
      S.connections.push(saved);
      S.connections.sort((a, b) => (a.group || '').localeCompare(b.group || '') || a.name.localeCompare(b.name));
      renderTree();
      const welcome = S.tabs.find((t) => t.kind === 'welcome');
      if (welcome) renderWelcome(welcome);
      m.close();
      toast(editing ? '连接已保存' : '连接已创建', 'ok');
    } catch (err) {
      toast(err.message, 'err', 6000);
      save.disabled = false;
    }
  });
  if (editing) {
    const del = h('button', { class: 'btn danger' }, '删除');
    del.addEventListener('click', async () => {
      m.close();
      await deleteConnection(conn);
    });
    m.footer.append(del);
  }
  const ncxInput = h('input', { type: 'file', accept: '.ncx', class: 'bg-file-input' });
  const ncxBtn = h('label', { class: 'btn', title: '从 .ncx 导入连接' }, '导入 .ncx', ncxInput);
  const finishNCXImport = async (data) => {
    const state = await apiGet('/api/state');
    S.connections = state.connections || [];
    renderTree();
    const welcome = S.tabs.find((t) => t.kind === 'welcome');
    if (welcome) renderWelcome(welcome);
    m.close();
    toast(`已导入 ${data.count || 0} 个连接` + (data.warnings?.length ? `；${data.warnings[0]}` : ''), data.warnings?.length ? 'warn' : 'ok', 6000);
  };
  const uploadNCX = async (file, confirmed = false, overwrite = false) => {
    const fd = new FormData();
    fd.append('file', file);
    fd.append('confirmed', confirmed ? 'true' : 'false');
    fd.append('overwrite', overwrite ? 'true' : 'false');
    const res = await fetch('/api/connections/import-ncx', { method: 'POST', headers: { 'X-NavMini-Token': TOKEN }, body: fd });
    const data = await res.json().catch(() => ({}));
    if (!res.ok) throw new Error(data.error || `HTTP ${res.status}`);
    if (data.needsConfirm) {
      const overwriteBox = h('input', { type: 'checkbox' });
      const body = h('div', {},
        h('p', { class: 'modal-msg' }, '检测到名称和连接配置均相同的连接：'),
        h('ul', { class: 'conflict-list' }, ...(data.conflicts || []).map((n) => h('li', {}, n))),
        h('label', { class: 'check' }, overwriteBox, h('span', {}, '覆盖更新同名同配置连接')),
        h('p', { class: 'hint' }, '不勾选则新增一个名为 _copy 的连接。'));
      const dm = openModal({ title: '连接已存在', body, wide: true });
      const cancelImport = h('button', { class: 'btn', onClick: () => dm.close() }, '取消');
      const go = h('button', { class: 'btn primary' }, '继续导入');
      go.addEventListener('click', async () => {
        go.disabled = true;
        try {
          dm.close();
          await uploadNCX(file, true, overwriteBox.checked);
        } catch (err) { toast(err.message, 'err', 7000); }
        finally { go.disabled = false; }
      });
      dm.footer.append(h('span', { class: 'spacer' }), cancelImport, go);
      return;
    }
    await finishNCXImport(data);
  };
  ncxInput.addEventListener('change', async () => {
    const file = ncxInput.files && ncxInput.files[0];
    if (!file) return;
    try { await uploadNCX(file); }
    catch (err) { toast(err.message, 'err', 8000); }
    finally { ncxInput.value = ''; }
  });
  m.footer.append(h('span', { class: 'spacer' }), ncxBtn, testBtn, cancel, save);
  setTimeout(() => name.focus(), 60);
}

function field(label, input, hint) {
  return h('div', { class: 'field' }, h('label', {}, label), input, hint ? h('span', { class: 'hint' }, hint) : null);
}

/* ------------------------------------------------------------------ */
/* settings / about / history                                         */
/* ------------------------------------------------------------------ */

async function openSettings(section = 'general') {
  if (typeof section !== 'string') section = 'general';
  let ai = { enabled: false, baseURL: 'https://api.deepseek.com', model: 'deepseek-chat', permission: 'readonly', autoExplain: true, hasKey: false };
  try { ai = Object.assign(ai, await apiGet('/api/ai/state')); } catch { /* keep defaults */ }

  let lang = S.settings.language || 'en';
  let bgName = S.settings.background || '';
  let active = section === 'ai' ? 'ai' : 'general';

  const language = h('select', { class: 'input' },
    h('option', { value: 'en' }, 'English'), h('option', { value: 'zh' }, '中文'));
  language.value = lang;
  const theme = h('select', { class: 'input' },
    h('option', { value: 'dark' }, '深色'), h('option', { value: 'light' }, '浅色'));
  theme.value = S.settings.theme;
  const fontFamily = h('select', { class: 'input' },
    h('option', { value: 'system' }, '系统'), h('option', { value: 'mono' }, '等宽'), h('option', { value: 'serif' }, '衬线'));
  fontFamily.value = S.settings.fontFamily || 'system';
  const fontSize = h('input', { class: 'input', type: 'number', min: 11, max: 24, value: S.settings.fontSize || 13 });
  const pageSize = h('input', { class: 'input', type: 'number', value: S.settings.pageSize });
  const maxRows = h('input', { class: 'input', type: 'number', value: S.settings.maxRows });
  const defaultSort = h('input', { type: 'checkbox' });
  defaultSort.checked = S.settings.defaultSort !== false;

  const bgPreview = h('div', { class: 'bg-preview' });
  const bgFile = h('input', { type: 'file', accept: 'image/png,image/jpeg,image/webp,image/gif,.png,.jpg,.jpeg,.webp,.gif', class: 'bg-file-input' });
  const bgPick = h('label', { class: 'btn sm bg-pick' }, '上传图片', bgFile);
  const bgHint = h('div', { class: 'hint' }, getLang() === 'zh'
    ? 'PNG / JPG / WebP / GIF · 最大 8 MB · 也可以把图片拖到这里'
    : 'PNG / JPG / WebP / GIF · max 8 MB · or drag an image here');
  const bgClear = h('button', { class: 'btn sm', onClick: async () => {
    try {
      await apiPost('/api/settings/background/clear', {});
      bgName = '';
      S.settings.background = '';
      renderBg();
      applyBackground();
      toast('背景已移除', 'ok');
    } catch (err) { toast(err.message, 'err', 5000); }
  } }, '移除');
  const renderBg = () => {
    bgPreview.innerHTML = '';
    if (bgName) {
      bgPreview.append(h('img', { src: `/api/background?v=${encodeURIComponent(bgName)}`, alt: 'background' }));
    } else {
      bgPreview.append(h('span', { class: 'faint' }, 'No background image'));
    }
  };
  renderBg();
  const uploadBg = async (file) => {
    if (!file) return;
    const fd = new FormData();
    fd.append('file', file);
    try {
      const res = await fetch('/api/settings/background', {
        method: 'POST', headers: { 'X-NavMini-Token': TOKEN }, body: fd,
      });
      const data = await res.json().catch(() => ({}));
      if (!res.ok) throw new Error(data.error || `HTTP ${res.status}`);
      bgName = data.background;
      S.settings.background = bgName;
      renderBg();
      applyBackground();
      toast('背景已更新', 'ok');
    } catch (err) { toast(err.message, 'err', 5000); }
    finally { bgFile.value = ''; }
  };
  bgFile.addEventListener('change', () => uploadBg(bgFile.files && bgFile.files[0]));
  bgPreview.addEventListener('dragover', (e) => { e.preventDefault(); bgPreview.classList.add('drag'); });
  bgPreview.addEventListener('dragleave', () => bgPreview.classList.remove('drag'));
  bgPreview.addEventListener('drop', (e) => {
    e.preventDefault();
    bgPreview.classList.remove('drag');
    uploadBg(e.dataTransfer && e.dataTransfer.files && e.dataTransfer.files[0]);
  });

  const general = h('div', { class: 'form-grid' },
    field('语言 / Language', language),
    field('主题', theme),
    field('界面字体', fontFamily),
    field('编辑区字号', fontSize),
    field('每页行数', pageSize, '表数据浏览的默认分页大小'),
    field('查询最大行数', maxRows, '单次查询最多返回的行数'),
    h('div', { class: 'field wide' }, h('label', { class: 'check' }, defaultSort, h('span', {}, '默认按主键倒序（无主键不生效）'))),
    h('div', { class: 'field wide' }, h('label', {}, '背景图片'), bgPreview, h('div', { class: 'row gap-8' }, bgPick, bgClear), bgHint),
  );

  const aiEnabled = h('input', { type: 'checkbox' });
  aiEnabled.checked = !!ai.enabled;
  const aiBase = h('input', { class: 'input', value: ai.baseURL || 'https://api.deepseek.com' });
  const aiModel = h('input', { class: 'input', value: ai.model || 'deepseek-chat', placeholder: 'deepseek-chat' });
  const aiKey = h('input', { class: 'input', type: 'password', placeholder: ai.hasKey ? '已保存，留空保持不变' : 'sk-…' });
  const aiPermission = h('select', { class: 'input' },
    h('option', { value: 'readonly' }, '只读（只能 SELECT / SHOW / EXPLAIN）'),
    h('option', { value: 'modify' }, '可修改（允许 INSERT / UPDATE / REPLACE）'),
    h('option', { value: 'delete' }, '可删除（允许 DELETE / DROP / ALTER 等）'));
  aiPermission.value = ai.permission || 'readonly';
  const aiExplain = h('input', { type: 'checkbox' });
  aiExplain.checked = ai.autoExplain !== false;
  const aiTest = h('button', { class: 'btn sm' }, '测试连接');
  aiTest.addEventListener('click', async () => {
    aiTest.disabled = true;
    const old = aiTest.textContent;
    aiTest.textContent = '测试中…';
    try {
      await apiPost('/api/ai/test', { baseURL: aiBase.value.trim(), model: aiModel.value.trim(), apiKey: aiKey.value });
      toast('连接成功', 'ok');
    } catch (err) { toast(err.message, 'err', 7000); }
    finally { aiTest.disabled = false; aiTest.textContent = old; }
  });
  const aiForm = h('div', { class: 'form-grid' },
    h('div', { class: 'field wide' }, h('label', { class: 'check' }, aiEnabled, h('span', {}, '启用助手'))),
    field('Base URL', aiBase, 'OpenAI-compatible endpoint'),
    field('模型', aiModel),
    field('API Key', aiKey, '仅保存在服务端，使用 AES-256-GCM 加密；不会回传到浏览器。'),
    field('权限模式', aiPermission, '权限在服务端强制校验；写操作仍需在弹窗中确认。'),
    h('div', { class: 'field wide' }, h('label', { class: 'check' }, aiExplain, h('span', {}, '生成后自动 EXPLAIN，提示全表扫描'))),
    h('div', { class: 'field wide' }, h('div', { class: 'row gap-8' }, aiTest)),
  );

  const content = h('div');
  const tabBar = h('div', { class: 'settings-tabs' });
  const generalBtn = h('button', { class: 'btn sm' }, '通用');
  const aiBtn = h('button', { class: 'btn sm' }, 'DeepSeek');
  const render = () => {
    content.innerHTML = '';
    content.append(active === 'ai' ? aiForm : general);
    generalBtn.classList.toggle('primary', active === 'general');
    aiBtn.classList.toggle('primary', active === 'ai');
  };
  generalBtn.addEventListener('click', () => { active = 'general'; render(); });
  aiBtn.addEventListener('click', () => { active = 'ai'; render(); });
  tabBar.append(generalBtn, aiBtn);
  content.append(general);
  const body = h('div', {}, tabBar, content);
  const m = openModal({ title: '设置', subtitle: 'General · DeepSeek', body, wide: true });
  render();

  const save = h('button', { class: 'btn primary' }, '保存');
  const cancel = h('button', { class: 'btn' }, '取消');
  cancel.addEventListener('click', () => m.close());
  save.addEventListener('click', async () => {
    const nextLang = language.value === 'zh' ? 'zh' : 'en';
    const changedLang = nextLang !== (S.settings.language || 'en');
    S.settings = Object.assign({}, S.settings, {
      language: nextLang,
      theme: theme.value === 'light' ? 'light' : 'dark',
      fontFamily: ['mono', 'serif'].includes(fontFamily.value) ? fontFamily.value : 'system',
      fontSize: Math.max(11, Math.min(24, Number(fontSize.value) || 13)),
      pageSize: Math.max(10, Math.min(5000, Number(pageSize.value) || 200)),
      maxRows: Math.max(10, Math.min(100000, Number(maxRows.value) || 1000)),
      background: bgName,
      defaultSort: defaultSort.checked,
    });
    try {
      await apiPost('/api/settings', S.settings);
      await apiPost('/api/ai/config', {
        enabled: aiEnabled.checked,
        baseURL: aiBase.value.trim(),
        model: aiModel.value.trim(),
        permission: aiPermission.value,
        autoExplain: aiExplain.checked,
        apiKey: aiKey.value,
      });
      applyTheme(S.settings.theme);
      applyFontSettings();
      applyBackground();
      window.dispatchEvent(new CustomEvent('navmini:ai-config'));
      toast('设置已保存', 'ok');
      m.close();
      if (changedLang) { setLang(nextLang); setTimeout(() => location.reload(), 250); }
    } catch (err) { toast(err.message, 'err', 6000); }
  });
  m.footer.append(h('span', { class: 'spacer' }), cancel, save);
}

function openAbout() {
  const zh = getLang() === 'zh';
  const body = h('div');
  body.innerHTML = `
    <div style="display:flex;gap:14px;align-items:center;margin-bottom:14px">
      <div class="welcome-logo" style="width:46px;height:46px"></div>
      <div><div style="font-size:18px;font-weight:650">NavMini</div>
      <div class="faint" style="font-size:11.5px">v${escapeHtml(VERSION)} · ${zh ? '轻量 MySQL 工作台' : 'a lightweight MySQL workbench'}</div></div>
    </div>
    <p class="modal-msg">${zh
      ? '基于 REA 对某软件 17.3.10 的静态构造分析，把「原生外壳 + 核心服务 + 驱动层 + 模块化 UI」压缩为单进程 Go 服务 + WebView 界面。'
      : 'Inspired by a static-construction study of a certain database GUI 17.3.10, NavMini compresses the native shell + core service + driver layer + modular UI into a single Go process and a WebView front end.'}</p>
    <div class="kv-grid" style="margin-top:12px">
      <div class="kv-row"><span class="k">${zh ? '后端' : 'Backend'}</span><span class="v">Go · database/sql</span></div>
      <div class="kv-row"><span class="k">${zh ? '驱动' : 'Driver'}</span><span class="v">go-sql-driver/mysql</span></div>
      <div class="kv-row"><span class="k">${zh ? '隧道' : 'Tunnel'}</span><span class="v">golang.org/x/crypto/ssh</span></div>
      <div class="kv-row"><span class="k">${zh ? '界面' : 'UI'}</span><span class="v">WKWebView + SPA</span></div>
      <div class="kv-row"><span class="k">${zh ? '凭据' : 'Credentials'}</span><span class="v">AES-256-GCM</span></div>
      <div class="kv-row"><span class="k">${zh ? '分析报告' : 'Architecture'}</span><span class="v">docs/software-architecture.md</span></div>
    </div>`;
  const m = openModal({ title: '关于 NavMini', body });
  m.footer.append(h('button', { class: 'btn primary', onClick: () => m.close() }, '关闭'));
}

function connEndpoint(conn) {
  if (!conn) return '';
  return `${conn.host || '127.0.0.1'}:${conn.port || 3306}`;
}

function saveCurrentQuery(tab) {
  if (!tab || !tab.editor) return;
  const sql = tab.editor.value.trim();
  if (!sql) { toast('没有可执行的 SQL', 'warn'); return; }
  const conn = connById(tab.connId);
  if (!conn) { toast('连接不存在', 'err'); return; }
  const name = h('input', { class: 'input', value: sql.split('\n')[0].slice(0, 60) || 'Query' });
  const m = openModal({ title: '保存查询', body: h('div', { class: 'form-grid' }, field('名称', name)) });
  const save = h('button', { class: 'btn primary' }, '保存');
  save.addEventListener('click', async () => {
    try {
      await apiPost('/api/saved-queries/save', {
        name: name.value.trim() || 'Query', sql, host: connEndpoint(conn), database: tab.db || '',
      });
      toast('已保存', 'ok'); m.close();
    } catch (err) { toast(err.message, 'err', 5000); }
  });
  m.footer.append(h('span', { class: 'spacer' }), h('button', { class: 'btn', onClick: () => m.close() }, '取消'), save);
}

async function openSavedQueries(connId) {
  const active = connId ? connById(connId) : (activeTab()?.connId ? connById(activeTab().connId) : null);
  const host = active ? connEndpoint(active) : '';
  let items = [];
  try { items = await apiGet('/api/saved-queries' + (host ? `?host=${encodeURIComponent(host)}` : '')); }
  catch (err) { toast(err.message, 'err', 5000); return; }
  const body = h('div');
  body.append(h('div', { class: 'faint', style: { fontSize: '11px', padding: '2px 4px 8px' } },
    host ? `${host} · ${items.length}` : `${items.length}`));
  if (!items.length) {
    body.append(h('div', { class: 'empty-state compact' }, h('h3', {}, '还没有保存的查询')));
  }
  for (const item of items) {
    const node = h('div', { class: 'history-item' },
      h('div', { class: 'sql' }, item.sql),
      h('div', { class: 'meta' },
        h('span', {}, item.name),
        item.database ? h('span', {}, item.database) : null,
        h('span', { class: 'faint' }, item.host || ''),
        h('span', { style: { marginLeft: 'auto' } }, timeAgo(item.updatedAt)),
      ),
    );
    node.addEventListener('click', () => {
      closeDrawer();
      if (!active) { toast('没有可用连接', 'warn'); return; }
      openQuery(active.id, item.database || '', item.sql);
    });
    const del = h('button', { class: 'btn ghost sm', title: '删除' }, '×');
    del.addEventListener('click', async (e) => {
      e.stopPropagation();
      try { await apiPost('/api/saved-queries/delete', { id: item.id }); node.remove(); toast('已删除', 'ok'); }
      catch (err) { toast(err.message, 'err', 5000); }
    });
    node.append(del);
    body.append(node);
  }
  openDrawer({ title: '已保存查询', body });
}

async function openHistory() {
  let items = [];
  try { items = await apiGet('/api/history'); } catch (err) { toast(err.message, 'err', 5000); return; }
  const body = h('div');
  const bar = h('div', { style: { display: 'flex', gap: '8px', padding: '2px 4px 8px', alignItems: 'center' } },
    h('span', { class: 'faint', style: { fontSize: '11px' } }, `${items.length} 条记录`),
    h('span', { class: 'flex-1' }),
    h('button', {
      class: 'btn sm', onClick: async () => {
        try {
          await api('/api/history', { method: 'DELETE' });
          closeDrawer();
          toast('历史已清空', 'ok');
        } catch (err) { toast(err.message, 'err', 5000); }
      },
    }, '清空'),
  );
  body.append(bar);
  if (!items.length) {
    body.append(h('div', { class: 'empty-state compact' }, h('span', { html: icon('history') }), h('h3', {}, '还没有查询历史')));
  }
  for (const item of items) {
    const node = h('div', { class: 'history-item' },
      h('div', { class: 'sql' }, item.sql),
      h('div', { class: 'meta' },
        h('span', {}, item.connName || ''),
        item.database ? h('span', {}, item.database) : null,
        h('span', { class: item.success ? 'ok' : 'err' }, item.success ? `${item.rows} 行` : '失败'),
        h('span', {}, formatMs(item.millis)),
        h('span', { style: { marginLeft: 'auto' } }, timeAgo(item.at)),
      ),
    );
    node.addEventListener('click', () => {
      const connId = activeTab()?.connId || S.connections[0]?.id;
      if (!connId) { toast('没有可用连接', 'warn'); return; }
      closeDrawer();
      openQuery(connId, item.database || '', item.sql);
    });
    body.append(node);
  }
  openDrawer({ title: '查询历史', body });
}

function importAIAttachment(att, ctx) {
  if (!att || !att.uploadId) { toast('该附件不是表格，无法导入', 'warn'); return; }
  if (!ctx || !ctx.selectedTable) { toast('请先在表数据页打开目标表', 'warn'); return; }
  const tab = openTable(ctx.connId, ctx.db || '', ctx.selectedTable);
  openImportWizard(tab, att);
}

window.__navmini = {
  newQuery: () => newQueryFromContext(),
  newConnection: () => openConnectionDialog(),
  closeTab: () => { const t = activeTab(); if (t) closeTab(t.id); },
  run: () => { const t = activeTab(); if (t && t.kind === 'query') runQuery(t, false); },
  history: () => openHistory(),
  savedQueries: (connId) => openSavedQueries(connId),
  settings: () => openSettings(),
  about: () => openAbout(),
  openQuery: (connId, db, sql) => openQuery(connId, db, sql),
  askAI: (ctx) => openAssistant(ctx),
  importAttachment: (att, ctx) => importAIAttachment(att, ctx),
  exportConnections: () => openExportConnections(),
};

init();

import { apiGet, apiPost } from './api.js';
import { h, toast, openModal, copyText, escapeHtml, formatNumber } from './dom.js';
import { icon } from './icons.js';
import { Grid } from './grid.js';
import { tr } from './i18n.js';

let live = null;

const MODE_LABEL = { readonly: '只读', modify: '可修改', delete: '可删除' };
const KIND_LABEL = { read: '只读', modify: '修改', delete: '删除', blocked: '已阻止' };

export async function openAssistant(ctx = {}) {
  if (live) {
    if (ctx.connId && ctx.connId !== live.connId) { closeAssistant(); }
    else {
      if (ctx.db) live.ctx.db = ctx.db;
      if (ctx.selectedRow) { live.ctx.selectedRow = ctx.selectedRow; live.ctx.selectedTable = ctx.selectedTable; renderRowContext(); }
      live.panel.classList.add('visible');
      return;
    }
  }
  if (!ctx.connId) { toast('请先选择一个连接', 'warn'); return; }
  const panel = buildPanel(ctx);
  live = {
    panel, ctx, connId: ctx.connId, sessionId: null, state: null, databases: [],
    selectedDbs: new Set(), history: [], busy: false, last: null,
  };
  document.body.append(panel);
  renderRowContext();
  requestAnimationFrame(() => panel.classList.add('visible'));
  await bootstrap();
}

export function closeAssistant() {
  if (!live) return;
  if (live.sessionId) apiPost('/api/session/close', { session: live.sessionId }).catch(() => {});
  live.panel.remove();
  live = null;
}

function buildPanel(ctx) {
  const panel = h('div', { class: 'ai-panel' });
  const modeBadge = h('span', { class: 'ai-mode' }, '只读');
  const settingsBtn = h('button', { class: 'btn ghost sm icon', title: 'DeepSeek 设置', html: icon('settings') });
  const closeBtn = h('button', { class: 'btn ghost sm icon', title: '关闭', html: icon('x') });
  const head = h('div', { class: 'ai-head' },
    h('span', { class: 'ai-logo', html: icon('sparkles') }),
    h('div', { class: 'ai-title' }, 'DeepSeek 助手', h('span', { class: 'ai-sub' }, '默认只生成 SQL，执行前确认')),
    modeBadge,
    settingsBtn, closeBtn,
  );

  const scopeBox = h('div', { class: 'ai-scope' });
  const notice = h('div', { class: 'ai-notice hidden' });
  const messages = h('div', { class: 'ai-messages' });
  const rowCtx = h('div', { class: 'ai-row-ctx hidden' });
  const input = h('textarea', { class: 'ai-input', placeholder: '描述你想查或改什么，例如：按国家统计客户数与余额，按余额倒序', spellcheck: 'false' });
  const sendBtn = h('button', { class: 'btn primary sm', html: icon('play') + '<span>发送</span>' });
  const composer = h('div', { class: 'ai-compose' }, rowCtx, input,
    h('div', { class: 'ai-compose-bar' }, h('span', { class: 'faint' }, '⌘↩ 发送'), h('span', { class: 'flex-1' }), sendBtn));
  panel.append(head, notice, scopeBox, messages, composer);
  panel._ui = { modeBadge, scopeBox, notice, messages, input, sendBtn, rowCtx, settingsBtn, closeBtn };
  closeBtn.addEventListener('click', closeAssistant);
  settingsBtn.addEventListener('click', () => (window.__navmini?.openSettings?.('ai') || openAISettings()));
  sendBtn.addEventListener('click', () => sendMessage());
  input.addEventListener('keydown', (e) => {
    if ((e.metaKey || e.ctrlKey) && e.key === 'Enter') { e.preventDefault(); sendMessage(); }
  });
  return panel;
}

async function bootstrap() {
  const { panel } = live;
  await loadState();
  await loadDatabases();
  renderScope();
  renderMessages();
  await ensureSession();
  if (!live.state?.hasKey) showNotice('尚未配置 DeepSeek API Key。配置后即可生成 SQL。', true);
  else if (!live.state?.enabled) showNotice('DeepSeek 助手未启用，请到设置里开启。', true);
}

async function ensureSession() {
  if (live.sessionId) return live.sessionId;
  try {
    const v = await apiPost('/api/session/open', { connectionId: live.connId, database: '' });
    live.sessionId = v.id;
    return v.id;
  } catch (err) {
    toast(err.message, 'err', 6000);
    throw err;
  }
}

async function loadState() {
  try {
    const st = await apiGet('/api/ai/state');
    live.state = st;
    const { modeBadge } = live.panel._ui;
    modeBadge.textContent = tr(MODE_LABEL[st.permission] || st.permission);
    modeBadge.dataset.mode = st.permission;
    panelClass(st);
  } catch (err) {
    toast(err.message, 'err', 5000);
  }
}

function panelClass(st) {
  if (!live || !st) return;
  live.panel.classList.toggle('ai-disabled', !st.enabled || !st.hasKey);
  const { sendBtn } = live.panel._ui;
  sendBtn.disabled = !st.enabled || !st.hasKey;
}

function showNotice(text, configurable = false) {
  const box = live.panel._ui.notice;
  box.innerHTML = '';
  box.append(h('span', { class: 'ai-notice-icon', html: icon('info') }), h('span', {}, text));
  if (configurable) {
    const b = h('button', { class: 'btn sm' }, '配置');
    b.addEventListener('click', () => (window.__navmini?.openSettings?.('ai') || openAISettings()));
    box.append(h('span', { class: 'flex-1' }), b);
  }
  box.classList.remove('hidden');
}

async function loadDatabases() {
  try {
    const sid = await ensureSession();
    const dbs = await apiGet('/api/databases?session=' + encodeURIComponent(sid));
    live.databases = dbs;
    const preferred = live.ctx.db;
    for (const d of dbs) {
      if (preferred && d.name === preferred) live.selectedDbs.add(d.name);
    }
    if (!live.selectedDbs.size && dbs.length) {
      const first = dbs.find((d) => !d.system) || dbs[0];
      live.selectedDbs.add(first.name);
    }
  } catch { /* notice shown elsewhere */ }
}

function renderScope() {
  const box = live.panel._ui.scopeBox;
  box.innerHTML = '';
  box.append(h('span', { class: 'ai-scope-label' }, '数据库'));
  if (!live.databases.length) { box.append(h('span', { class: 'faint' }, '加载中…')); return; }
  for (const db of live.databases) {
    if (db.system) continue;
    const cb = h('input', { type: 'checkbox' });
    cb.checked = live.selectedDbs.has(db.name);
    const label = h('label', { class: 'ai-db-chip' + (cb.checked ? ' active' : '') }, cb, h('span', {}, db.name));
    cb.addEventListener('change', () => {
      if (cb.checked) live.selectedDbs.add(db.name);
      else live.selectedDbs.delete(db.name);
      label.classList.toggle('active', cb.checked);
    });
    box.append(label);
  }
  const all = h('button', { class: 'link-btn', style: { marginLeft: 'auto' } }, '仅当前库');
  all.addEventListener('click', () => {
    live.selectedDbs.clear();
    if (live.ctx.db) live.selectedDbs.add(live.ctx.db);
    else {
      const first = live.databases.find((d) => !d.system);
      if (first) live.selectedDbs.add(first.name);
    }
    renderScope();
  });
  box.append(all);
}

function renderRowContext() {
  const box = live.panel._ui.rowCtx;
  if (!live.ctx.selectedRow) { box.classList.add('hidden'); return; }
  box.innerHTML = '';
  box.append(h('span', { html: icon('rows'), style: { display: 'flex' } }),
    h('span', {}, `已附带 ${live.ctx.selectedTable || ''} 选中行`),
    h('span', { class: 'flex-1' }));
  const clear = h('button', { class: 'btn ghost sm icon', title: '移除上下文', html: icon('x') });
  clear.addEventListener('click', () => { live.ctx.selectedRow = null; live.ctx.selectedTable = ''; renderRowContext(); });
  box.append(clear);
  box.classList.remove('hidden');
}

function renderMessages() {
  const box = live.panel._ui.messages;
  box.innerHTML = '';
  if (!live.history.length) {
    box.append(h('div', { class: 'ai-empty' },
      h('div', { class: 'ai-empty-icon', html: icon('sparkles') }),
      h('h3', {}, '让 DeepSeek 帮你写 SQL'),
      h('p', {}, '描述需求，助手会结合表结构、主键和索引生成一条 SQL；默认只生成，不会自动执行。'),
      h('p', { class: 'faint' }, '例：找出过去 30 天订单金额前 10 的客户，按金额倒序')));
    return;
  }
  for (const msg of live.history) {
    const bubble = h('div', { class: 'ai-bubble ' + msg.role });
    bubble.append(h('div', { class: 'ai-bubble-text' }, msg.content));
    box.append(bubble);
    if (msg.response) box.append(renderSQLCard(msg.response));
  }
  box.scrollTop = box.scrollHeight;
}

function renderSQLCard(r) {
  const card = h('div', { class: 'ai-sql' });
  const head = h('div', { class: 'ai-sql-head' },
    h('span', { class: 'ai-kind kind-' + (r.kind || 'read') }, KIND_LABEL[r.kind] || (r.kind || 'SQL')),
    h('span', { class: 'ai-sql-title' }, '生成的 SQL'),
    h('span', { class: 'flex-1' }));
  const copy = h('button', { class: 'btn ghost sm icon', title: '复制 SQL', html: icon('copy') });
  copy.addEventListener('click', () => copyText(r.sql));
  const open = h('button', { class: 'btn ghost sm icon', title: '在查询中打开', html: icon('terminal') });
  open.addEventListener('click', () => {
    window.__navmini?.openQuery(live.connId, firstDb(), r.sql);
    closeAssistant();
  });
  head.append(copy, open);
  card.append(head);
  if (r.notes) card.append(h('div', { class: 'ai-sql-note' }, r.notes));
  if (r.blocked) {
    card.append(h('div', { class: 'ai-blocked', html: icon('shield') + ' ' + escapeHtml(r.blockReason || '当前权限不允许执行该语句') }));
  }
  card.append(h('pre', { class: 'ai-sql-code' }, r.sql || '（没有生成 SQL）'));
  if (r.warnings && r.warnings.length) {
    const warnings = h('div', { class: 'ai-warnings' });
    for (const w of r.warnings) warnings.append(h('div', { class: 'ai-warning', html: icon('alert') + ' ' + escapeHtml(w) }));
    card.append(warnings);
  }
  if (r.plan && r.plan.length) {
    const table = h('table', { class: 'ai-plan' });
    table.innerHTML = `<thead><tr><th>表</th><th>type</th><th>key</th><th>rows</th></tr></thead><tbody>${r.plan.map((p) => `<tr>
      <td>${escapeHtml(p.table || '')}</td><td>${escapeHtml(p.type || '')}</td>
      <td>${escapeHtml(p.key || '—')}</td><td>${escapeHtml(p.rows || '')}</td></tr>`).join('')}</tbody>`;
    const slow = (r.warnings || []).some((w) => String(w).includes('全表扫描'));
    card.append(h('div', { class: 'ai-plan-label' }, slow ? '执行计划（存在全表扫描）' : '执行计划'), table);
  }
  const actions = h('div', { class: 'ai-sql-actions' });
  const run = h('button', { class: 'btn sm ' + (r.kind === 'read' ? 'primary' : 'danger'), disabled: r.blocked || !r.sql },
    r.kind === 'read' ? '运行 SELECT' : '确认执行');
  run.addEventListener('click', () => executeAI(r, run, card._runResult));
  actions.append(run);
  card.append(actions);
  card._runResult = h('div', { class: 'ai-run-result hidden' });
  card.append(card._runResult);
  return card;
}

function firstDb() {
  return [...live.selectedDbs][0] || live.ctx.db || '';
}

async function sendMessage() {
  if (live.busy) return;
  const input = live.panel._ui.input;
  const text = input.value.trim();
  if (!text) return;
  if (!live.state?.enabled || !live.state?.hasKey) { toast('请先配置并启用 DeepSeek', 'warn', 3000); return; }
  if (!live.selectedDbs.size) { toast('请至少选择一个数据库', 'warn'); return; }
  await ensureSession();
  live.busy = true;
  live.panel._ui.sendBtn.disabled = true;
  live.history.push({ role: 'user', content: text });
  input.value = '';
  renderMessages();
  const loading = h('div', { class: 'ai-bubble assistant loading' }, h('span', { class: 'spinner' }), h('span', {}, 'DeepSeek 思考中…'));
  live.panel._ui.messages.append(loading);
  live.panel._ui.messages.scrollTop = 1e9;
  try {
    const payload = {
      session: live.sessionId,
      databases: [...live.selectedDbs],
      currentDatabase: firstDb(),
      messages: live.history.map((m) => ({ role: m.role, content: m.content })),
    };
    if (live.ctx.selectedRow) {
      payload.selectedRow = live.ctx.selectedRow;
      payload.selectedTable = live.ctx.selectedTable || '';
    }
    const res = await apiPost('/api/ai/chat', payload);
    if (!live) return;
    const msg = {
      role: 'assistant',
      content: res.reply || (res.sql ? '已生成 SQL，请核对后执行。' : '已完成。'),
      response: res,
    };
    live.history.push(msg);
    live.last = res;
    renderMessages();
    if (res.schemaTruncated) toast(`Schema 较大，已截断到 ${res.schemaTables} 张表`, 'warn', 4000);
  } catch (err) {
    toast(err.message, 'err', 7000);
    if (live) {
      live.history.push({ role: 'assistant', content: '请求失败：' + err.message });
      renderMessages();
    }
  } finally {
    if (live) {
      live.busy = false;
      live.panel._ui.sendBtn.disabled = !live.state?.enabled || !live.state?.hasKey;
    }
  }
}

async function executeAI(r, btn, resultEl) {
  if (r.blocked) return;
  if (r.kind !== 'read') {
    const ok = await confirmWrite(r);
    if (!ok) return;
  }
  btn.disabled = true;
  btn.textContent = '执行中…';
  try {
    const res = await apiPost('/api/ai/execute', {
      session: live.sessionId,
      database: firstDb(),
      sql: r.sql,
      confirmed: r.kind !== 'read',
    });
    if (!live) return;
    renderRunResult(resultEl, res);
  } catch (err) {
    toast(err.message, 'err', 7000);
  } finally {
    btn.disabled = false;
    btn.textContent = r.kind === 'read' ? '运行 SELECT' : '确认执行';
  }
}

function confirmWrite(r) {
  return new Promise((resolve) => {
    let settled = false;
    const finish = (v) => { if (!settled) { settled = true; resolve(v); } };
    const check = h('input', { type: 'checkbox' });
    const m = openModal({ title: '确认执行 AI 生成的 SQL', subtitle: MODE_LABEL[live.state?.permission] || '', wide: true, onClose: () => finish(false) });
    m.body.append(
      h('div', { class: 'ai-confirm-warn', html: icon('alert') + ' 该操作会修改数据库，请确认 SQL 与影响范围。' }),
      h('pre', { class: 'ai-sql-code' }, r.sql),
      h('label', { class: 'ai-confirm-check' }, check, ' 我已核对 SQL，确认执行'),
    );
    const cancel = h('button', { class: 'btn' }, '取消');
    const run = h('button', { class: 'btn danger', disabled: true }, '确认执行');
    check.addEventListener('change', () => { run.disabled = !check.checked; });
    cancel.addEventListener('click', () => { finish(false); m.close(); });
    run.addEventListener('click', () => { finish(true); m.close(); });
    m.footer.append(h('span', { class: 'spacer' }), cancel, run);
  });
}

function renderRunResult(el, res) {
  if (!el) return;
  el.classList.remove('hidden');
  el.innerHTML = '';
  if (res.error) el.append(h('div', { class: 'ai-run-error' }, res.error));
  const result = (res.results || [])[0];
  if (result && result.columns && result.columns.length) {
    const gridHost = h('div', { class: 'ai-result-grid' });
    el.append(gridHost);
    const grid = new Grid(gridHost, { sortable: false, emptyText: '没有返回行' });
    grid.setData({ columns: result.columns, rows: result.rows, pk: [] });
  } else {
    const affected = (res.results || []).reduce((a, x) => a + (x.affected || 0), 0);
    el.append(h('div', { class: 'ai-run-ok' }, `执行完成 · ${formatNumber(affected)} 行受影响`));
  }
}

function openAISettings() {
  const st = live?.state || {};
  const enabled = h('input', { type: 'checkbox' });
  enabled.checked = !!st.enabled;
  const baseURL = h('input', { class: 'input', value: st.baseURL || 'https://api.deepseek.com' });
  const model = h('select', { class: 'input' },
    h('option', { value: 'deepseek-chat' }, 'deepseek-chat'),
    h('option', { value: 'deepseek-reasoner' }, 'deepseek-reasoner'));
  model.value = st.model || 'deepseek-chat';
  const permission = h('select', { class: 'input' },
    h('option', { value: 'readonly' }, '只读（只能 SELECT / SHOW / EXPLAIN）'),
    h('option', { value: 'modify' }, '可修改（允许 INSERT / UPDATE / REPLACE）'),
    h('option', { value: 'delete' }, '可删除（允许 DELETE / DROP / ALTER 等）'));
  permission.value = st.permission || 'readonly';
  const autoExplain = h('input', { type: 'checkbox' });
  autoExplain.checked = st.autoExplain !== false;
  const key = h('input', { class: 'input', type: 'password', placeholder: st.hasKey ? '已保存，留空保持不变' : 'sk-...' });
  const result = h('div', { class: 'test-result' });
  const form = h('div', {},
    h('div', { class: 'form-grid' },
      h('div', { class: 'field span2' }, h('label', {}, 'DeepSeek API'), h('div', { class: 'ai-toggle-line' }, h('label', { class: 'switch' }, enabled, h('span')), h('span', { class: 'faint' }, '启用助手'))),
      h('div', { class: 'field span2' }, h('label', {}, 'Base URL'), baseURL),
      h('div', { class: 'field' }, h('label', {}, '模型'), model),
      h('div', { class: 'field' }, h('label', {}, '权限模式'), permission),
      h('div', { class: 'field span2' }, h('label', {}, 'API Key'), key, h('span', { class: 'hint' }, '仅保存在服务端，使用 AES-256-GCM 加密；不会回传到浏览器。')),
      h('div', { class: 'field span2' }, h('label', { class: 'ai-check-line' }, autoExplain, ' 生成后自动 EXPLAIN，提示全表扫描')),
    ),
    h('p', { class: 'faint', style: { fontSize: '11.5px', lineHeight: '1.7' } }, '权限在服务端强制校验；写操作仍需在弹窗中确认。'),
    result,
  );
  const m = openModal({ title: 'DeepSeek 设置', subtitle: 'API / 权限', body: form, wide: true });
  const test = h('button', { class: 'btn' }, '测试连接');
  test.addEventListener('click', async () => {
    test.disabled = true;
    result.className = 'test-result show';
    result.textContent = '测试中…';
    try {
      const res = await apiPost('/api/ai/test', { baseURL: baseURL.value.trim(), model: model.value, apiKey: key.value.trim() });
      result.className = 'test-result show ok';
      result.textContent = `连接成功 · ${res.model} · ${res.latency} ms`;
    } catch (err) {
      result.className = 'test-result show err';
      result.textContent = err.message;
    } finally {
      test.disabled = false;
    }
  });
  const cancel = h('button', { class: 'btn' }, '取消');
  cancel.addEventListener('click', () => m.close());
  const save = h('button', { class: 'btn primary' }, '保存');
  save.addEventListener('click', async () => {
    save.disabled = true;
    try {
      const out = await apiPost('/api/ai/config', {
        enabled: enabled.checked, baseURL: baseURL.value.trim(), model: model.value,
        permission: permission.value, autoExplain: autoExplain.checked, apiKey: key.value.trim(),
      });
      if (live) {
        live.state = out;
        live.panel._ui.modeBadge.textContent = tr(MODE_LABEL[out.permission] || out.permission);
        live.panel._ui.modeBadge.dataset.mode = out.permission;
        panelClass(out);
        if (out.hasKey && out.enabled) live.panel._ui.notice.classList.add('hidden');
        else showNotice(!out.hasKey ? '尚未配置 DeepSeek API Key。' : 'DeepSeek 助手未启用。', true);
      }
      toast('DeepSeek 设置已保存', 'ok');
      m.close();
    } catch (err) {
      toast(err.message, 'err', 6000);
      save.disabled = false;
    }
  });
  m.footer.append(h('span', { class: 'spacer' }), test, cancel, save);
}


document.addEventListener('navmini:ai-config', async () => {
  if (!live) return;
  await loadState();
  const notice = live.panel._ui.notice;
  if (live.state?.enabled && live.state?.hasKey) {
    notice.classList.add('hidden');
  } else if (!live.state?.hasKey) {
    showNotice('尚未配置 DeepSeek API Key。配置后即可生成 SQL。', true);
  } else {
    showNotice('DeepSeek 助手未启用，请到设置里开启。', true);
  }
});

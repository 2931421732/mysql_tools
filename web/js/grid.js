import { h, escapeHtml, contextMenu, copyText, toast } from './dom.js';

const ROW_H = 27;
const HEAD_H = 31;
const ROWNUM_W = 52;

export class Grid {
  constructor(host, opts = {}) {
    this.host = host;
    this.opts = opts;
    this.columns = [];
    this.rows = [];
    this.pk = new Set();
    this.widths = [];
    this.selected = { row: -1, col: -1 };
    this.sort = null;
    this.editable = false;
    this.emptyText = opts.emptyText || '没有数据';
    this._raf = 0;

    host.classList.add('grid');
    host.innerHTML = `
      <div class="grid-head"><div class="grid-head-inner"></div></div>
      <div class="grid-body"><div class="grid-canvas"></div></div>`;
    this.head = host.querySelector('.grid-head');
    this.headInner = host.querySelector('.grid-head-inner');
    this.body = host.querySelector('.grid-body');
    this.canvas = host.querySelector('.grid-canvas');

    this.body.addEventListener('scroll', () => this.onScroll(), { passive: true });
    this.body.addEventListener('click', (e) => this.onClick(e));
    this.body.addEventListener('dblclick', (e) => this.onDblClick(e));
    this.body.addEventListener('contextmenu', (e) => this.onContext(e));
    this.headInner.addEventListener('click', (e) => this.onHeadClick(e));
    this.headInner.addEventListener('mousedown', (e) => this.onHeadMouseDown(e));
    this.headInner.addEventListener('dragstart', (e) => {
      const th = e.target.closest('.gh-cell[data-col]');
      if (!th || !e.dataTransfer) { e.preventDefault(); return; }
      const col = this.columns[Number(th.dataset.col)];
      e.dataTransfer.setData('text/plain', col.name);
      e.dataTransfer.effectAllowed = 'copy';
    });
    window.addEventListener('resize', () => this.scheduleRender());
  }

  setData({ columns = [], rows = [], pk = [], sort = null, editable = false, emptyText, timestampDisplay = false } = {}) {
    const changed = this.signature(columns) !== this.signature(this.columns) || this.widths.length !== columns.length;
    this.columns = columns;
    this.rows = rows;
    this.pk = new Set(pk || []);
    this.sort = sort;
    this.editable = !!editable;
    this.timestampDisplay = !!timestampDisplay;
    if (emptyText) this.emptyText = emptyText;
    this.selected = { row: -1, col: -1 };
    if (changed || !this.widths.length) this.computeWidths();
    this.body.scrollTop = 0;
    this.body.scrollLeft = 0;
    this.renderHead();
    this.renderRows();
  }

  signature(columns) {
    return columns.map((c) => c.name + ':' + (c.dbType || '')).join('|');
  }

  computeWidths() {
    this.widths = this.columns.map((c) => {
      const name = (c.name || '').length;
      let w = Math.max(96, Math.min(230, name * 8 + 54));
      if (c.numeric) w = Math.max(96, Math.min(150, w));
      if (c.binary) w = Math.max(140, Math.min(240, w));
      if (name > 18) w = Math.min(260, w + 20);
      return w;
    });
  }

  totalWidth() {
    return ROWNUM_W + this.widths.reduce((a, b) => a + b, 0);
  }

  renderHead() {
    const sortInfo = this.sort ? this.sort : null;
    let html = `<div class="gh-cell rownum" style="width:${ROWNUM_W}px">#</div>`;
    this.columns.forEach((c, i) => {
      const w = this.widths[i];
      const isPk = this.pk.has(c.name);
      let ind = '';
      if (sortInfo && sortInfo.column === c.name) ind = sortInfo.desc ? ' ↓' : ' ↑';
      const colTitle = [c.name, c.dbType || '', c.comment || ''].filter(Boolean).join(' · ');
      html += `<div class="gh-cell ${this.opts.sortable ? 'sortable' : ''}" draggable="true" data-col="${i}" style="width:${w}px" title="${escapeHtml(colTitle)}">
        <span class="col-name">${escapeHtml(c.name)}${isPk ? ' <span class="pk">PK</span>' : ''}${ind ? `<span class="sort-ind">${ind}</span>` : ''}</span>
        <span class="type">${escapeHtml((c.dbType || '').toLowerCase())}</span>
        <div class="gh-resizer" data-col="${i}"></div>
      </div>`;
    });
    this.headInner.innerHTML = html;
    this.headInner.style.width = this.totalWidth() + 'px';
  }

  onScroll() {
    this.headInner.style.transform = `translateX(${-this.body.scrollLeft}px)`;
    this.scheduleRender();
  }

  scheduleRender() {
    if (this._raf) return;
    this._raf = requestAnimationFrame(() => {
      this._raf = 0;
      this.renderRows();
    });
  }

  cellText(value, col = null) {
    if (value === null || value === undefined) return 'NULL';
    const formatted = this.timestampText(value, col);
    if (formatted) return formatted;
    return String(value).replace(/\r?\n/g, ' ⏎ ');
  }

  timestampText(value, col) {
    if (!this.timestampDisplay || !col) return null;
    const type = String(col.dbType || '').toUpperCase();
    if (type.includes('DATE') || type.includes('TIME')) return null;
    const name = String(col.name || '').toLowerCase();
    if (!/(^|_)(time|timestamp|ts|at|created|updated|deleted|expire|mtime|ctime)($|_)/.test(name) &&
        !/(timestamp|_at|_time|^time$|created|updated)/.test(name)) return null;
    const raw = String(value);
    if (!/^\d{10,16}$/.test(raw)) return null;
    let ms;
    if (raw.length >= 16) ms = Number(raw.slice(0, 13));
    else if (raw.length >= 13) ms = Number(raw.slice(0, 13));
    else ms = Number(raw) * 1000;
    if (!Number.isFinite(ms)) return null;
    const d = new Date(ms);
    if (Number.isNaN(d.getTime()) || d.getFullYear() < 2000 || d.getFullYear() > 2100) return null;
    const p = (n) => String(n).padStart(2, '0');
    return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}:${p(d.getSeconds())}`;
  }

  cellClasses(value, col, row, ci) {
    let cls = 'gc';
    if (col.numeric) cls += ' num';
    if (value === null || value === undefined) cls += ' null';
    if (value !== null && typeof value === 'string' && value.startsWith('0x')) cls += ' bin';
    if (this.selected.row === row && this.selected.col === ci) cls += ' cell-selected';
    if (this.editable && this.opts.canEdit && this.opts.canEdit(row, ci)) cls += ' editable';
    return cls;
  }

  renderRows() {
    const total = this.rows.length;
    this.canvas.style.height = Math.max(total * ROW_H, 10) + 'px';
    this.canvas.style.minWidth = this.totalWidth() + 'px';
    if (!total) {
      this.canvas.innerHTML = `<div class="grid-empty">${escapeHtml(this.emptyText)}</div>`;
      return;
    }
    const scrollTop = this.body.scrollTop;
    const height = this.body.clientHeight || 400;
    const start = Math.max(0, Math.floor(scrollTop / ROW_H) - 4);
    const end = Math.min(total, Math.ceil((scrollTop + height) / ROW_H) + 4);
    let html = '';
    for (let r = start; r < end; r++) {
      const row = this.rows[r];
      const odd = r % 2 ? ' odd' : '';
      const sel = this.selected.row === r ? ' selected' : '';
      html += `<div class="grid-row${odd}${sel}" data-row="${r}" style="transform:translateY(${r * ROW_H}px);width:${this.totalWidth()}px">`;
      html += `<div class="rownum">${r + 1}</div>`;
      for (let c = 0; c < this.columns.length; c++) {
        const col = this.columns[c];
        const value = row[c];
        const text = this.cellText(value, col);
        const raw = value === null || value === undefined ? 'NULL' : String(value);
        const title = text !== this.cellText(value) && text.length > 0
          ? `${text} · raw: ${raw.slice(0, 300)}`
          : (text.length > 40 ? text.slice(0, 400) : '');
        html += `<div class="${this.cellClasses(value, col, r, c)}" data-row="${r}" data-col="${c}" style="width:${this.widths[c]}px"${title ? ` title="${escapeHtml(title)}"` : ''}>${escapeHtml(text)}</div>`;
      }
      html += `</div>`;
    }
    this.canvas.innerHTML = html;
  }

  cellFromEvent(e) {
    const cell = e.target.closest('.gc');
    if (!cell || !this.canvas.contains(cell)) return null;
    return { row: Number(cell.dataset.row), col: Number(cell.dataset.col), cell };
  }

  onClick(e) {
    const hit = this.cellFromEvent(e);
    if (!hit) return;
    this.select(hit.row, hit.col);
    if (this.opts.onSelect) this.opts.onSelect(hit.row, hit.col);
  }

  select(row, col) {
    this.canvas.querySelectorAll('.gc.cell-selected').forEach((el) => el.classList.remove('cell-selected'));
    this.canvas.querySelectorAll('.grid-row.selected').forEach((el) => el.classList.remove('selected'));
    this.selected = { row, col };
    const cell = this.canvas.querySelector(`.gc[data-row="${row}"][data-col="${col}"]`);
    if (cell) {
      cell.classList.add('cell-selected');
      const rowEl = cell.closest('.grid-row');
      if (rowEl) rowEl.classList.add('selected');
    }
  }

  onDblClick(e) {
    const hit = this.cellFromEvent(e);
    if (!hit) return;
    if (!this.editable) return;
    if (this.opts.canEdit && !this.opts.canEdit(hit.row, hit.col)) return;
    if (this.opts.onEdit) this.startEdit(hit.row, hit.col, hit.cell);
  }

  async startEdit(row, col, cell) {
    const colDef = this.columns[col];
    if (colDef.binary) {
      toast('二进制列不可直接编辑', 'warn', 2200);
      return;
    }
    const bodyRect = this.body.getBoundingClientRect();
    const cellRect = cell.getBoundingClientRect();
    const oldValue = this.rows[row][col];
    const input = document.createElement('input');
    input.className = 'cell-editor';
    input.value = oldValue === null || oldValue === undefined ? '' : String(oldValue);
    input.style.left = cellRect.left - bodyRect.left + this.body.scrollLeft + 'px';
    input.style.top = cellRect.top - bodyRect.top + this.body.scrollTop + 'px';
    input.style.width = Math.max(140, this.widths[col] - 2) + 'px';
    input.style.height = ROW_H - 2 + 'px';
    this.body.append(input);
    input.focus();
    input.select();

    let done = false;
    const finish = async (commit) => {
      if (done) return;
      done = true;
      const value = input.value;
      input.remove();
      if (!commit || value === (oldValue === null || oldValue === undefined ? '' : String(oldValue))) return;
      try {
        await this.opts.onEdit(row, col, value, oldValue);
        this.updateCell(row, col, value);
      } catch (err) {
        toast(err.message || String(err), 'err', 5000);
      }
    };
    input.addEventListener('keydown', (e) => {
      if (e.key === 'Enter') { e.preventDefault(); finish(true); }
      else if (e.key === 'Escape') { e.preventDefault(); finish(false); }
      e.stopPropagation();
    });
    input.addEventListener('blur', () => finish(true));
  }

  updateCell(row, col, value) {
    this.rows[row][col] = value;
    const cell = this.canvas.querySelector(`.gc[data-row="${row}"][data-col="${col}"]`);
    if (!cell) return;
    cell.textContent = this.cellText(value, this.columns[col]);
    cell.className = this.cellClasses(value, this.columns[col], row, col);
  }

  onContext(e) {
    const hit = this.cellFromEvent(e);
    if (!hit) return;
    e.preventDefault();
    this.select(hit.row, hit.col);
    const items = [];
    const value = this.rows[hit.row][hit.col];
    if (this.opts.onDetail) {
      items.push({ label: '查看详情', icon: 'file', action: () => this.opts.onDetail(hit.row, hit.col) });
      items.push({ sep: true });
    }
    items.push({ label: '复制单元格', icon: 'copy', shortcut: '⌘C', action: () => copyText(value === null ? '' : String(value)) });
    items.push({
      label: '复制整行 (TSV)', icon: 'rows', action: () => copyText(this.rows[hit.row].map((v) => (v === null ? '' : String(v))).join('\t')),
    });
    items.push({
      label: '复制列名', icon: 'columns', action: () => copyText(this.columns[hit.col].name),
    });
    if (this.editable) {
      items.push({ sep: true });
      items.push({ label: '编辑单元格', icon: 'pencil', action: () => this.startEdit(hit.row, hit.col, hit.cell) });
      items.push({ label: '设为 NULL', icon: 'minus', action: () => this.setNull(hit.row, hit.col) });
      if (this.opts.onDeleteRow) {
        items.push({ sep: true });
        items.push({ label: '删除该行', icon: 'trash', danger: true, action: () => this.opts.onDeleteRow(hit.row) });
      }
    }
    if (this.opts.onAskAI) {
      items.push({ sep: true });
      items.push({ label: '让 AI 修改此行', icon: 'sparkles', action: () => this.opts.onAskAI(hit.row) });
    }
    if (this.opts.onGenerateSQL && this.editable) {
      items.push({ sep: true });
      items.push({ label: '生成 INSERT', icon: 'plus', action: () => this.opts.onGenerateSQL(hit.row, 'insert') });
      items.push({ label: '生成 UPDATE', icon: 'pencil', action: () => this.opts.onGenerateSQL(hit.row, 'update') });
      items.push({ label: '生成 DELETE', icon: 'trash', danger: true, action: () => this.opts.onGenerateSQL(hit.row, 'delete') });
    }
    if (this.opts.extraMenu) this.opts.extraMenu(items, hit);
    contextMenu(e.clientX, e.clientY, items);
  }

  async setNull(row, col) {
    if (!this.opts.onEdit) return;
    try {
      await this.opts.onEdit(row, col, null, this.rows[row][col]);
      this.updateCell(row, col, null);
    } catch (err) {
      toast(err.message || String(err), 'err', 5000);
    }
  }

  onHeadClick(e) {
    if (e.target.closest('.gh-resizer')) return;
    const th = e.target.closest('.gh-cell[data-col]');
    if (!th || !this.opts.sortable || !this.opts.onSort) return;
    const col = this.columns[Number(th.dataset.col)];
    if (!col) return;
    let dir = 'asc';
    if (this.sort && this.sort.column === col.name) {
      dir = this.sort.desc ? null : 'desc';
    }
    this.sort = dir ? { column: col.name, desc: dir === 'desc' } : null;
    this.renderHead();
    this.opts.onSort(col.name, dir);
  }

  onHeadMouseDown(e) {
    const grip = e.target.closest('.gh-resizer');
    if (!grip) return;
    e.preventDefault();
    e.stopPropagation();
    const col = Number(grip.dataset.col);
    const startX = e.clientX;
    const startW = this.widths[col];
    const onMove = (ev) => {
      this.widths[col] = Math.max(60, Math.min(600, startW + ev.clientX - startX));
      const th = this.headInner.querySelector(`.gh-cell[data-col="${col}"]`);
      if (th) th.style.width = this.widths[col] + 'px';
      this.canvas.querySelectorAll(`.gc[data-col="${col}"]`).forEach((el) => { el.style.width = this.widths[col] + 'px'; });
      this.headInner.style.width = this.totalWidth() + 'px';
      this.canvas.style.minWidth = this.totalWidth() + 'px';
    };
    const onUp = () => {
      document.removeEventListener('mousemove', onMove);
      document.removeEventListener('mouseup', onUp);
      this.renderRows();
    };
    document.addEventListener('mousemove', onMove);
    document.addEventListener('mouseup', onUp);
  }

  getSelectedRow() {
    if (this.selected.row < 0) return -1;
    return this.selected.row;
  }

  destroy() {
    this.host.innerHTML = '';
    this.host.classList.remove('grid');
  }
}

export function gridToCSV(columns, rows) {
  const esc = (v) => {
    const s = v === null || v === undefined ? '' : String(v);
    return /[",\n\r]/.test(s) ? '"' + s.replaceAll('"', '""') + '"' : s;
  };
  return '\ufeff' + columns.map((c) => esc(c.name)).join(',') + '\n' +
    rows.map((r) => r.map(esc).join(',')).join('\n') + '\n';
}

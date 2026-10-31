import { h } from './dom.js';
import { icon } from './icons.js';

const KEYWORDS = new Set(`
SELECT FROM WHERE INSERT INTO VALUES UPDATE SET DELETE CREATE TABLE ALTER DROP TRUNCATE RENAME
JOIN INNER LEFT RIGHT FULL OUTER CROSS ON USING AS AND OR NOT NULL IS IN BETWEEN LIKE REGEXP
GROUP BY ORDER HAVING LIMIT OFFSET UNION ALL DISTINCT CASE WHEN THEN ELSE END EXISTS
PRIMARY KEY FOREIGN REFERENCES UNIQUE INDEX CONSTRAINT DEFAULT AUTO_INCREMENT COMMENT
ENGINE CHARSET COLLATE CHARACTER DATETIME TIMESTAMP VARCHAR INT BIGINT TINYINT SMALLINT
DECIMAL FLOAT DOUBLE TEXT BLOB JSON ENUM DATE TIME YEAR BOOLEAN UNSIGNED ZEROFILL
SHOW DATABASES TABLES COLUMNS INDEXES CREATE VIEW TRIGGER PROCEDURE FUNCTION EVENT
DESCRIBE EXPLAIN ANALYZE FORMAT PARTITION BEGIN START TRANSACTION COMMIT ROLLBACK SAVEPOINT
GRANT REVOKE USER IDENTIFIED BY PASSWORD PRIVILEGES WITH OPTION LOCK UNLOCK READ WRITE
IF ELSE WHILE DECLARE CURSOR RETURN CALL USE DATABASE SCHEMA TEMPORARY REPLACE IGNORE
ASC DESC OVER WINDOW RECURSIVE LATERAL STRAIGHT_JOIN NATURAL SQL_CALC_FOUND_ROWS
DUPLICATE KEY LAST_INSERT_ID CONNECTION_ID CURRENT_USER NOW CURDATE CURTIME
MATCH AGAINST BOOLEAN MODE SEPARATOR GROUP_CONCAT
`.trim().split(/\s+/));

const FUNCTIONS = new Set(`
COUNT SUM AVG MIN MAX ROUND ABS CEIL FLOOR MOD POW SQRT RAND
CONCAT CONCAT_WS LENGTH CHAR_LENGTH SUBSTRING SUBSTR LEFT RIGHT TRIM LTRIM RTRIM
UPPER LOWER REPLACE REVERSE LPAD RPAD INSTR LOCATE FORMAT
NOW CURDATE CURTIME UNIX_TIMESTAMP FROM_UNIXTIME DATE_FORMAT STR_TO_DATE DATEDIFF
DATE_ADD DATE_SUB TIMESTAMPDIFF EXTRACT YEAR MONTH DAY HOUR MINUTE SECOND
COALESCE IFNULL NULLIF IF GREATEST LEAST CAST CONVERT
JSON_EXTRACT JSON_UNQUOTE JSON_OBJECT JSON_ARRAY JSON_CONTAINS JSON_VALID
ROW_NUMBER RANK DENSE_RANK LAG LEAD FIRST_VALUE LAST_VALUE NTILE
UUID MD5 SHA2 LAST_INSERT_ID DATABASE USER VERSION
`.trim().split(/\s+/));

function highlightSQL(text) {
  const re = /(--[^\n]*|#[^\n]*|\/\*[\s\S]*?\*\/)|('(?:[^'\\]|\\.)*(?:'|$)|"(?:[^"\\]|\\.)*(?:"|$))|(`(?:[^`]|``)*(?:`|$))|(\b\d+(?:\.\d+)?\b)|([A-Za-z_][A-Za-z0-9_$]*)|([=<>!+\-*/%,;().|&^~]+)/g;
  let out = '';
  let last = 0;
  let m;
  while ((m = re.exec(text)) !== null) {
    if (m.index > last) out += escape(text.slice(last, m.index));
    const [full, comment, str, quotedIdent, num, word, op] = m;
    if (comment) out += `<span class="tok-com">${escape(full)}</span>`;
    else if (str) out += `<span class="tok-str">${escape(full)}</span>`;
    else if (quotedIdent) out += `<span class="tok-id">${escape(full)}</span>`;
    else if (num) out += `<span class="tok-num">${escape(full)}</span>`;
    else if (word) {
      const upper = word.toUpperCase();
      const after = text.slice(m.index + full.length).match(/^\s*\(/);
      if (KEYWORDS.has(upper)) out += `<span class="tok-key">${escape(full)}</span>`;
      else if (FUNCTIONS.has(upper) || after) out += `<span class="tok-fn">${escape(full)}</span>`;
      else out += escape(full);
    } else if (op) out += `<span class="tok-op">${escape(full)}</span>`;
    last = m.index + full.length;
  }
  if (last < text.length) out += escape(text.slice(last));
  return out;
}

function escape(s) {
  return s.replaceAll('&', '&amp;').replaceAll('<', '&lt;').replaceAll('>', '&gt;');
}

export class SQLEditor {
  constructor(host, opts = {}) {
    this.host = host;
    this.opts = opts;
    this.popup = null;
    this.popupItems = [];
    this.popupIndex = 0;
    this.completeSeq = 0;

    host.innerHTML = `
      <div class="editor">
        <div class="gutter"><div class="gutter-inner"></div></div>
        <div class="code">
          <pre class="highlight" aria-hidden="true"></pre>
          <textarea spellcheck="false" autocapitalize="off" autocomplete="off" autocorrect="off" wrap="off"></textarea>
        </div>
      </div>`;
    this.root = host.querySelector('.editor');
    this.gutter = host.querySelector('.gutter-inner');
    this.pre = host.querySelector('.highlight');
    this.ta = host.querySelector('textarea');
    this.ta.value = opts.value || '';
    this.bind();
    this.refresh();
  }

  get value() { return this.ta.value; }

  setValue(text, silent = false) {
    this.ta.value = text;
    this.refresh();
    if (!silent && this.opts.onChange) this.opts.onChange(text);
  }

  focus() { this.ta.focus(); }

  getSelection() {
    return { start: this.ta.selectionStart, end: this.ta.selectionEnd, text: this.ta.value.slice(this.ta.selectionStart, this.ta.selectionEnd) };
  }

  bind() {
    const ta = this.ta;
    ta.addEventListener('input', () => {
      this.refresh();
      if (this.opts.onChange) this.opts.onChange(ta.value);
      this.scheduleComplete(false);
    });
    ta.addEventListener('scroll', () => this.syncScroll());
    ta.addEventListener('keydown', (e) => this.onKeyDown(e));
    ta.addEventListener('blur', () => setTimeout(() => this.hidePopup(), 120));
    ta.addEventListener('click', () => this.hidePopup());
    window.addEventListener('resize', () => this.hidePopup());
  }

  onKeyDown(e) {
    const meta = e.metaKey || e.ctrlKey;
    if (meta && e.key === 'Enter') {
      e.preventDefault();
      this.hidePopup();
      if (this.opts.onRun) this.opts.onRun(e.shiftKey);
      return;
    }
    if (e.key === 'Tab' && !this.popup) {
      e.preventDefault();
      const { start, end } = this.getSelection();
      this.ta.setRangeText('  ', start, end, 'end');
      this.refresh();
      if (this.opts.onChange) this.opts.onChange(this.ta.value);
      return;
    }
    if (meta && e.key === ' ') {
      e.preventDefault();
      this.showCompletions(true);
      return;
    }
    if (this.popup) {
      if (e.key === 'ArrowDown') { e.preventDefault(); this.movePopup(1); return; }
      if (e.key === 'ArrowUp') { e.preventDefault(); this.movePopup(-1); return; }
      if (e.key === 'Enter' || e.key === 'Tab') { e.preventDefault(); this.acceptPopup(); return; }
      if (e.key === 'Escape') { e.preventDefault(); this.hidePopup(); return; }
    }
    if (e.key === 'Escape') this.hidePopup();
  }

  refresh() {
    const text = this.ta.value;
    this.pre.innerHTML = highlightSQL(text) + '\n';
    const lines = text.split('\n').length;
    let nums = '';
    for (let i = 1; i <= lines; i++) nums += i + '\n';
    this.gutter.textContent = nums;
    this.syncScroll();
  }

  syncScroll() {
    this.pre.scrollTop = this.ta.scrollTop;
    this.pre.scrollLeft = this.ta.scrollLeft;
    this.gutter.style.transform = `translateY(${-this.ta.scrollTop}px)`;
  }

  scheduleComplete(force) {
    clearTimeout(this._completeTimer);
    this._completeTimer = setTimeout(() => this.showCompletions(force), 90);
  }

  async showCompletions(force) {
    if (!this.opts.complete) return;
    const pos = this.ta.selectionStart;
    if (pos !== this.ta.selectionEnd) return this.hidePopup();
    const before = this.ta.value.slice(0, pos);
    const m = before.match(/[A-Za-z0-9_$]*$/);
    const word = m ? m[0] : '';
    if (!force && word.length < 2) return this.hidePopup();
    const seq = ++this.completeSeq;
    let items = [];
    try {
      items = await this.opts.complete(word, this.ta.value, pos);
    } catch {
      items = [];
    }
    if (seq !== this.completeSeq || !items || !items.length) {
      if (seq === this.completeSeq) this.hidePopup();
      return;
    }
    const lw = word.toLowerCase();
    items = items.filter((it) => it.label.toLowerCase().startsWith(lw)).slice(0, 40);
    if (!items.length) return this.hidePopup();
    this.popupItems = items;
    this.popupIndex = 0;
    this.wordStart = pos - word.length;
    this.renderPopup();
  }

  renderPopup() {
    this.hidePopup(false);
    const el = h('div', { class: 'ac-popup' });
    this.popupItems.forEach((it, i) => {
      const row = h('div', { class: 'ac-item' + (i === this.popupIndex ? ' active' : '') },
        h('span', { class: 'ac-kind kind-' + (it.kind || 'kw'), html: kindLabel(it.kind) }),
        h('span', { class: 'ac-label' }, it.label),
        it.detail ? h('span', { class: 'ac-detail' }, it.detail) : null,
      );
      row.addEventListener('mousedown', (e) => {
        e.preventDefault();
        this.popupIndex = i;
        this.acceptPopup();
      });
      el.append(row);
    });
    document.body.append(el);
    const pos = this.caretPosition();
    const rect = el.getBoundingClientRect();
    let left = pos.x;
    let top = pos.y + pos.lineHeight + 3;
    if (top + rect.height > window.innerHeight - 8) top = pos.y - rect.height - 3;
    el.style.left = Math.max(6, Math.min(left, window.innerWidth - rect.width - 8)) + 'px';
    el.style.top = Math.max(6, top) + 'px';
    this.popup = el;
    const active = el.querySelector('.ac-item.active');
    if (active) active.scrollIntoView({ block: 'nearest' });
  }

  movePopup(delta) {
    if (!this.popup) return;
    this.popupIndex = (this.popupIndex + delta + this.popupItems.length) % this.popupItems.length;
    this.popup.querySelectorAll('.ac-item').forEach((el, i) => el.classList.toggle('active', i === this.popupIndex));
    const active = this.popup.querySelector('.ac-item.active');
    if (active) active.scrollIntoView({ block: 'nearest' });
  }

  acceptPopup() {
    if (!this.popup || !this.popupItems.length) return;
    const item = this.popupItems[this.popupIndex];
    const caret = this.ta.selectionStart;
    this.ta.setRangeText(item.label, this.wordStart, caret, 'end');
    this.hidePopup();
    this.refresh();
    if (this.opts.onChange) this.opts.onChange(this.ta.value);
    this.ta.focus();
  }

  hidePopup(reset = true) {
    if (reset) this.completeSeq++;
    if (this.popup) {
      this.popup.remove();
      this.popup = null;
    }
  }

  caretPosition() {
    const ta = this.ta;
    const cs = getComputedStyle(ta);
    const div = document.createElement('div');
    for (const prop of ['fontFamily', 'fontSize', 'fontWeight', 'lineHeight', 'letterSpacing', 'paddingTop', 'paddingLeft', 'paddingRight', 'borderTopWidth', 'borderLeftWidth', 'whiteSpace', 'tabSize']) {
      div.style[prop] = cs[prop];
    }
    div.style.position = 'fixed';
    div.style.left = '-9999px';
    div.style.top = '0';
    div.style.visibility = 'hidden';
    div.style.whiteSpace = 'pre';
    div.style.width = ta.clientWidth + 'px';
    div.textContent = ta.value.slice(0, ta.selectionStart);
    const marker = document.createElement('span');
    marker.textContent = ta.value.slice(ta.selectionStart, ta.selectionStart + 1) || '.';
    div.append(marker);
    document.body.append(div);
    const relTop = marker.offsetTop - ta.scrollTop;
    const relLeft = marker.offsetLeft - ta.scrollLeft;
    const rect = ta.getBoundingClientRect();
    div.remove();
    return {
      x: rect.left + relLeft + parseFloat(cs.borderLeftWidth),
      y: rect.top + relTop + parseFloat(cs.borderTopWidth),
      lineHeight: parseFloat(cs.lineHeight) || 20,
    };
  }
}

function kindLabel(kind) {
  switch (kind) {
    case 'table': return 'T';
    case 'column': return 'C';
    case 'database': return 'D';
    case 'keyword': return 'K';
    case 'function': return 'F';
    default: return '·';
  }
}

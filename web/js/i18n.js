// Lightweight UI locale layer. Chinese source strings are the keys; English is
// the default. Unknown strings fall back to Chinese so new UI never renders blank.
const EN = {
  '连接': 'Connections', '新建连接': 'New Connection', '新建连接 (⌘N)': 'New Connection (⌘N)',
  '搜索连接、库、表': 'Search connections, databases, tables', '刷新连接树': 'Refresh tree',
  '查询历史': 'Query History', '设置': 'Settings', '切换主题': 'Toggle theme',
  '新建查询': 'New Query', '新建查询 (⌘T)': 'New Query (⌘T)', 'DeepSeek 助手': 'DeepSeek Assistant',
  '服务器监控': 'Server Monitor', '关于 NavMini': 'About NavMini', '未连接': 'Not connected',
  '欢迎': 'Welcome', '快速连接': 'Quick connect', '快捷键': 'Shortcuts',
  '运行全部 / 运行选中': 'Run all / Run selection', '新建连接': 'New connection',
  '自动补全': 'Auto complete', '关闭标签页': 'Close tab', '关闭其他标签': 'Close other tabs',
  '关闭': 'Close', '关闭 (⌘W)': 'Close (⌘W)', '取消': 'Cancel', '保存': 'Save', '应用': 'Apply',
  '删除': 'Delete', '编辑': 'Edit', '复制': 'Copy', '复制 SQL': 'Copy SQL', '复制名称': 'Copy name',
  '复制库名': 'Copy database name', '刷新': 'Refresh', '刷新对象': 'Refresh objects',
  '加载中…': 'Loading…', '加载失败': 'Load failed', '加载数据…': 'Loading data…',
  '加载对象…': 'Loading objects…', '加载表结构…': 'Loading schema…', '加载 DDL…': 'Loading DDL…',
  '表': 'Table', '视图': 'View', '数据库': 'Database', '数据': 'Data', '结构': 'Structure',
  '表信息': 'Table Info', '导出': 'Export', '导出 CSV': 'Export CSV', '导出 Excel': 'Export Excel',
  '插入': 'Insert', '插入行': 'Insert row', '删除行': 'Delete row', '删除选中行': 'Delete selected rows',
  '筛选 / 排序': 'Filter / Sort', '筛选与排序': 'Filter & Sort', '添加条件': 'Add condition',
  '添加排序': 'Add sort', '删除条件': 'Remove condition', '删除排序': 'Remove sort',
  '清空': 'Clear', '清除条件': 'Clear filters', '重置': 'Reset', '上移': 'Move up',
  '升序': 'Ascending', '降序': 'Descending', '升序 ASC': 'Ascending', '降序 DESC': 'Descending',
  '运算符': 'Operator', '值': 'Value', '第二个值': 'Second value', '列': 'Column',
  '与上一条件的连接': 'Join with previous', '为空': 'is null', '不为空': 'is not null',
  '包含': 'contains', '开头是': 'starts with', '结尾是': 'ends with', '默认': 'Default',
  '最多 3 个排序字段': 'Up to 3 sort keys', '未设置排序；默认按主键升序分页。':
    'No sort set; paging uses the primary key ascending.',
  '还没有筛选条件，点击「添加条件」。': 'No conditions yet. Click “Add condition”.',
  '快速备份到当前库': 'Backup to current database', '快速备份到当前库…': 'Backup to current database…',
  '目标表名': 'Target table', '备份内容': 'Contents', '仅结构': 'Structure only',
  '仅数据（目标表需已存在）': 'Data only (target must exist)',
  '结构 + 数据（默认）': 'Structure + data (default)', '开始备份': 'Start backup', '备份中…': 'Backing up…',
  '目标表不能与源表同名': 'Target must differ from source',
  '导入 Excel': 'Import Excel', '选择 Excel 文件': 'Choose Excel file', '工作表': 'Worksheet',
  '表头行': 'Header row', '下一步': 'Next', '上一步': 'Back', '开始导入': 'Start import',
  '导入中…': 'Importing…', '源表头': 'Source header', '目标列': 'Target column',
  '自动匹配': 'Auto match', '手动匹配': 'Manual match', '跳过': 'Skip', '已导入': 'Imported',
  '设置': 'Settings', '设置 (⌘,)': 'Settings (⌘,)', '通用': 'General', '语言': 'Language', '主题': 'Theme', '深色': 'Dark',
  '浅色': 'Light', '编辑区字号': 'Editor font size', '界面字体': 'UI font',
  '系统': 'System', '等宽': 'Monospace', '衬线': 'Serif', '背景图片': 'Background image',
  '上传图片': 'Upload image', '移除': 'Remove', '每页行数': 'Page size',
  '查询最大行数': 'Max query rows', '启用助手': 'Enable assistant', '模型': 'Model',
  '权限模式': 'Permission mode', '测试连接': 'Test connection', '已保存': 'Saved',
  '仅保存在服务端，使用 AES-256-GCM 加密；不会回传到浏览器。':
    'Stored server-side with AES-256-GCM; never sent back to the browser.',
  '已保存，留空保持不变': 'Saved; leave blank to keep', 'API Key': 'API Key',
  '只读': 'Read only', '可修改': 'Modify', '可删除': 'Delete',
  '只读（只能 SELECT / SHOW / EXPLAIN）': 'Read only (SELECT / SHOW / EXPLAIN)',
  '可修改（允许 INSERT / UPDATE / REPLACE）': 'Modify (INSERT / UPDATE / REPLACE)',
  '可删除（允许 DELETE / DROP / ALTER 等）': 'Delete (DELETE / DROP / ALTER etc.)',
  '权限在服务端强制校验；写操作仍需在弹窗中确认。':
    'Permissions are enforced server-side; writes still require confirmation.',
  '测试中…': 'Testing…', '连接成功': 'Connected', '连接失败：': 'Connection failed: ',
  '保存成功': 'Saved', '设置已保存': 'Settings saved', '历史已清空': 'History cleared',
  '没有可执行的 SQL': 'No SQL to run', '执行中…': 'Running…', '执行失败': 'Execution failed',
  '运行': 'Run', '选中': 'Selection', '停止': 'Stop', '解释': 'Explain', '运行 SELECT': 'Run SELECT', '解释': 'Explain', '停止': 'Stop',
  '确认执行': 'Confirm and run', '确认执行 AI 生成的 SQL': 'Confirm AI-generated SQL',
  '生成的 SQL': 'Generated SQL', '已生成 SQL，请核对后执行。': 'SQL generated. Review it before running.',
  '执行计划': 'Execution plan', '执行计划（存在全表扫描）': 'Execution plan (full scan)',
  '全表扫描': 'Full table scan', '没有返回行': 'No rows returned', '没有数据': 'No data',
  '查询没有返回数据': 'The query returned no rows', '还没有执行结果': 'No results yet',
  '没有可用连接': 'No connection available', '请先选择一个连接': 'Select a connection first',
  '请先新建一个连接': 'Create a connection first', '请先选择一行': 'Select a row first',
  '请先配置并启用 DeepSeek': 'Configure and enable DeepSeek first',
  '请至少选择一个数据库': 'Select at least one database', '仅当前库': 'Current database only',
  '已发送 KILL': 'KILL sent', '终止连接': 'Kill connection', '进程列表': 'Process list',
  '服务器变量': 'Server variables', '状态变量': 'Status variables', '查询/秒': 'Queries/sec',
  '连接数': 'Connections', '运行时间': 'Uptime', '流量': 'Traffic', '慢查询': 'Slow queries',
  '缓冲池命中': 'Buffer pool hit', '每 5 秒自动刷新': 'Auto refresh every 5s',
  '关于 NavMini': 'About NavMini', '后端': 'Backend', '驱动': 'Driver', '隧道': 'Tunnel',
  '界面': 'UI', '凭据': 'Credentials', '分析报告': 'Architecture notes',
  '尚未配置 DeepSeek API Key。': 'DeepSeek API key is not configured.',
  '尚未配置 DeepSeek API Key。配置后即可生成 SQL。':
    'DeepSeek API key is not configured. Add one to generate SQL.',
  'DeepSeek 设置': 'DeepSeek settings', 'DeepSeek 设置已保存': 'DeepSeek settings saved',
  'DeepSeek 助手未启用。': 'DeepSeek assistant is disabled.',
  'DeepSeek 助手未启用，请到设置里开启。': 'DeepSeek is disabled. Enable it in Settings.',
  '未设置': 'Not set', '只读': 'Read only', '关闭': 'Close', '打开': 'Open',
  '取消': 'Cancel', '确定': 'OK', '确认': 'Confirm', '是': 'Yes', '否': 'No',
  '时间': 'Time', '命令': 'Command', '状态': 'Status', '描述': 'Description',
  '类型': 'Type', '主键': 'Primary key', '唯一': 'Unique', '索引': 'Index',
  '列名': 'Column', '注释': 'Comment', '默认值': 'Default', '额外': 'Extra',
  '字符集': 'Charset', '外键': 'Foreign keys', '引用': 'References', '引擎': 'Engine',
  '表名': 'Table name', '备注': 'Comment', '分组': 'Group', '标识颜色': 'Color',
  '主机': 'Host', '端口': 'Port', '用户名': 'Username', '密码': 'Password',
  '默认数据库': 'Default database', '连接名称': 'Connection name',
  '连接超时（秒）': 'Connect timeout (s)', 'SSL 模式': 'SSL mode',
  'SSH 隧道': 'SSH tunnel', 'SSH 主机': 'SSH host', 'SSH 端口': 'SSH port',
  'SSH 用户': 'SSH user', 'SSH 密码': 'SSH password', '私钥路径': 'Private key path',
  'SSH 密码 / 私钥口令': 'SSH password / key passphrase', '认证方式': 'Auth method',
  '创建': 'Create', '编辑连接': 'Edit connection', '删除连接': 'Delete connection',
  '连接已创建': 'Connection created', '连接已保存': 'Connection saved',
  '连接已删除': 'Connection deleted', '连接不存在': 'Connection not found',
  '加载中…': 'Loading…', '没有对象': 'No objects', '没有数据库': 'No databases',
  '还没有连接，点击 + 新建': 'No connections yet. Click + to create one.',
  '提示：单击左侧表名打开数据浏览器；在结果网格中右键可复制单元格/整行。SQL 编辑器支持 ⌘↩ 执行与表/列名补全。':
    'Tip: click a table on the left to browse it; right-click a result cell/row to copy. The SQL editor supports ⌘↩ and table/column completion.',
  '按 ⌘↩ 运行 SQL，结果将显示在这里。': 'Press ⌘↩ to run SQL; results appear here.',
  '拖动调整宽度': 'Drag to resize', '拖动调整编辑区高度': 'Drag to resize editor',
  '未连接': 'Not connected', '已连接': 'Connected', '正在连接…': 'Connecting…',
  '默认只生成 SQL，执行前确认': 'Generates SQL first; review before running',
  '描述需求，助手会结合表结构、主键和索引生成一条 SQL；默认只生成，不会自动执行。':
    'Describe the task; the assistant uses schema, keys and indexes to generate one SQL. It does not execute automatically.',
  '描述你想查或改什么，例如：按国家统计客户数与余额，按余额倒序':
    'Describe what to query or change, e.g. customers by country with balance, descending',
  '发送': 'Send', '配置': 'Configure', '执行计划': 'Plan', '生成的 SQL': 'Generated SQL',
  '全表扫描': 'Full scan', '运行 SELECT': 'Run SELECT', '确认执行': 'Confirm and run',
  '在查询中打开': 'Open in editor', '复制': 'Copy', '已附带': 'Attached', '移除上下文': 'Remove context',
  '数据库': 'Databases', '仅当前库': 'Current database only',
  '一个受某软件启发的轻量 MySQL 工作台 · Go + 原生 WebView': 'A lightweight MySQL workbench · Go + native WebView',
  '已保存': 'Saved', '已保存查询': 'Saved queries', '保存查询': 'Save query', '名称': 'Name',
  '还没有保存的查询': 'No saved queries yet', '已删除': 'Deleted', '已导入': 'Imported',
  '选择 Excel 文件': 'Choose Excel file', '加载表头 / 预览': 'Load headers / preview',
  '读取中…': 'Reading…', '表头行': 'Header row', '源表头': 'Source header',
  '目标列': 'Target column', '开始导入': 'Start import', '导入中…': 'Importing…',
  '未读取到表头': 'No headers found', '跳过': 'Skip', '选择文件': 'Choose file',
  '导入 Excel': 'Import Excel', '导出 Excel': 'Export Excel', 'Excel': 'Excel',
  '保存查询成功': 'Query saved', '没有可导出的数据': 'No data to export',
  '通用': 'General',
  '导入': 'Import', '导出': 'Export', '插入': 'Insert', '删除': 'Delete', '保存': 'Save', '已保存': 'Saved',
  'CSV': 'CSV', 'Excel': 'Excel', '名称': 'Name', '保存查询': 'Save query',
  '已保存查询': 'Saved queries', '还没有保存的查询': 'No saved queries yet', '语言 / Language': 'Language', '背景图片': 'Background image',
  '上传图片': 'Upload image', '移除': 'Remove', '启用助手': 'Enable assistant',
  '测试连接': 'Test connection', '保存成功': 'Saved', '连接成功': 'Connected',
  '表数据浏览的默认分页大小': 'Default page size for table browsing',
  '单次查询最多返回的行数': 'Maximum rows returned per query',
  '生成后自动 EXPLAIN，提示全表扫描': 'Run EXPLAIN automatically and warn about full scans',
  '背景已更新': 'Background updated', '背景已移除': 'Background removed',
  '已导出 Excel': 'Excel exported', '最多附加 3 个附件': 'Up to 3 attachments', '该附件不是表格，无法导入': 'This attachment is not tabular', '请先在表数据页打开目标表': 'Open a table first', '附件已添加': 'Attachment added', 'Attach file': 'Attach file', 'Import': 'Import', 'xlsx / csv / sql / text': 'xlsx / csv / sql / text', '已导出 CSV': 'CSV exported', '已保存': 'Saved',
  '预览': 'Preview', '— 跳过 —': '— Skip —',
  'DeepSeek 助手': 'DeepSeek Assistant', '默认只生成 SQL，执行前确认': 'Generates SQL only; review before running',
  '尚未配置 DeepSeek API Key。配置后即可生成 SQL。': 'DeepSeek API key is not configured. Add one to generate SQL.',
  '移除上下文': 'Remove context', '在查询中打开': 'Open in editor',
  '运行 SELECT': 'Run SELECT', '确认执行': 'Confirm and run',
  '执行计划': 'Execution plan', '生成的 SQL': 'Generated SQL',
  '全表扫描': 'Full scan', '只读': 'Read only', '可修改': 'Modify', '可删除': 'Delete',
  '让 DeepSeek 帮你写 SQL': 'Ask DeepSeek to write SQL',
  '例：找出过去 30 天订单金额前 10 的客户，按金额倒序':
    'e.g. top 10 customers by order amount in the last 30 days, descending',
};

let lang = 'en';
const PATTERNS = [
  [/^(\d+) 行$/, '$1 rows'],
  [/^(\d+) 个库$/, '$1 databases'],
  [/^(\d+) 条记录$/, '$1 records'],
  [/^(\d+) 行受影响$/, '$1 rows affected'],
  [/^(\d+) 个筛选$/, '$1 filters'],
  [/^(\d+) 行 · (.*)$/, '$1 rows · $2'],
  [/^进程列表 · (\d+)$/, 'Process list · $1'],
  [/^结果 (\d+)$/, 'Result $1'],
  [/^接收 (.*)$/, 'Received $1'],
  [/^运行中 (.*)$/, 'Running $1'],
  [/^已附带 (.*) 选中行$/, 'Attached selected row from $1'],
  [/^已导入 (\d+) 行（扫描 (\d+)，(.*)）$/, 'Imported $1 rows (scanned $2, $3)'],
];

export function getLang() { return lang; }
export function setLang(next) {
  lang = next === 'zh' ? 'zh' : 'en';
  document.documentElement.lang = lang === 'zh' ? 'zh-CN' : 'en';
  document.documentElement.dataset.lang = lang;
}
export function tr(value) {
  if (lang !== 'en' || typeof value !== 'string') return value;
  if (Object.prototype.hasOwnProperty.call(EN, value)) return EN[value];
  for (const [re, rep] of PATTERNS) {
    if (re.test(value)) return value.replace(re, rep);
  }
  return value;
}
export function isChinese(value) {
  return typeof value === 'string' && /[\u4e00-\u9fff]/.test(value);
}

const SKIP_TAGS = new Set(['SCRIPT', 'STYLE', 'PRE', 'CODE', 'TEXTAREA', 'INPUT', 'SELECT', 'OPTION']);
const SKIP_CLASS = /(^|\s)(gc|grid|sql|mono|editor|code)(\s|$)/;

export function translateTree(root = document.body) {
  if (lang !== 'en' || !root) return;
  const walker = document.createTreeWalker(root, NodeFilter.SHOW_TEXT, {
    acceptNode(node) {
      const p = node.parentElement;
      if (!p || SKIP_TAGS.has(p.tagName) || SKIP_CLASS.test(p.className || '')) return NodeFilter.FILTER_REJECT;
      if (!isChinese(node.nodeValue)) return NodeFilter.FILTER_REJECT;
      return NodeFilter.FILTER_ACCEPT;
    },
  });
  const nodes = [];
  while (walker.nextNode()) nodes.push(walker.currentNode);
  for (const n of nodes) {
    const t = n.nodeValue;
    const trimmed = t.trim();
    const replaced = tr(trimmed);
    if (replaced !== trimmed) n.nodeValue = t.replace(trimmed, replaced);
  }
  for (const el of root.querySelectorAll('[title],[placeholder],[aria-label]')) {
    for (const attr of ['title', 'placeholder', 'aria-label']) {
      const v = el.getAttribute(attr);
      if (v && isChinese(v)) el.setAttribute(attr, tr(v));
    }
  }
}

# NavMini v0.3.0

发布日期：2026-11-01

## 亮点

- English-first UI + 中文切换（默认 English），Settings 统一语言/主题/字体/背景/DeepSeek
- 设置入口：顶部工具栏、侧栏、macOS 菜单 `NavMini → 设置…`（⌘,）
- 常见 SQL 按连接 `host:port` 保存与显示
- 未连接为灰色，连接后变绿色
- Excel 导入向导：sheet、表头行、自动/手动列映射、批量事务插入、时间格式归一化
- XLSX 导出：表数据/查询结果、表头、可选 SQL sheet；时间类型按文本写入
- 修复 WKWebView 文件选择器、背景图上传与背景图被不透明层遮挡的问题

## 下载

| 文件 | 大小 | SHA-256 |
|---|---|---|
| `NavMini-0.3.0-macos-arm64.dmg` | 5.4 MB | `58e95cb7b17b09dd8ca0607887114788cc3ca41d1c89bf9dad06732ed191f999` |
| `NavMini-0.3.0-windows-amd64.exe` | 13.0 MB | `4422abaf9dac91569279b1a255fcf367ec7593311b64a908fef834aa17c5ebe5` |
| `NavMini-0.3.0-windows-amd64.zip` | 5.1 MB | `6abb0d896197434446767579d5c9306457468ec86082d1a99f57932f09d328e4` |

## 安装

### macOS（Apple Silicon，macOS 13+）

1. 打开 DMG，将 `NavMini.app` 拖入 Applications
2. ad-hoc 签名、未公证；若被拦截，右键 App → 打开，或：
   `xattr -dr com.apple.quarantine /Applications/NavMini.app`

### Windows（x64）

1. 解压 zip，或直接下载 exe
2. 双击 `NavMini.exe`：控制台启动本地服务并打开默认浏览器
3. 保持控制台运行；关闭即停止服务
4. 便携版：Go 后端 + 浏览器 UI，不是原生 WebView2 窗口

## 数据与隐私

- 背景图片、连接配置、AI Key 只保存在本机应用配置目录，**不会提交到 Git 或 Release**
- 连接密码与 DeepSeek API Key 使用 AES-256-GCM 加密
- 服务只监听 `127.0.0.1` 随机端口

## 已知限制

- 仅 MySQL；无 PostgreSQL / MongoDB / Redis / SQLite
- Windows 包为浏览器 UI 便携版
- Excel 导入支持 `.xlsx/.xlsm`，上限 20 MB / 5 万行
- DeepSeek 需自备 API Key，默认只读、不自动执行

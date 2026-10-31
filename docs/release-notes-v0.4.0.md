# NavMini v0.4.0

发布日期：2026-11-01

## 亮点

- **DeepSeek 附件**：回形针附加 xlsx/xlsm/csv/tsv/txt/sql/md/json（最多 3 个、20 MB）
  - 表格附件解析 sheet/表头/预览，可直接交给当前表的导入向导
  - 文本/SQL 作为有长度限制的不可信上下文
  - 提示词明确禁止执行附件内指令；AI 仍只生成 SQL、不自动执行
- **原生 UI 沉降**：窗口顶部预留 30px，避开 macOS 关闭/最小化/全屏按钮
- 修复背景图片文件选择器、上传和背景被不透明层遮挡的问题
- English-first UI + 中文切换；Settings 统一语言/主题/字体/背景/DeepSeek
- 常见 SQL 按连接 `host:port` 保存与显示
- Excel 导入向导与 XLSX 导出（表头、可选 SQL sheet）

## 下载

| 文件 | 大小 | SHA-256 |
|---|---|---|
| `NavMini-0.4.0-macos-arm64.dmg` | 5.5 MB | `013ada072f7407c3d3f654af08c9b8f56a26de875e2d6e38f1d3fb1c13b5272d` |
| `NavMini-0.4.0-windows-amd64.exe` | 13.0 MB | `a300faec0dee26b4c45f03c370ed6b15ba594dae58dfff4ca96a8dff032ddb37` |
| `NavMini-0.4.0-windows-amd64.zip` | 5.2 MB | `9b8149aef2c2094383b16e6b081d77ee73ffb48fc40ae47f3b64bb857899990c` |

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

- 背景图片、连接配置、DeepSeek API Key 只保存在本机应用配置目录，
  **不会进入 Git 或 Release**
- 连接密码与 API Key 使用 AES-256-GCM 加密
- 附件会上传到配置的 DeepSeek 端点；敏感数据请谨慎附加
- 服务只监听 `127.0.0.1` 随机端口

## 已知限制

- 仅 MySQL；无 PostgreSQL / MongoDB / Redis / SQLite
- Windows 包为浏览器 UI 便携版
- 附件不支持图片/PDF/旧版 .xls；无 OCR
- AI 每次只允许一条语句、不自动执行

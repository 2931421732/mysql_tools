# NavMini v0.5.0

发布日期：2026-11-04

## 亮点

- **Windows 原生应用**：单个 `NavMini.exe`，内嵌 Go 服务 + WebView2 原生窗口，不再打开浏览器
- **`.ncx` 连接导入**：连接对话框 → 导入 .ncx，解析明文 XML/JSON 连接导出
- **详情面板**：表格/查询结果选中单元格 → 详情，底部完整展示大文本/JSON，不修改原数据
- **内联筛选/排序**：不再弹窗，直接在表头上方展开；覆盖层不改变结果区高度
- **字段拖拽筛选**：把表头字段拖到筛选栏即可添加条件
- **字段 COMMENT**：鼠标悬浮表头显示注释
- **连接状态**：未连接白色，连接成功绿色
- **数据库快速导出**：仅结构 / 结构+全部数据 / 自定义（全部结构 + 指定表数据）
- **时间戳显示**：数字时间戳可切换为可读时间，仅影响显示，编辑提交原始值
- 继续包含 Excel 导入向导、XLSX 导出、DeepSeek 附件、背景图、Settings 等 0.4.0 功能

## 下载

| 文件 | 大小 | SHA-256 |
|---|---|---|
| `NavMini-0.5.0-macos-arm64.dmg` | 5.5 MB | `898b80d1b39c2360590143156497f950aff3f17684c16ead2d597263e3168824` |
| `NavMini-0.5.0-windows-amd64.exe` | 15.9 MB | `c441a3d420233c879d005c86cc5bf90f4e347c6e24ec92ea7521a24b4e51510e` |
| `NavMini-0.5.0-windows-amd64.zip` | 6.4 MB | `d2031ede0564512f6c7fe1d59c46663c554d16c909872c151358cc7d1c86f3ee` |

## 安装

### macOS（Apple Silicon，macOS 13+）

1. 打开 DMG，把 `NavMini.app` 拖入 Applications
2. ad-hoc 签名、未公证；若被拦截，右键 App → 打开，或：
   `xattr -dr com.apple.quarantine /Applications/NavMini.app`

### Windows（x64）

1. 下载 exe 或 zip；双击 `NavMini.exe`
2. 需要 Microsoft Edge WebView2 Runtime（Win10/11 通常已预装）
3. 单 exe 内嵌服务，不需要浏览器或额外服务端
4. 未代码签名，SmartScreen 可能提示，选择“仍要运行”
5. 程序日志：`%LOCALAPPDATA%\NavMini\shell.log`

## 数据与隐私

- 背景图片、连接配置、DeepSeek API Key 只保存在本机应用配置目录，
  **不会进入 Git 或 Release**
- 连接密码与 API Key 使用 AES-256-GCM 加密
- 附件会上传到配置的 DeepSeek 端点；敏感数据请谨慎附加
- 服务只监听 `127.0.0.1` 随机端口

## 已知限制

- 仅 MySQL；无 PostgreSQL / MongoDB / Redis / SQLite
- NCX 仅支持明文 XML/JSON 连接导出；加密/二进制 NCX 不解密
- Windows 构建需要 WebView2 Runtime；本版在 macOS 上交叉编译，未在 Windows 本机运行验证
- 时间戳显示为只读展示；原始值仍是更新依据

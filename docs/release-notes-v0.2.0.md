# NavMini v0.2.0

发布日期：2026-11-01

## 亮点

- 多条件筛选/排序面板：AND/OR、IN/BETWEEN、多字段排序
- 选中行一键生成 INSERT / UPDATE / DELETE SQL
- 表树右键导出 CSV、当前库快速备份（结构+数据）
- DeepSeek 助手：加密 API Key、三档权限、Schema/索引上下文、EXPLAIN 提示、
  多库范围、只读探测、写操作确认

## 下载

| 文件 | 大小 | SHA-256 |
|---|---|---|
| `NavMini-0.2.0-macos-arm64.dmg` | 3.8 MB | `d28dc55255a19fd79b1e998ce4631953ce12838db4c7dfaef9590be6e117dad6` |
| `NavMini-0.2.0-windows-amd64.exe` | 8.7 MB | `95c608df0b6b0a91cac957e99e644b1588c32d40f201dcc986802276cf157a91` |
| `NavMini-0.2.0-windows-amd64.zip` | 3.5 MB | `cc628b4072e64a6faf14ae7d4ba046968afc8dc326c612ae0adc79c0a3950016` |

## 安装

### macOS（Apple Silicon，macOS 13+）

1. 打开 `NavMini-0.2.0-macos-arm64.dmg`
2. 将 `NavMini.app` 拖到 Applications
3. 本构建为 ad-hoc 签名，未做 Apple 公证；若被拦截，右键 App → 打开，
   或执行 `xattr -dr com.apple.quarantine /Applications/NavMini.app`

### Windows（x64）

1. 解压 `NavMini-0.2.0-windows-amd64.zip`，或直接下载 exe
2. 双击 `NavMini.exe`；控制台窗口启动本地服务并自动打开默认浏览器
3. 保持控制台运行（关闭即停止服务）
4. 这是便携版：Go 后端 + 浏览器 UI；本版本没有 Windows 原生 WebView2 外壳
5. 未代码签名，SmartScreen 可能提示，选择“仍要运行”

## 数据与安全

- macOS：`~/Library/Application Support/NavMini`
- Windows：`%USERPROFILE%\.navmini`
- 连接密码与 DeepSeek API Key 使用 AES-256-GCM 本地加密
- 服务只监听 `127.0.0.1` 随机端口，启动生成随机 token

## DeepSeek

默认关闭。打开右上角 AI 助手 → 齿轮 → 填 Base URL、模型、API Key → 测试连接 → 保存。
默认权限为只读；写/删操作需切换权限并在弹窗中确认。

## 已知限制

- 仅 MySQL 协议；没有 PostgreSQL / MongoDB / Redis / SQLite
- macOS 包仅 arm64；未公证
- Windows 包是浏览器 UI 便携版，不是原生窗口
- AI 每次只允许一条语句、不自动执行、不支持流式

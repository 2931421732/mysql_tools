# NavMini v0.2.1

发布日期：2026-11-01

## 本次变更

- Branding cleanup：README 中的原软件名称统一改为「某软件」，架构文档同步改名
- 同步 Gitee / GitHub 两个仓库，并增加 GitHub Actions 自动发布
- 功能与 v0.2.0 一致：多条件筛选、行 SQL、快速备份、DeepSeek 助手

## 下载

| 文件 | 大小 | SHA-256 |
|---|---|---|
| `NavMini-0.2.1-macos-arm64.dmg` | 3.8 MB | `a31dc2517947317513e64bd13490a39f1c5e90af6a5bc86deaf62b54c3b0d9fc` |
| `NavMini-0.2.1-windows-amd64.exe` | 8.7 MB | `09ff2c78694dd707b435279a6bb510bde32a17e325359007e272724e6e69276b` |
| `NavMini-0.2.1-windows-amd64.zip` | 3.5 MB | `96ee59cb4f2ab0d2791d1e936b0db8d8badd625ed077fc4a5b4f668af022d12c` |

## 安装

### macOS（Apple Silicon，macOS 13+）

1. 打开 `NavMini-0.2.1-macos-arm64.dmg`
2. 将 `NavMini.app` 拖到 Applications
3. ad-hoc 签名、未公证；若被拦截，右键 App → 打开，或：
   `xattr -dr com.apple.quarantine /Applications/NavMini.app`

### Windows（x64）

1. 解压 zip，或直接下载 exe
2. 双击 `NavMini.exe`：控制台启动本地服务，并自动打开默认浏览器
3. 保持控制台运行；关闭即停止服务
4. 这是便携版：Go 后端 + 浏览器 UI，不是原生 WebView2 窗口

## 数据与安全

- macOS：`~/Library/Application Support/NavMini`
- Windows：`%USERPROFILE%\.navmini`
- 连接密码与 DeepSeek API Key 使用 AES-256-GCM 本地加密
- 只监听 `127.0.0.1` 随机端口，启动生成随机 token

## 已知限制

- 仅 MySQL；无 PostgreSQL / MongoDB / Redis / SQLite
- Windows 包为浏览器 UI 便携版
- DeepSeek 需自备 API Key，默认只读、不自动执行

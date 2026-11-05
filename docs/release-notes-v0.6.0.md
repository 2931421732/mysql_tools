# NavMini v0.6.0

发布日期：2026-11-05

## AI Provider

设置中新增三种 Provider：

- **DeepSeek**：OpenAI Chat Completions
- **Claude Code**：Anthropic Messages
- **Codex**：OpenAI Responses

支持两种配置来源：

- 手动配置：Base URL / API Key / 模型下拉框（从 `/models` 获取）
- **引用本地配置**：只读读取，不修改原文件
  - Claude Code：`~/.claude/settings.json`（或项目 `.claude/settings*.json`）
  - Codex：`~/.codex/config.toml` + `auth.json`（或 `CODEX_HOME` / 项目 `.codex`）
  - DeepSeek：环境变量或项目 `.env`

新增 Anthropic Messages 与 OpenAI Responses 适配器；模型/协议/密钥均只保留在服务端。

## 附件

- 支持 `.zip`、`.tar`、`.tar.gz`、`.tgz` 内存解压
- 自动列出文件清单并提取日志/源码/文本内容，不落盘
- 支持无数据库会话直接分析日志、源代码、压缩包

## 验证

- 本机 Claude Code 配置检测成功，并通过真实 Anthropic 兼容端点测试
- Codex 配置检测、模型回退、Responses 适配器单测通过
- 本机 `beecode.cc` 在当前网络下 DNS 无法解析，Codex 在线请求未能验证
- 压缩包上传/解压/内容提取测试通过

## 下载

| 文件 | 大小 | SHA-256 |
|---|---|---|
| `NavMini-0.6.0-macos-arm64.dmg` | 5.6 MB | `126875c093f84e19be36ae12296df2400e8ff81a5d4dd9d3fb75993dd57613fa` |
| `NavMini-0.6.0-windows-amd64.exe` | 16.1 MB | `a9e3aad4adb2d1509f0972038f0b5c977691592c141fdeca6bed6963ef559faa` |
| `NavMini-0.6.0-windows-amd64.zip` | 6.4 MB | `d05895fec980a8e3d1ae975a9a18906b6609ea32cf15e8d47c8338851266d72e` |

## 安装

### macOS（Apple Silicon，macOS 13+）

打开 DMG → 拖入 Applications。被 Gatekeeper 拦截时右键打开，或：
`xattr -dr com.apple.quarantine /Applications/NavMini.app`

### Windows（x64）

双击 `NavMini.exe`，需要 WebView2 Runtime；单 exe 原生窗口，不使用浏览器。

## 安全

本地配置只读引用，不会改写 Claude Code / Codex 的配置文件；API Key 不回传浏览器。

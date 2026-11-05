# NavMini v0.7.0

发布日期：2026-11-06

## AI 助手

- 新增 **查数据 / 排障** 双模式，各自使用独立 System Prompt
- 排障模式不选数据库也能分析错误、日志、源码
- 自动带入当前 SQL 编辑器内容、选中 SQL、最近一次执行错误
- 新增 **上下文预算与预览**：
  - 预计 tokens
  - Schema / SQL / 错误 / 附件 / 历史分段字符数
  - 发送前的完整系统上下文预览
- 继续支持 DeepSeek / Claude Code / Codex Provider 和本地配置引用

## 性能与稳定性

- CSV 导出增加 64KB 缓冲
- 查询结果按最大行数预分配
- 等待执行锁后重新检查 Session 是否已关闭
- Grid 每帧只计算一次列宽
- 表浏览器搜索防抖
- API 日志增加耗时（ms）
- 打开库继续只加载轻量结构元数据，不展开左侧表树

## 下载

| 文件 | 大小 | SHA-256 |
|---|---|---|
| `NavMini-0.7.0-macos-arm64.dmg` | 5.6 MB | `49b2805ed50c6676f825fb8567096a9ab2448e4c8d50f0cb5a134a6cb5f60597` |
| `NavMini-0.7.0-windows-amd64.exe` | 16.1 MB | `5a2a9b8724e8867bc16c0e47a731e4bd19f7cc67688e4e354841934aee6ffa68` |
| `NavMini-0.7.0-windows-amd64.zip` | 6.4 MB | `22b45236aba115830951420a7b8207bee327f560a598ad9c96782d1e97b55af6` |

## 安装

### macOS（Apple Silicon，macOS 13+）

打开 DMG → 拖入 Applications。被 Gatekeeper 拦截时右键打开，或：
`xattr -dr com.apple.quarantine /Applications/NavMini.app`

### Windows（x64）

双击 `NavMini.exe`，需要 WebView2 Runtime；单 exe 原生窗口，不使用浏览器。

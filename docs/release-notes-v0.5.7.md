# NavMini v0.5.7

发布日期：2026-11-04

## 本次改动

- DeepSeek 助手支持上传**服务日志**和**源代码**
  - 日志：`.log/.out/.err/.trace`，保留头尾，适合排查超时/异常堆栈
  - 源代码：`.go/.py/.js/.ts/.java/.c/.cpp/.cs/.php/.rb/.rs/.swift/.sql` 等
  - 不选数据库时也可以直接分析附件
  - 附件上下文上限提升，日志保留开头 + 结尾
- DeepSeek 模型改为下拉选择
  - 设置中从服务端 `GET /models` 动态获取当前可用模型
  - 支持「刷新模型」；获取失败时回退到 `deepseek-chat` / `deepseek-reasoner`
  - 模型列表不再写死

## 下载

| 文件 | 大小 | SHA-256 |
|---|---|---|
| `NavMini-0.5.7-macos-arm64.dmg` | 5.5 MB | `09843529d726f37a20c4727acc7dd22d38a99061b3b61b961a5dfe69e8dfcded` |
| `NavMini-0.5.7-windows-amd64.exe` | 16.0 MB | `c6c7d9aee79add649afcaf23786ba9aaa25836e1d828428bb1885a303b62b8fa` |
| `NavMini-0.5.7-windows-amd64.zip` | 6.4 MB | `34f599005706b86175b6fdd3273b8bf1ecd8b2015843b28c6fd874f42f1c80c9` |

## 安装

### macOS（Apple Silicon，macOS 13+）

打开 DMG → 拖入 Applications。被 Gatekeeper 拦截时右键打开，或：
`xattr -dr com.apple.quarantine /Applications/NavMini.app`

### Windows（x64）

双击 `NavMini.exe`，需要 WebView2 Runtime；单 exe 原生窗口，不使用浏览器。

## 隐私

日志/源代码附件会发送到配置的 DeepSeek 端点；背景图片、连接配置、AI Key
仅保存在本机配置目录，不会进入 Git 或 Release。

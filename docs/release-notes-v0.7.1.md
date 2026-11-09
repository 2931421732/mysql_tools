# NavMini v0.7.1

发布日期：2026-11-09

## 修复 Claude Code 聊天 400

原因：Anthropic Messages 请求固定发送了 `temperature=0.2`。
部分 Claude 模型（如 LiteLLM 后面的 `claude-sonnet-5-5`）只接受
`temperature=1`，因此返回：

```text
UnsupportedParamsError: does not support temperature=0.2.
Only temperature=1 is supported.
```

修复：

- Anthropic Messages 适配器不再发送 `temperature`；
- 使用服务端默认温度；
- 增加回归测试，确认 Anthropic 请求体不包含 `temperature`；
- OpenAI Chat / Responses / DeepSeek 行为不变。

## 下载

| 文件 | 大小 | SHA-256 |
|---|---|---|
| `NavMini-0.7.1-macos-arm64.dmg` | 5.6 MB | `51dd33034db89efd296512aa91211fe92d1b42fb011d7642ce6361794075335a` |
| `NavMini-0.7.1-windows-amd64.exe` | 16.1 MB | `6f4e98cd469495f7b9563cdce7f6cb2ba0ca5300b4ce640b8849f4942b303556` |
| `NavMini-0.7.1-windows-amd64.zip` | 6.4 MB | `886786e52a0df75de6652499f64fef38ee9e86a6b693b9b84d8fdf8136f4081a` |

## 安装

### macOS（Apple Silicon，macOS 13+）

打开 DMG → 拖入 Applications。被 Gatekeeper 拦截时右键打开，或：
`xattr -dr com.apple.quarantine /Applications/NavMini.app`

### Windows（x64）

双击 `NavMini.exe`，需要 WebView2 Runtime；单 exe 原生窗口，不使用浏览器。

# NavMini v0.6.1

发布日期：2026-11-05

## 本次改动

- 左上搜索只过滤**连接源**：连接名、host、user、group；不再匹配库/表
- 点击库节点时在右侧打开表浏览器：
  - 加载该库全部表/视图
  - 顶部搜索框 + 搜索按钮，按表名过滤
  - 显示行数，点击直接打开表
  - 支持刷新表列表
  - 抽屉可关闭，不改变底部状态栏

## 下载

| 文件 | 大小 | SHA-256 |
|---|---|---|
| `NavMini-0.6.1-macos-arm64.dmg` | 5.6 MB | `a7d055f41d718063e21386f63abcaa027b871f40bff1de51de8b41c6f06d218f` |
| `NavMini-0.6.1-windows-amd64.exe` | 16.1 MB | `59e1cc42df48a8ea9369db52ab32582a6b7dc7cd9f7855ac306e4b81e79d948e` |
| `NavMini-0.6.1-windows-amd64.zip` | 6.4 MB | `983305a6c5cbd2eee87d8906e3dc249ea9a730a147d9f330b2c7130dd8e63512` |

## 安装

### macOS（Apple Silicon，macOS 13+）

打开 DMG → 拖入 Applications。被 Gatekeeper 拦截时右键打开，或：
`xattr -dr com.apple.quarantine /Applications/NavMini.app`

### Windows（x64）

双击 `NavMini.exe`，需要 WebView2 Runtime；单 exe 原生窗口，不使用浏览器。

# NavMini v0.6.3

发布日期：2026-11-06

## 点击库名变慢的排查结果

用 500 万行 `perf_lab.big_events` 实测：

- `/api/databases`：约 7 ms
- `/api/objects?db=perf_lab`：约 7 ms
- 点击库时没有任何 `/api/rows` 或 `/api/table` 请求

后端并没有查询表数据。真正的 UI 阻塞点是：

1. 点击库名时把该库加入 `openDbs`，然后调用 `renderTree()`；
2. 左侧树随即展开并渲染该库所有表/视图；
3. 表很多时会阻塞界面，看起来像“点击库在加载半天”；
4. 同时反复点击会重复发起 `/api/objects`，被 `runMu` 串行排队。

## 修复

- 点击库名**不再展开左侧表树**，只打开右侧表浏览器；
- 右键菜单的刷新仍会更新左侧树；
- `/api/objects` 增加按库的 single-flight，加载中重复点击不会重复请求；
- 刷新表列表不再触发整棵树重渲染；
- 保持打开库只查轻量元数据，打开表才查数据。

修复后点击 `perf_lab`：1 次 `/api/objects`，抽屉立即出现，500 万行表没有任何数据查询。

## 下载

| 文件 | 大小 | SHA-256 |
|---|---|---|
| `NavMini-0.6.3-macos-arm64.dmg` | 5.6 MB | `5bca71371f3722eef20471a0aeb6fbb529019b3ecb645aea3f4365db198954c3` |
| `NavMini-0.6.3-windows-amd64.exe` | 16.1 MB | `68ab4922c109f48cef55815a057064b51605cef4a9a1b755ff3944cdeb249001` |
| `NavMini-0.6.3-windows-amd64.zip` | 6.4 MB | `9eb6c0e3862fe8c9b27b30c9bb50a66f0b969157360d4d33b6a6f5d9ab149170` |

## 安装

### macOS（Apple Silicon，macOS 13+）

打开 DMG → 拖入 Applications。被 Gatekeeper 拦截时右键打开，或：
`xattr -dr com.apple.quarantine /Applications/NavMini.app`

### Windows（x64）

双击 `NavMini.exe`，需要 WebView2 Runtime；单 exe 原生窗口，不使用浏览器。

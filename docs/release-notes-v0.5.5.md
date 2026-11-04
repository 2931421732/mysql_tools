# NavMini v0.5.5

发布日期：2026-11-04

## 本次改动

- WHERE 条件与 ORDER BY 合并到一个紧凑块，左右布局，节省垂直空间
- 条件块改为在数据区上方展开，数据网格和底部状态栏仍然可见，不再被覆盖
- 表结构首次打开后缓存：关闭/重新打开表不再重复请求 `/api/table`
- 库右键「刷新库结构」、表右键「刷新表结构」、工具栏刷新都会使对应缓存失效并重新加载
- 数据库刷新时同步刷新该库已打开的表结构

## 下载

| 文件 | 大小 | SHA-256 |
|---|---|---|
| `NavMini-0.5.5-macos-arm64.dmg` | 5.5 MB | `04324f94935134a01251e1c0e1ca472279e14ece6b924002c8fc07f0ae70e9ce` |
| `NavMini-0.5.5-windows-amd64.exe` | 16.0 MB | `d9e6bf0a71b5c6df1a711d8408b6a82f4d96f1002c006d578d34fb89df8d712f` |
| `NavMini-0.5.5-windows-amd64.zip` | 6.4 MB | `27fd7547d2109e16ecadfabe2535ddc6bd7033b33c3b9a6bc51a4c2cc1f37a86` |

## 安装

### macOS（Apple Silicon，macOS 13+）

打开 DMG → 拖入 Applications。被 Gatekeeper 拦截时右键打开，或：
`xattr -dr com.apple.quarantine /Applications/NavMini.app`

### Windows（x64）

双击 `NavMini.exe`，需要 WebView2 Runtime；单 exe 原生窗口，不使用浏览器。

## 隐私

背景图片、连接配置、AI Key 仅保存在本机配置目录，不会进入 Git 或 Release。

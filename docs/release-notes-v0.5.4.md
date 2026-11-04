# NavMini v0.5.4

发布日期：2026-11-04

## 本次改动

- 连接树支持多选、全选、批量删除
  - 侧栏「多选」按钮
  - 每行复选框、全选、删除选中、退出
- NCX 导入冲突处理
  - 名称和连接配置相同的连接会提示
  - 可勾选「覆盖更新」
  - 不勾选则在原名称后新增 `_copy`（重复时 `_copy2` ...）
- 继续保留 NCX AES-128-CBC 密码解密/加密导出

## 下载

| 文件 | 大小 | SHA-256 |
|---|---|---|
| `NavMini-0.5.4-macos-arm64.dmg` | 5.5 MB | `b938c45ef3197b613789f2807df926bc514bd6eb2bedf53ac08615362a4f4eaa` |
| `NavMini-0.5.4-windows-amd64.exe` | 15.9 MB | `994f0516a78b1a2c64d654e9dec3380d00069e0ae452ae14318ee66f29188c05` |
| `NavMini-0.5.4-windows-amd64.zip` | 6.4 MB | `4ea0e175c579aa16073272cda0255dc48d63755982f2fd3959efa424ad73cadf` |

## 安装

### macOS（Apple Silicon，macOS 13+）

打开 DMG → 拖入 Applications。被 Gatekeeper 拦截时右键打开，或：
`xattr -dr com.apple.quarantine /Applications/NavMini.app`

### Windows（x64）

双击 `NavMini.exe`，需要 WebView2 Runtime；单 exe 原生窗口，不使用浏览器。

## 隐私

背景图片、连接配置、AI Key 仅保存在本机配置目录，不会进入 Git 或 Release。

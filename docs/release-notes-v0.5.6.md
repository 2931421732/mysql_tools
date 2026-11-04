# NavMini v0.5.6

发布日期：2026-11-04

## 修复

NCX 密码解密/加密的 padding 处理错误：

- 原实现按零填充去除尾部 `\x00`，但实际数据使用 **PKCS#7**；
- 导致密码尾部残留 `\x01`–`\x10` 控制字符，登录失败；
- 现在解密自动识别并移除 PKCS#7 padding，同时兼容零填充；
- 导出连接时也改为 PKCS#7，重新导出的 `.ncx` 可被兼容客户端正确解密。

> 之前已导入过 NCX 连接的，请用新版重新导入并选择「覆盖更新」以修正密码。

## 下载

| 文件 | 大小 | SHA-256 |
|---|---|---|
| `NavMini-0.5.6-macos-arm64.dmg` | 5.5 MB | `cf0d123ff65fa63c73213614c4575c20bc7d9fa73a6b7f177299626e181c513f` |
| `NavMini-0.5.6-windows-amd64.exe` | 16.0 MB | `aad6d2b6919dc1419f38e1fcc575f8a5bbc6396f16828b5a56202d74daf64ef9` |
| `NavMini-0.5.6-windows-amd64.zip` | 6.4 MB | `1a282ea94d04355b16faca20d79ad7e86f5f7db4efe302deb67b408611d9d39b` |

## 安装

### macOS（Apple Silicon，macOS 13+）

打开 DMG → 拖入 Applications。被 Gatekeeper 拦截时右键打开，或：
`xattr -dr com.apple.quarantine /Applications/NavMini.app`

### Windows（x64）

双击 `NavMini.exe`，需要 WebView2 Runtime；单 exe 原生窗口，不使用浏览器。

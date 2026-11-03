# NavMini v0.5.3

发布日期：2026-11-04

## 本次改动

- `.ncx` 导入现在支持 **AES-128-CBC** 密码解密（Navicat 12+，key/iv 与零填充）
- 新增 **导出连接 (.ncx)**：
  - 侧栏「导出」按钮
  - 可选择要导出的连接
  - 可选是否包含密码；密码使用相同的 AES-128-CBC 方式加密
- 只导入 MySQL / MariaDB 连接；其他 ConnType 自动跳过
- 支持 SSH 主机/端口/用户名/认证方式/密码/私钥字段
- 继续保留 v0.5.2 的 WHERE 条件、默认主键倒序、单行 SQL、行号对齐

## 下载

| 文件 | 大小 | SHA-256 |
|---|---|---|
| `NavMini-0.5.3-macos-arm64.dmg` | 5.5 MB | `faf8b8fe430e763610de7eb1058a1fb493e21e0d589ce8f417fa16575000a367` |
| `NavMini-0.5.3-windows-amd64.exe` | 15.9 MB | `c98584f9146a042b2283af27a4f1eaf483e273bc30440117331f2d641cfe7c16` |
| `NavMini-0.5.3-windows-amd64.zip` | 6.4 MB | `d1de3b244a4433f35f2af8e84379eea07e9b9d583d7e55b9aa16f87090717763` |

## 安装

### macOS（Apple Silicon，macOS 13+）

打开 DMG → 拖入 Applications。ad-hoc 签名、未公证；被拦截时右键打开，或：
`xattr -dr com.apple.quarantine /Applications/NavMini.app`

### Windows（x64）

双击 `NavMini.exe`，需要 WebView2 Runtime；单 exe 原生窗口，不使用浏览器。

## 安全

- 导出的 `.ncx` 若包含密码，密码使用 Navicat 兼容的 AES-128-CBC 加密；
  仍应按敏感凭据保管文件。
- 背景图片、连接配置、DeepSeek API Key 只保存在本机配置目录，不会进入 Git 或 Release。

# NavMini v0.5.1

发布日期：2026-11-04

## 本次改动

- Settings 新增「默认按主键倒序」，默认开启；没有主键时不生效
- 筛选条件默认展开；筛选栏新增字段输入提示，输入字段名回车即可快速添加条件
- 筛选/排序应用后生成**单行 SQL 摘要**，超长省略，可一键复制
- 修复 `#` 行号列在 10 行后变宽导致数据单元格错位的问题（行号列固定 52px）
- 继续包含 0.5.0 的 Windows 原生 WebView2、NCX 导入、详情面板、数据库导出、时间戳显示等

## 下载

| 文件 | 大小 | SHA-256 |
|---|---|---|
| `NavMini-0.5.1-macos-arm64.dmg` | 5.5 MB | `fb1acfa627efa344cab5812296dfbe7097f5b0ec57bbb4ddef7ac1a012362c3d` |
| `NavMini-0.5.1-windows-amd64.exe` | 15.9 MB | `36e7864c2b7877ad3086a2f351239b32b58c57c7d3f93a3659b545517915f851` |
| `NavMini-0.5.1-windows-amd64.zip` | 6.4 MB | `140d8ab5d5f65540608fc4db233dd2b7393146f7c6d576183f2e3ad4ff54c137` |

## 安装

### macOS（Apple Silicon，macOS 13+）

1. 打开 DMG，把 `NavMini.app` 拖入 Applications
2. ad-hoc 签名、未公证；被拦截时右键 App → 打开，或：
   `xattr -dr com.apple.quarantine /Applications/NavMini.app`

### Windows（x64）

1. 下载 exe 或 zip，双击 `NavMini.exe`
2. 需要 Microsoft Edge WebView2 Runtime（Win10/11 通常已预装）
3. 单 exe，原生 WebView2 窗口，不使用浏览器
4. 日志：`%LOCALAPPDATA%\NavMini\shell.log`

## 隐私

- 背景图片、连接配置、DeepSeek API Key 只保存在本机应用配置目录，不会进入 Git 或 Release
- 连接密码与 API Key 使用 AES-256-GCM 加密

## 已知限制

- 仅 MySQL
- NCX 仅支持明文 XML/JSON；加密/二进制不解密
- Windows 包在 macOS 上交叉编译，未在 Windows 本机运行验证
- 时间戳显示仅影响显示，编辑仍提交原始值

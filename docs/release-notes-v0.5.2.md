# NavMini v0.5.2

发布日期：2026-11-04

## 本次改动

- 去掉表格顶部的筛选条，改为表工具栏 **WHERE** 按钮
- 新增查询条件时在 WHERE 条件行内手动输入字段名（带候选提示）和值，
  不再使用顶部快速字段输入
- 条件默认展开；没有字段或值时不参与查询
- 保留默认主键倒序、单行 SQL 摘要复制、# 行号列对齐修复
- Windows 原生 WebView2 单 exe 同步更新

## 下载

| 文件 | 大小 | SHA-256 |
|---|---|---|
| `NavMini-0.5.2-macos-arm64.dmg` | 5.5 MB | `f35fc005d0ab73b519212d436812249de13edaa8e65f8207639bf8f9a859ceb1` |
| `NavMini-0.5.2-windows-amd64.exe` | 15.9 MB | `ba47a3a0e073ccc97a11c2103f94b55aa20b3481d5a1ea7113ffde8e0f1a4953` |
| `NavMini-0.5.2-windows-amd64.zip` | 6.4 MB | `113b7bbf15d2a710398de3777523377498ac53ef1592445225196510a503c8dc` |

## 安装

### macOS（Apple Silicon，macOS 13+）

打开 DMG → 拖入 Applications。ad-hoc 签名、未公证；被拦截时右键打开，或：
`xattr -dr com.apple.quarantine /Applications/NavMini.app`

### Windows（x64）

双击 `NavMini.exe`，需要 WebView2 Runtime（Win10/11 通常预装）；
单 exe 内嵌服务，不使用浏览器。

## 隐私

背景图片、连接配置、AI Key 仅保存在本机应用配置目录，不会进入 Git 或 Release。

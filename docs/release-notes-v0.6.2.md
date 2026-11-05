# NavMini v0.6.2

发布日期：2026-11-06

## 加载逻辑优化

排查后确认，之前打开库慢的主要原因是额外的统计查询，而不是真的读取了整库数据：

- 打开连接/库列表时，会聚合所有 `information_schema.TABLES` 的表数和大小；
- 打开库对象列表时，会读取 `TABLE_ROWS`、`DATA_LENGTH`、`INDEX_LENGTH`、`UPDATE_TIME`；
- 首次打开表数据时，会同步执行 `SELECT COUNT(*)`，大表会全表扫描或等到 3 秒超时。

现在改为：

- 库列表只查询 schema 名称/字符集/排序规则，不再聚合全库统计；
- 打开库只查询轻量表元数据：表名、类型、引擎、注释；
- 不再读取表行数/数据大小/索引大小/更新时间；
- 打开库时**不请求表数据**；
- 打开表时才查询结构并分页加载数据（`LIMIT 200`）；
- 首屏不再同步 `COUNT(*)`，避免大表全表扫描。

> 初始加载不再显示精确总行数；需要时可后续增加按需「统计行数」。

## 下载

| 文件 | 大小 | SHA-256 |
|---|---|---|
| `NavMini-0.6.2-macos-arm64.dmg` | 5.6 MB | `ff426a81989f293ce4a81cc3c09dafb030b4e8d7c22e02a2375fd067a200cbd2` |
| `NavMini-0.6.2-windows-amd64.exe` | 16.1 MB | `5bc8b3c9b22b3cff5b15e7b0f5921d33d37c0802b33f2d6e7e939d29dabaca1a` |
| `NavMini-0.6.2-windows-amd64.zip` | 6.4 MB | `9dce25f36f86877b7c8aa132cd3af6e836afa5715b799a5144702e9db2c8232d` |

## 安装

### macOS（Apple Silicon，macOS 13+）

打开 DMG → 拖入 Applications。被 Gatekeeper 拦截时右键打开，或：
`xattr -dr com.apple.quarantine /Applications/NavMini.app`

### Windows（x64）

双击 `NavMini.exe`，需要 WebView2 Runtime；单 exe 原生窗口，不使用浏览器。

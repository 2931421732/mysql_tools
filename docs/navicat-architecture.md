# Navicat Premium Lite 17.3.10 构造分析（REA 取证）

> 目标：`/Applications/Navicat Premium Lite.app`（arm64，v17.3.10，Bundle ID `com.navicat.NavicatPremiumLite`）
> 工具：REA 6.8.0（native-macos provider：`inspect_desktop_application`、`inspect_macho`、
> `inspect_signature`、`inspect_native_dispatch_metadata`、`inspect_analysis_view`）
> 产物：本文 + `rea-evidence.json`（REA 证据束，可复查）

本文把「观测到的事实」和「基于命名/依赖关系的推断」分开标注。本轮环境没有可用的反编译器
（Ghidra 缺 macOS arm64 decompile，Hopper/IDA 未安装），因此**没有函数级反编译**；结论来自
Mach-O 装载信息、签名、Objective-C 元数据协议、bundle 资源（nib/strings/dylib）与公开符号。

---

## 1. 交付形态与信任链

| 项目 | 观测值 | 来源 |
|---|---|---|
| Bundle 版本 | 17.3.10（`CFBundleShortVersionString`） | `inspect_plist` / Info.plist |
| 可执行文件 | Universal Mach-O（arm64 + x86_64），71,501,568 B | `inspect_macho` |
| SHA-256 | `10970609083b101330839cd72085863bb4c6aaecc9e30719c2bc0499a8029b8f` | `inspect_macho` |
| 签名 | Developer ID Application: PremiumSoft CyberTech Ltd. (VH7G2MRF27)，hardened runtime | `inspect_signature` |
| Entitlements | `allow-jit`、`allow-unsigned-executable-memory`、`disable-library-validation`、`automation.apple-events`、`keychain-access-groups`、App Group（与 Launcher 共享） | `inspect_signature` |
| Bundle 体积 | 约 1.1 GB，3,393 个文件 | `inspect_desktop_application` |
| 结构 | `Contents/{MacOS, Frameworks, Resources, Applications}` + 内嵌 Launcher.app + provisioning profile | `inspect_desktop_application` |

`disable-library-validation` + JIT + 可执行内存，说明宿主需要加载自签/自编译的 dylib 与 JS 引擎；
`keychain-access-groups` 与 `com.prect.Navicat.launcherGroup` 说明凭据走 Keychain、并由 Launcher
子 App 共享连接配置。

---

## 2. 进程装载结构

主可执行文件直接链接 62 个镜像（`inspect_analysis_view` macho/dependencies，证据
`ev_23ed7568…`），可以分成五层：

```
AppKit / SwiftUI / WebKit / PDFKit / Quartz / Security / Kerberos / libobjc / libc++
        ▲
Navicat Premium Lite (UI shell, 71 MB)
  ├─ 私有 UI 框架: NAVTabBarView, qb(Query Builder), EE, FileMonitor, HexFiend
  ├─ 功能 dylib:   libcc(154 MB) libcf libdv libmv libsv libvf libstat
  │                libqexp libxl libnpdf libncharts libndataprofiling libnai libaggrpipe
  ├─ 数据访问:     libmariadb / libpq / libsqlite3mc / libmongoc+bson / libredis++
  │                libtdsodbc(MS SQL) / Frameworks/arm64/libclntsh*(Oracle Instant Client)
  └─ 基础设施:     libssl/libcrypto, libssh, libmozjs-115, Sparkle, icu/glib/cairo…
```

要点：

- `libcc.dylib` 是**总装层**：它自己不链数据库驱动，而是链 `libcf/libdv/libmv/libqexp/libnai/
  libncharts/libndataprofiling/libaggrpipe/libxl/libnpdf` 以及 `libssh`、`libmozjs-115`、
  `libredis++`、`libmongoc`（证据 `ev_b281fd95…`）。这与「核心服务 + 功能插件」的分层一致。
- 数据库驱动以独立 dylib 形态存在（MariaDB/MySQL、PostgreSQL、SQLite、MongoDB、Redis、
  MS SQL via ODBC、Oracle），主程序不直接绑定某一家协议。
- `libmozjs-115`（SpiderMonkey）+ `Resources/acorn.js`、`walk.js`、`nav_*.js` 表明脚本层是
  真实存在的：`nav_mongo.js`、`nav_collection.js`、`nav_cursor.js` 等对应 mongosh 风格 API，
  `acorn` 用于语法分析（查询编辑器/BI 计算字段）。
- 更新用 Sparkle 2.8；`EE.framework`、`qb.framework` 为闭源组件。

---

## 3. UI 的「模块化表单」构造

`Resources/Base.lproj` 中有 **710 个 nib**，`zh-Hans.lproj` 有 **572 个 .strings**。命名本身就是
架构文档：

```
<Feature><Part>Module_<rdbms>.nib
ConnectionFormGeneralModule_{mysql,mariadb,pgsql,ora,mssql,sqlite,mongodb,redis,snowflake}.nib
ConnectionFormAdvancedModule_*.nib
ConnectionFormSSHModule.nib / ConnectionFormSSLModule.nib / ConnectionFormHTTPTunnelModule.nib
SQLEditorModule.nib / SQLResultDatasetModule.nib / SQLResultMessageModule.nib
SQLResultInfoModule.nib / SQLResultSummaryModule.nib
TableDesign{Fields,Indexes,ForeignKeys,Triggers,Checks,Options,Partitions}Module_mysql.nib
MainWindowController.nib / NAVConnectionTreeViewController.nib / NAVTabBarView
```

Objective-C 元数据（`inspect_native_dispatch_metadata`，证据 `ev_2f39e335…`）给出了驱动力：

- `CCUIFormBehaviourProtocol` — 统一「表单行为」协议（新建/设计/复制/打开、是否复用已有 Tab）。
- `NAVFormModuleOwner` — 表单模块宿主（`isEdited`、`moduleTitleOfModule:`、剪贴板）。
- `SQLEditorModuleOwner` — 编辑器与宿主解耦（连接类型、holder、snippet、是否加载文本）。
- `SQLResultDatasetModule` / `SQLResultMessageModule` / `SQLResultInfoModule` /
  `SQLResultSummaryModule` — 一个查询的结果被拆成四种可独立摆放的模块。
- `ConsoleTextAreaViewDelegate` — 控制台执行/中止/历史导航/自动补全。
- `NAVTableViewDelegate`、`NAVFieldSelectionFrameDataSource` — 网格与字段选择的数据源协议。
- 方言差异只在 `_mysql/_pgsql/_mssql/_ora/_mongodb/_redis/_snowflake` 后缀模块里，主 UI 共用。

**推断**：一个窗口/标签由「Form Service + Module Owner + N 个 Module（nib）」拼装，连接类型通过
trait/模块后缀注入，而不是 if/else 堆在窗口控制器里。这与 `IConsoleV2ConnTypeTrait`
（`createConsoleFormService`、`setupExtraForCSForm:`）的命名相互印证。

---

## 4. 功能面（按 nib 清单归纳）

| 域 | 代表模块 |
|---|---|
| 连接管理 | ConnectionForm{General,Advanced,SSH,SSL,HTTPTunnel}, ManageConnection, ConnectionTreeFilter |
| SQL 工作台 | SQLEditor, QueryDesign, QueryExplain(Visual/Stat), SQLPreview, Snippet, ConsoleV2 |
| 结果与数据 | SQLResult{ Dataset, Info, Message, Summary }, GridView, FilterAndSortManager, DataFilterSettingPane |
| 表设计 | TableDesign{ Fields, Indexes, ForeignKeys, Triggers, Checks, Options, Partitions, Storage } |
| 对象设计 | View/Function/Event/Sequence/User/Role/Privilege/Tablespace…（每种对象一组 General+Advanced） |
| 建模 | ERDiagram, ModelWorkspace*, ModelSynchronization, DataDictionary |
| 传输/同步 | ImportWizard, ExportWizard, DataTransfer, DataSynchronization, StructureSynchronization |
| 运维 | ServerMonitor{Status,Process,Variable,Cluster}, Backup/Restore, Schedule/Automation, Debugger |
| BI / AI | Charts*(DataSource/Dashboard/ChartDesign), AIAssistant*(AttachSchema/QueryDesign/Chat) |
| 其它 | DataProfiling, DataGeneration, GridFS, Redis PubSub, Cloud, URIHandler |

这是一套「一个核心 + 一组对象设计器 + 一组批处理向导」的典型数据库 IDE 构造。

---

## 5. 数据流（根据模块边界推断，未做运行时观测）

```
NIB Module (UI)
   │  owner protocol / form service
   ▼
libcc  (会话、对象模型、命令调度、结果集)
   ├─ libcf   连接/表单服务
   ├─ libdv   数据视图（网格、过滤、排序）
   ├─ libqexp 查询/导出
   ├─ libmv   模型（ER）
   ├─ libnai  AI 助手
   └─ libncharts / libndataprofiling / libstat / libxl / libnpdf
   ▼
driver dylib (mariadb / pq / sqlite3mc / mongoc / redis++ / odbc / oracle)
   ▼
DB server
```

SSH/SSL/HTTP Tunnel 属于连接层，不是驱动层；因此同一驱动可以套不同隧道——这对新工具是可直接
借鉴的边界划分。

---

## 6. 值得仿照的 6 条设计原则

1. **UI 模块化**：窗口只做布局，功能以 Module(nib) + Owner 协议组合，结果集拆成四个可复用面板。
2. **方言隔离**：通用工作台 + `_<dialect>` 扩展模块，而不是为每个数据库复制一套 UI。
3. **连接即配置树**：General / Advanced / SSH / SSL / HTTP Tunnel 分层，凭据交给系统钥匙串。
4. **驱动插件化**：核心不绑定协议，驱动是独立动态库，便于增量支持新数据库。
5. **延迟加载的对象树**：连接 → 库 → 表/视图/函数 → 列/索引，逐级拉取，避免一次性元数据风暴。
6. **一切可脚本化**：内置 JS 引擎 + 资源脚本，把批处理、转换、BI 计算统一到脚本层。

---

## 7. 证据与局限

| 证据 | ID |
|---|---|
| Bundle 清单（3,393 项） | `ev_924edc0b0076fb6ae3a08487f71d4d341dea79be5832e5a30c3dd52625943021` |
| 主程序 Mach-O（80 load commands / 62 deps / 2,125 imports） | `ev_64db6fc7d157d31e93637f02cfbd3bb90e6493334825eee53cb1ba2265a2a5b3` |
| 主程序依赖页（62 条） | `ev_23ed75681b7d7401127ee5295311881769740b94e4e4cceacc18329311735921` |
| 代码签名 / entitlements | `ev_3c227f824f61114b4dd29f5ad2bb054c2d98f35ab552c6caf8d42dde88e6d11e` |
| ObjC 协议/方法元数据 | `ev_2f39e33511dd359f7dd26c4d059800de5a0000199d505c9cd83e83a05ecddef2` |
| libcc Mach-O（54 load commands / 2,945 exports） | `ev_985b999f57eda0ad0f3046a4c4a06b50f9fcb7381c4b446d57201d194bd4b1d0` |
| libcc 依赖页（40 条） | `ev_b281fd95dedba8ddee7b7d687a4b5e67b43ef434fcf833cb614df644982bdb6e` |

局限：

- 反编译器不可用（Ghidra 缺 `macos/arm64/decompile`），未做函数级伪代码；行为结论均为静态推断。
- ObjC **类**列表在 600 条预算内为空（协议已解码至上限），类实现细节未展开。
- 依赖来自 `otool/dyld_info/nm`，剥离符号可能缺失；`source_exhaustive=false` 的集合不能当全集。
- 未运行 App，未观测网络/数据库流量；`libcc` 内部调用链未验证。

---

## 8. 新工具：NavMini 的映射

新工具用 **Go** 实现（理由见 `README.md`），把上面的构造压缩成「单进程 + 服务层 + 驱动层 + 模块化
Web UI」：

| Navicat | NavMini |
|---|---|
| AppKit 外壳 + NIB | 原生 WKWebView 外壳（Swift）+ 单页 UI |
| libcc + libcf/libdv/libqexp | `internal/mysqlx`（会话/元数据/查询/表编辑/隧道） |
| libmariadb / libpq / … | `database/sql` + `go-sql-driver/mysql`（驱动接口预留） |
| ConnectionForm{General,SSH,SSL} | 连接对话框（常规 / SSH / SSL），AES-GCM 本地保管凭据 |
| SQLEditor + 4 个结果模块 | SQL 编辑器 + 结果/消息/耗时/行数 四区 |
| TableDesign* | Data / Structure / DDL 三视图 + 基于主键的行编辑与 SQL 预览 |
| ServerMonitor | 进程列表 / 状态变量 |
| 710 个 nib | 6 个 UI 模块（连接树、编辑器、网格、表视图、监控、设置） |

实现范围与限制见 `README.md`。

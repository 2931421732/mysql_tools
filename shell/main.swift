import Cocoa
import WebKit

final class AppDelegate: NSObject, NSApplicationDelegate, NSWindowDelegate, WKUIDelegate {
    private var window: NSWindow!
    private var webView: WKWebView!
    private var server: Process?
    private var outputBuffer = Data()
    private var serverURL: URL?

    func applicationDidFinishLaunching(_ notification: Notification) {
        buildMenu()
        buildWindow()
        startServer()
    }

    func applicationShouldTerminateAfterLastWindowClosed(_ sender: NSApplication) -> Bool { true }

    func applicationWillTerminate(_ notification: Notification) {
        stopServer()
    }

    private func buildWindow() {
        let config = WKWebViewConfiguration()
        config.preferences.setValue(true, forKey: "developerExtrasEnabled")
        config.websiteDataStore = .default()
        // The window uses .fullSizeContentView so the traffic-light buttons sit
        // over the web content. Tell the SPA to reserve a top inset.
        config.userContentController.addUserScript(WKUserScript(
            source: "window.__NAVMINI_NATIVE__ = true;",
            injectionTime: .atDocumentStart,
            forMainFrameOnly: true
        ))

        webView = WKWebView(frame: .zero, configuration: config)
        webView.uiDelegate = self
        webView.setValue(false, forKey: "drawsBackground")
        if #available(macOS 12.0, *) {
            webView.underPageBackgroundColor = NSColor(calibratedRed: 0.07, green: 0.08, blue: 0.10, alpha: 1)
        }

        window = NSWindow(
            contentRect: NSRect(x: 0, y: 0, width: 1440, height: 900),
            styleMask: [.titled, .closable, .miniaturizable, .resizable, .fullSizeContentView],
            backing: .buffered,
            defer: false
        )
        window.title = "NavMini"
        window.titlebarAppearsTransparent = true
        window.backgroundColor = NSColor(calibratedRed: 0.07, green: 0.08, blue: 0.10, alpha: 1)
        window.minSize = NSSize(width: 960, height: 620)
        window.contentView = webView
        window.delegate = self
        window.setFrameAutosaveName("NavMini.MainWindow")
        window.center()
        window.makeKeyAndOrderFront(nil)
        NSApp.activate(ignoringOtherApps: true)
    }

    private func startServer() {
        guard let resourceURL = Bundle.main.resourceURL else {
            loadError("无法定位应用资源目录。")
            return
        }
        let binary = resourceURL.appendingPathComponent("navmini")
        guard FileManager.default.isExecutableFile(atPath: binary.path) else {
            loadError("缺少内置的 navmini 可执行文件。\n请先运行 scripts/build-app.sh。")
            return
        }

        let process = Process()
        process.executableURL = binary
        process.arguments = ["--listen", "127.0.0.1:0", "--open=false"]
        var environment = ProcessInfo.processInfo.environment
        if environment["NAVMINI_HOME"] == nil {
            environment["NAVMINI_HOME"] = ("~/Library/Application Support/NavMini" as NSString).expandingTildeInPath
        }
        process.environment = environment

        let pipe = Pipe()
        process.standardOutput = pipe
        process.standardError = FileHandle.standardError
        pipe.fileHandleForReading.readabilityHandler = { [weak self] handle in
            let data = handle.availableData
            guard let self, !data.isEmpty else { return }
            self.consume(data)
        }
        process.terminationHandler = { [weak self] proc in
            DispatchQueue.main.async {
                guard let self else { return }
                if self.serverURL == nil {
                    self.loadError("NavMini 服务启动失败（退出码 \(proc.terminationStatus)）。")
                }
            }
        }

        do {
            try process.run()
            self.server = process
            DispatchQueue.main.asyncAfter(deadline: .now() + 15) { [weak self] in
                guard let self, self.serverURL == nil else { return }
                self.loadError("启动超时：NavMini 服务没有在 15 秒内就绪。")
            }
        } catch {
            loadError("无法启动 NavMini 服务：\(error.localizedDescription)")
        }
    }

    private func consume(_ data: Data) {
        outputBuffer.append(data)
        while let newline = outputBuffer.firstIndex(of: 0x0A) {
            let lineData = outputBuffer.prefix(upTo: newline)
            outputBuffer.removeSubrange(outputBuffer.startIndex...newline)
            guard let line = String(data: lineData, encoding: .utf8) else { continue }
            if line.hasPrefix("NAVMINI_READY ") {
                let raw = String(line.dropFirst("NAVMINI_READY ".count)).trimmingCharacters(in: .whitespacesAndNewlines)
                guard let url = URL(string: raw) else { continue }
                DispatchQueue.main.async { [weak self] in
                    self?.serverURL = url
                    self?.webView.load(URLRequest(url: url))
                }
            }
        }
    }

    private func stopServer() {
        guard let server, server.isRunning else { return }
        server.terminate()
        let deadline = Date().addingTimeInterval(2)
        while server.isRunning && Date() < deadline {
            usleep(50_000)
        }
        if server.isRunning {
            kill(server.processIdentifier, SIGKILL)
        }
    }

    func windowWillClose(_ notification: Notification) {
        stopServer()
    }

    // MARK: - WKUIDelegate

    // WKWebView does not open a file picker on macOS without a UI delegate.
    // This is what the Settings → Background image / file input relies on.
    func webView(_ webView: WKWebView,
                 runOpenPanelWith parameters: WKOpenPanelParameters,
                 initiatedByFrame frame: WKFrameInfo,
                 completionHandler: @escaping ([URL]?) -> Void) {
        let panel = NSOpenPanel()
        panel.canChooseFiles = true
        panel.canChooseDirectories = parameters.allowsDirectories
        panel.allowsMultipleSelection = parameters.allowsMultipleSelection
        panel.allowsOtherFileTypes = true
        if let host = webView.window {
            panel.beginSheetModal(for: host) { response in
                completionHandler(response == .OK ? panel.urls : nil)
            }
        } else {
            panel.begin { response in
                completionHandler(response == .OK ? panel.urls : nil)
            }
        }
    }

    private func loadError(_ message: String) {
        let escaped = message
            .replacingOccurrences(of: "&", with: "&amp;")
            .replacingOccurrences(of: "<", with: "&lt;")
            .replacingOccurrences(of: ">", with: "&gt;")
            .replacingOccurrences(of: "\n", with: "<br>")
        let html = """
        <html><head><meta charset="utf-8"><style>
        body{margin:0;height:100vh;display:flex;align-items:center;justify-content:center;
        background:#101318;color:#dbe2ea;font:14px -apple-system,"PingFang SC",sans-serif}
        .box{max-width:520px;padding:28px 32px;border:1px solid #2a313b;border-radius:12px;background:#1a1f26}
        h1{font-size:18px;margin:0 0 10px;color:#33c9a4}p{line-height:1.7;color:#9aa5b1;margin:0}
        </style></head><body><div class="box"><h1>NavMini 无法启动</h1><p>\(escaped)</p></div></body></html>
        """
        webView.loadHTMLString(html, baseURL: nil)
    }

    // MARK: - Menu

    private func buildMenu() {
        let mainMenu = NSMenu()

        let appItem = NSMenuItem()
        mainMenu.addItem(appItem)
        let appMenu = NSMenu()
        appItem.submenu = appMenu
        appMenu.addItem(withTitle: "关于 NavMini", action: #selector(showAbout), keyEquivalent: "")
        appMenu.addItem(.separator())
        appMenu.addItem(withTitle: "设置…", action: #selector(showSettings), keyEquivalent: ",")
        appMenu.addItem(.separator())
        appMenu.addItem(withTitle: "隐藏 NavMini", action: #selector(NSApplication.hide(_:)), keyEquivalent: "h")
        let hideOthers = NSMenuItem(title: "隐藏其他", action: #selector(NSApplication.hideOtherApplications(_:)), keyEquivalent: "h")
        hideOthers.keyEquivalentModifierMask = [.command, .option]
        appMenu.addItem(hideOthers)
        appMenu.addItem(withTitle: "显示全部", action: #selector(NSApplication.unhideAllApplications(_:)), keyEquivalent: "")
        appMenu.addItem(.separator())
        appMenu.addItem(withTitle: "退出 NavMini", action: #selector(NSApplication.terminate(_:)), keyEquivalent: "q")

        let fileItem = NSMenuItem()
        mainMenu.addItem(fileItem)
        let fileMenu = NSMenu(title: "文件")
        fileItem.submenu = fileMenu
        fileMenu.addItem(withTitle: "新建查询", action: #selector(newQuery), keyEquivalent: "t")
        fileMenu.addItem(withTitle: "新建连接", action: #selector(newConnection), keyEquivalent: "n")
        fileMenu.addItem(.separator())
        fileMenu.addItem(withTitle: "关闭标签页", action: #selector(closeTab), keyEquivalent: "w")
        fileMenu.addItem(withTitle: "刷新界面", action: #selector(reloadPage), keyEquivalent: "r")

        let editItem = NSMenuItem()
        mainMenu.addItem(editItem)
        let editMenu = NSMenu(title: "编辑")
        editItem.submenu = editMenu
        editMenu.addItem(withTitle: "撤销", action: Selector(("undo:")), keyEquivalent: "z")
        let redo = NSMenuItem(title: "重做", action: Selector(("redo:")), keyEquivalent: "z")
        redo.keyEquivalentModifierMask = [.command, .shift]
        editMenu.addItem(redo)
        editMenu.addItem(.separator())
        editMenu.addItem(withTitle: "剪切", action: #selector(NSText.cut(_:)), keyEquivalent: "x")
        editMenu.addItem(withTitle: "复制", action: #selector(NSText.copy(_:)), keyEquivalent: "c")
        editMenu.addItem(withTitle: "粘贴", action: #selector(NSText.paste(_:)), keyEquivalent: "v")
        editMenu.addItem(withTitle: "全选", action: #selector(NSText.selectAll(_:)), keyEquivalent: "a")

        let viewItem = NSMenuItem()
        mainMenu.addItem(viewItem)
        let viewMenu = NSMenu(title: "显示")
        viewItem.submenu = viewMenu
        viewMenu.addItem(withTitle: "运行查询", action: #selector(runQuery), keyEquivalent: "\r")
        viewMenu.addItem(withTitle: "查询历史", action: #selector(showHistory), keyEquivalent: "k")
        viewMenu.addItem(.separator())
        viewMenu.addItem(withTitle: "放大", action: #selector(zoomIn), keyEquivalent: "+")
        viewMenu.addItem(withTitle: "缩小", action: #selector(zoomOut), keyEquivalent: "-")
        viewMenu.addItem(withTitle: "实际大小", action: #selector(zoomReset), keyEquivalent: "0")
        viewMenu.addItem(.separator())
        viewMenu.addItem(withTitle: "进入全屏幕", action: #selector(NSWindow.toggleFullScreen(_:)), keyEquivalent: "f")

        let windowItem = NSMenuItem()
        mainMenu.addItem(windowItem)
        let windowMenu = NSMenu(title: "窗口")
        windowItem.submenu = windowMenu
        windowMenu.addItem(withTitle: "最小化", action: #selector(NSWindow.performMiniaturize(_:)), keyEquivalent: "m")
        windowMenu.addItem(withTitle: "缩放", action: #selector(NSWindow.performZoom(_:)), keyEquivalent: "")
        NSApp.windowsMenu = windowMenu

        NSApp.mainMenu = mainMenu
    }

    @objc private func showAbout() { eval("window.__navmini?.about()") }
    @objc private func showSettings() { eval("window.__navmini?.settings()") }
    @objc private func newQuery() { eval("window.__navmini?.newQuery()") }
    @objc private func newConnection() { eval("window.__navmini?.newConnection()") }
    @objc private func closeTab() { eval("window.__navmini?.closeTab()") }
    @objc private func runQuery() { eval("window.__navmini?.run()") }
    @objc private func showHistory() { eval("window.__navmini?.history()") }
    @objc private func reloadPage() { if let url = serverURL { webView.load(URLRequest(url: url)) } }
    @objc private func zoomIn() { webView.pageZoom = min(2.0, webView.pageZoom + 0.1) }
    @objc private func zoomOut() { webView.pageZoom = max(0.6, webView.pageZoom - 0.1) }
    @objc private func zoomReset() { webView.pageZoom = 1.0 }

    private func eval(_ js: String) {
        webView.evaluateJavaScript(js, completionHandler: nil)
    }
}

let app = NSApplication.shared
let delegate = AppDelegate()
app.delegate = delegate
app.setActivationPolicy(.regular)
app.run()

//go:build windows

// NavMini Windows shell: a native WebView2 window around the embedded Go
// server. No external browser is involved.
package main

import (
	"bufio"
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unsafe"

	webview2 "github.com/jchv/go-webview2"
)

//go:embed server.exe
var serverExe []byte

const version = "0.6.0"

func main() {
	base := filepath.Join(os.Getenv("LOCALAPPDATA"), "NavMini")
	if base == "" || base == "NavMini" {
		base = filepath.Join(os.TempDir(), "NavMini")
	}
	_ = os.MkdirAll(base, 0o755)
	logPath := filepath.Join(base, "shell.log")
	logFile, _ := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	logf := func(format string, args ...any) {
		if logFile != nil {
			fmt.Fprintf(logFile, time.Now().Format(time.RFC3339)+" "+format+"\n", args...)
		}
	}

	serverPath := filepath.Join(base, "navmini-server.exe")
	if err := os.WriteFile(serverPath, serverExe, 0o755); err != nil {
		fail("无法释放 NavMini 服务：" + err.Error())
		return
	}
	home := os.Getenv("NAVMINI_HOME")
	if home == "" {
		home = filepath.Join(os.Getenv("APPDATA"), "NavMini")
		if home == "NavMini" {
			home = filepath.Join(base, "config")
		}
	}
	cmd := exec.Command(serverPath, "--listen", "127.0.0.1:0", "--open=false")
	cmd.Env = append(os.Environ(), "NAVMINI_HOME="+home)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		fail("无法读取服务输出：" + err.Error())
		return
	}
	if logFile != nil {
		cmd.Stderr = logFile
	}
	if err := cmd.Start(); err != nil {
		fail("无法启动 NavMini 服务：" + err.Error())
		return
	}
	defer func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
	}()

	urlCh := make(chan string, 1)
	go func() {
		sc := bufio.NewScanner(stdout)
		for sc.Scan() {
			line := sc.Text()
			logf("%s", line)
			if strings.HasPrefix(line, "NAVMINI_READY ") {
				select {
				case urlCh <- strings.TrimSpace(strings.TrimPrefix(line, "NAVMINI_READY ")):
				default:
				}
			}
		}
	}()

	var url string
	select {
	case url = <-urlCh:
	case <-time.After(20 * time.Second):
		fail("NavMini 服务启动超时。日志：" + logPath)
		return
	}
	logf("server ready: %s", url)

	w := webview2.NewWithOptions(webview2.WebViewOptions{
		Debug:     false,
		AutoFocus: true,
		DataPath:  filepath.Join(base, "WebView2"),
		WindowOptions: webview2.WindowOptions{
			Title:  "NavMini " + version,
			Width:  1440,
			Height: 900,
			Center: true,
		},
	})
	if w == nil {
		fail("无法创建 WebView2 窗口。请安装 Microsoft Edge WebView2 Runtime。")
		return
	}
	defer w.Destroy()
	w.Navigate(url)
	w.Run()
	logf("window closed")
}

func fail(message string) {
	p, _ := syscall.UTF16PtrFromString(message)
	user32 := syscall.NewLazyDLL("user32.dll")
	proc := user32.NewProc("MessageBoxW")
	_, _, _ = proc.Call(0, uintptr(unsafe.Pointer(p)), uintptr(unsafe.Pointer(mustUTF16("NavMini"))), 0x10)
}

func mustUTF16(s string) *uint16 {
	p, _ := syscall.UTF16PtrFromString(s)
	return p
}

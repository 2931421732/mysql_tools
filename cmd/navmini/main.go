package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"

	"navmini/internal/config"
	"navmini/internal/mysqlx"
	"navmini/internal/server"
)

var version = "0.5.5"

func main() {
	listen := flag.String("listen", "127.0.0.1:0", "listen address (host:port; port 0 picks a free port)")
	home := flag.String("home", "", "config directory (default ~/.navmini)")
	open := flag.Bool("open", true, "open the UI in the default browser")
	tokenFlag := flag.String("token", "", "fixed API token (default: random per run)")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println("NavMini", version)
		return
	}

	store, err := config.Open(*home)
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	if err := os.MkdirAll(store.Home(), 0o700); err != nil {
		log.Fatalf("config: %v", err)
	}

	token := strings.TrimSpace(*tokenFlag)
	if token == "" {
		b := make([]byte, 24)
		if _, err := rand.Read(b); err != nil {
			log.Fatalf("token: %v", err)
		}
		token = hex.EncodeToString(b)
	}

	mgr := mysqlx.NewManager()
	defer mgr.CloseAll()
	go mgr.Reap(30*time.Minute, 5*time.Minute)

	ln, err := net.Listen("tcp", *listen)
	if err != nil {
		log.Fatalf("listen %s: %v", *listen, err)
	}
	addr, _ := ln.Addr().(*net.TCPAddr)
	host := "127.0.0.1"
	if addr != nil && !addr.IP.IsUnspecified() {
		host = addr.IP.String()
	}
	url := fmt.Sprintf("http://%s:%d/?token=%s", host, addr.Port, token)

	// The macOS shell reads this single line to discover the URL and token.
	fmt.Printf("NAVMINI_READY %s\n", url)
	os.Stdout.Sync()

	handler := server.New(store, mgr, token, version).Handler()
	srv := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 15 * time.Second,
		IdleTimeout:       5 * time.Minute,
	}

	if *open {
		go func() {
			time.Sleep(150 * time.Millisecond)
			if err := openBrowser(url); err != nil {
				log.Printf("open browser: %v (open %s manually)", err, url)
			}
		}()
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sig
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
		mgr.CloseAll()
	}()

	log.Printf("NavMini %s listening on %s (config %s)", version, url, store.Home())
	if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
		log.Fatalf("serve: %v", err)
	}
}

func openBrowser(url string) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", url).Start()
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	default:
		if _, err := exec.LookPath("xdg-open"); err == nil {
			return exec.Command("xdg-open", url).Start()
		}
		return fmt.Errorf("no browser launcher found")
	}
}

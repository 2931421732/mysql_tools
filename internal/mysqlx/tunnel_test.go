package mysqlx

import (
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	"io"
	"net"
	"strconv"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"

	"navmini/internal/config"
)

func TestTunnelForwards(t *testing.T) {
	target := startEchoServer(t)
	host, port := startTestSSHServer(t, "demo", "secret")
	tun, err := StartTunnel(config.SSHConfig{
		Enabled: true, Host: host, Port: port, User: "demo", Auth: "password", Password: "secret",
	}, target)
	if err != nil {
		t.Fatalf("StartTunnel: %v", err)
	}
	defer tun.Close()
	if tun.Fingerprint() == "" {
		t.Error("expected a host key fingerprint")
	}

	conn, err := net.DialTimeout("tcp", tun.Addr(), 3*time.Second)
	if err != nil {
		t.Fatalf("dial tunnel: %v", err)
	}
	defer conn.Close()
	if _, err := conn.Write([]byte("ping")); err != nil {
		t.Fatalf("write: %v", err)
	}
	buf := make([]byte, 4)
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	if _, err := io.ReadFull(conn, buf); err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(buf) != "ping" {
		t.Fatalf("got %q", buf)
	}
}

func startEchoServer(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				_, _ = io.Copy(c, c)
			}(conn)
		}
	}()
	return ln.Addr().String()
}

func startTestSSHServer(t *testing.T, user, pass string) (string, int) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.NewSignerFromKey(key)
	if err != nil {
		t.Fatal(err)
	}
	cfg := &ssh.ServerConfig{
		PasswordCallback: func(c ssh.ConnMetadata, pw []byte) (*ssh.Permissions, error) {
			if c.User() == user && string(pw) == pass {
				return nil, nil
			}
			return nil, fmt.Errorf("denied")
		},
	}
	cfg.AddHostKey(signer)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go serveSSH(conn, cfg)
		}
	}()
	addr := ln.Addr().(*net.TCPAddr)
	return addr.IP.String(), addr.Port
}

func serveSSH(conn net.Conn, cfg *ssh.ServerConfig) {
	_, chans, reqs, err := ssh.NewServerConn(conn, cfg)
	if err != nil {
		_ = conn.Close()
		return
	}
	go ssh.DiscardRequests(reqs)
	for newCh := range chans {
		if newCh.ChannelType() != "direct-tcpip" {
			_ = newCh.Reject(ssh.UnknownChannelType, "unsupported")
			continue
		}
		var req struct {
			Host       string
			Port       uint32
			OriginHost string
			OriginPort uint32
		}
		if err := ssh.Unmarshal(newCh.ExtraData(), &req); err != nil {
			_ = newCh.Reject(ssh.ConnectionFailed, "bad payload")
			continue
		}
		upstream, err := net.Dial("tcp", net.JoinHostPort(req.Host, strconv.Itoa(int(req.Port))))
		if err != nil {
			_ = newCh.Reject(ssh.ConnectionFailed, err.Error())
			continue
		}
		ch, chReqs, err := newCh.Accept()
		if err != nil {
			_ = upstream.Close()
			continue
		}
		go ssh.DiscardRequests(chReqs)
		go func() { _, _ = io.Copy(ch, upstream); _ = ch.Close() }()
		go func() { _, _ = io.Copy(upstream, ch); _ = upstream.Close() }()
	}
}

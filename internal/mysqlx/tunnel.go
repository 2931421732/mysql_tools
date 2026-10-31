package mysqlx

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"

	"navmini/internal/config"
)

// Tunnel is a local TCP listener forwarded through an SSH connection.
type Tunnel struct {
	listener    net.Listener
	client      *ssh.Client
	fingerprint string
	once        sync.Once
}

func StartTunnel(cfg config.SSHConfig, target string) (*Tunnel, error) {
	if cfg.Host == "" || cfg.User == "" {
		return nil, errors.New("SSH host and user are required")
	}
	port := cfg.Port
	if port == 0 {
		port = 22
	}
	sshAddr := net.JoinHostPort(cfg.Host, fmt.Sprintf("%d", port))

	auths, err := sshAuthMethods(cfg)
	if err != nil {
		return nil, err
	}
	if len(auths) == 0 {
		return nil, errors.New("no usable SSH credentials (set a password, a private key, or start ssh-agent)")
	}

	var fingerprint string
	clientCfg := &ssh.ClientConfig{
		User:    cfg.User,
		Auth:    auths,
		Timeout: 12 * time.Second,
		HostKeyCallback: func(_ string, _ net.Addr, key ssh.PublicKey) error {
			fingerprint = ssh.FingerprintSHA256(key)
			return nil
		},
	}

	dialer := net.Dialer{Timeout: 12 * time.Second}
	raw, err := dialer.Dial("tcp", sshAddr)
	if err != nil {
		return nil, fmt.Errorf("dial SSH %s: %w", sshAddr, err)
	}
	cc, chans, reqs, err := ssh.NewClientConn(raw, sshAddr, clientCfg)
	if err != nil {
		_ = raw.Close()
		return nil, fmt.Errorf("SSH handshake: %w", err)
	}
	client := ssh.NewClient(cc, chans, reqs)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		_ = client.Close()
		return nil, err
	}
	t := &Tunnel{listener: ln, client: client, fingerprint: fingerprint}
	go t.serve(target)
	return t, nil
}

func sshAuthMethods(cfg config.SSHConfig) ([]ssh.AuthMethod, error) {
	var out []ssh.AuthMethod
	switch cfg.Auth {
	case "key":
		path := expandHome(cfg.KeyPath)
		if path == "" {
			return nil, errors.New("private key path is empty")
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read private key: %w", err)
		}
		var signer ssh.Signer
		if cfg.Password != "" {
			signer, err = ssh.ParsePrivateKeyWithPassphrase(raw, []byte(cfg.Password))
		} else {
			signer, err = ssh.ParsePrivateKey(raw)
		}
		if err != nil {
			return nil, fmt.Errorf("parse private key: %w", err)
		}
		out = append(out, ssh.PublicKeys(signer))
	case "agent":
		sock := os.Getenv("SSH_AUTH_SOCK")
		if sock == "" {
			return nil, errors.New("SSH_AUTH_SOCK is not set")
		}
		conn, err := net.Dial("unix", sock)
		if err != nil {
			return nil, fmt.Errorf("connect ssh-agent: %w", err)
		}
		out = append(out, ssh.PublicKeysCallback(agent.NewClient(conn).Signers))
	default:
		if cfg.Password != "" {
			out = append(out, ssh.Password(cfg.Password))
		}
	}
	return out, nil
}

func (t *Tunnel) serve(target string) {
	for {
		local, err := t.listener.Accept()
		if err != nil {
			return
		}
		go func(local net.Conn) {
			remote, err := t.client.Dial("tcp", target)
			if err != nil {
				_ = local.Close()
				return
			}
			done := make(chan struct{}, 2)
			go func() { _, _ = io.Copy(remote, local); done <- struct{}{} }()
			go func() { _, _ = io.Copy(local, remote); done <- struct{}{} }()
			<-done
			_ = local.Close()
			_ = remote.Close()
		}(local)
	}
}

func (t *Tunnel) Addr() string { return t.listener.Addr().String() }

func (t *Tunnel) Fingerprint() string { return t.fingerprint }

func (t *Tunnel) Close() {
	t.once.Do(func() {
		_ = t.listener.Close()
		_ = t.client.Close()
	})
}

func expandHome(p string) string {
	if len(p) > 1 && p[:2] == "~/" {
		if home, err := os.UserHomeDir(); err == nil {
			return home + p[1:]
		}
	}
	return p
}

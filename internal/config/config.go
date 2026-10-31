package config

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Connection is a saved MySQL connection profile. Passwords live only in memory
// (and encrypted on disk); they are never serialized to the HTTP API.
type Connection struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Group     string    `json:"group"`
	Color     string    `json:"color"`
	Host      string    `json:"host"`
	Port      int       `json:"port"`
	User      string    `json:"user"`
	Password  string    `json:"-"`
	HasSecret bool      `json:"hasPassword"`
	Database  string    `json:"database"`
	Charset   string    `json:"charset"`
	SSLMode   string    `json:"sslMode"`
	Timeout   int       `json:"timeout"`
	SSH       SSHConfig `json:"ssh"`
	Remark    string    `json:"remark"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type SSHConfig struct {
	Enabled     bool   `json:"enabled"`
	Host        string `json:"host"`
	Port        int    `json:"port"`
	User        string `json:"user"`
	Auth        string `json:"auth"` // password | key | agent
	KeyPath     string `json:"keyPath"`
	Password    string `json:"-"`
	HasPassword bool   `json:"hasPassword"`
}

func (c Connection) Addr() string {
	host := strings.TrimSpace(c.Host)
	if host == "" {
		host = "127.0.0.1"
	}
	port := c.Port
	if port == 0 {
		port = 3306
	}
	return fmt.Sprintf("%s:%d", host, port)
}

func (c Connection) DisplayName() string {
	if strings.TrimSpace(c.Name) != "" {
		return c.Name
	}
	return c.User + "@" + c.Addr()
}

func (c Connection) Clone() Connection {
	out := c
	return out
}

type storedConnection struct {
	Connection
	PasswordEnc    string `json:"passwordEnc,omitempty"`
	SSHPasswordEnc string `json:"sshPasswordEnc,omitempty"`
}

// Settings holds UI preferences that survive restarts.
type Settings struct {
	Theme    string `json:"theme"`
	PageSize int    `json:"pageSize"`
	MaxRows  int    `json:"maxRows"`
	FontSize int    `json:"fontSize"`
}

func DefaultSettings() Settings {
	return Settings{Theme: "dark", PageSize: 200, MaxRows: 1000, FontSize: 13}
}

// HistoryEntry is one executed statement.
type HistoryEntry struct {
	ID       string    `json:"id"`
	SQL      string    `json:"sql"`
	ConnName string    `json:"connName"`
	Database string    `json:"database"`
	Success  bool      `json:"success"`
	Rows     int       `json:"rows"`
	Millis   int64     `json:"millis"`
	At       time.Time `json:"at"`
}

type Store struct {
	mu       sync.RWMutex
	home     string
	key      []byte
	conns    []Connection
	settings Settings
	history  []HistoryEntry
}

func DefaultHome() string {
	if v := strings.TrimSpace(os.Getenv("NAVMINI_HOME")); v != "" {
		return v
	}
	dir, err := os.UserHomeDir()
	if err != nil {
		return ".navmini"
	}
	return filepath.Join(dir, ".navmini")
}

func Open(home string) (*Store, error) {
	if home == "" {
		home = DefaultHome()
	}
	if err := os.MkdirAll(home, 0o700); err != nil {
		return nil, fmt.Errorf("create config dir: %w", err)
	}
	key, err := loadOrCreateKey(filepath.Join(home, "secret.key"))
	if err != nil {
		return nil, err
	}
	s := &Store{home: home, key: key, settings: DefaultSettings()}
	if err := s.loadConnections(); err != nil {
		return nil, err
	}
	if err := s.loadSettings(); err != nil {
		return nil, err
	}
	if err := s.loadHistory(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) Home() string { return s.home }

func (s *Store) Connections() []Connection {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Connection, len(s.conns))
	for i, c := range s.conns {
		cc := c.Clone()
		cc.Password = ""
		cc.SSH.Password = ""
		out[i] = cc
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Group != out[j].Group {
			return out[i].Group < out[j].Group
		}
		return strings.ToLower(out[i].DisplayName()) < strings.ToLower(out[j].DisplayName())
	})
	return out
}

// Raw returns the in-memory profile including secrets. For server-side use only.
func (s *Store) Raw(id string) (Connection, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, c := range s.conns {
		if c.ID == id {
			return c.Clone(), true
		}
	}
	return Connection{}, false
}

func (s *Store) Upsert(in Connection) (Connection, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	in.Name = strings.TrimSpace(in.Name)
	in.Host = strings.TrimSpace(in.Host)
	in.User = strings.TrimSpace(in.User)
	if in.Host == "" {
		in.Host = "127.0.0.1"
	}
	if in.Port == 0 {
		in.Port = 3306
	}
	if in.Charset == "" {
		in.Charset = "utf8mb4"
	}
	if in.SSLMode == "" {
		in.SSLMode = "preferred"
	}
	if in.SSH.Port == 0 {
		in.SSH.Port = 22
	}
	if in.Timeout <= 0 {
		in.Timeout = 10
	}
	if in.Name == "" {
		in.Name = in.User + "@" + in.Host
	}

	idx := -1
	for i := range s.conns {
		if s.conns[i].ID == in.ID {
			idx = i
			break
		}
	}
	if idx >= 0 {
		old := s.conns[idx]
		in.CreatedAt = old.CreatedAt
		if in.Password == "" {
			in.Password = old.Password
		}
		if in.SSH.Password == "" {
			in.SSH.Password = old.SSH.Password
		}
	} else {
		in.CreatedAt = now
		if in.ID == "" {
			in.ID = newID()
		}
	}
	in.UpdatedAt = now
	in.HasSecret = in.Password != ""
	in.SSH.HasPassword = in.SSH.Password != ""

	if idx >= 0 {
		s.conns[idx] = in
	} else {
		s.conns = append(s.conns, in)
	}
	if err := s.saveConnectionsLocked(); err != nil {
		return Connection{}, err
	}
	out := in.Clone()
	out.Password = ""
	out.SSH.Password = ""
	return out, nil
}

func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := s.conns[:0]
	for _, c := range s.conns {
		if c.ID != id {
			out = append(out, c)
		}
	}
	s.conns = out
	return s.saveConnectionsLocked()
}

func (s *Store) GetSettings() Settings {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.settings
}

func (s *Store) SaveSettings(v Settings) error {
	if v.PageSize <= 0 || v.PageSize > 5000 {
		v.PageSize = 200
	}
	if v.MaxRows <= 0 || v.MaxRows > 100000 {
		v.MaxRows = 1000
	}
	if v.FontSize < 11 || v.FontSize > 24 {
		v.FontSize = 13
	}
	if v.Theme != "light" {
		v.Theme = "dark"
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.settings = v
	return writeJSONAtomic(filepath.Join(s.home, "settings.json"), v, 0o600)
}

func (s *Store) History() []HistoryEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]HistoryEntry, len(s.history))
	copy(out, s.history)
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out
}

func (s *Store) AddHistory(e HistoryEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e.ID = newID()
	e.At = time.Now()
	s.history = append(s.history, e)
	if len(s.history) > 300 {
		s.history = s.history[len(s.history)-300:]
	}
	_ = writeJSONAtomic(filepath.Join(s.home, "history.json"), s.history, 0o600)
}

func (s *Store) ClearHistory() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.history = nil
	return writeJSONAtomic(filepath.Join(s.home, "history.json"), []HistoryEntry{}, 0o600)
}

func (s *Store) loadConnections() error {
	path := filepath.Join(s.home, "connections.json")
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read connections: %w", err)
	}
	var stored []storedConnection
	if err := json.Unmarshal(raw, &stored); err != nil {
		return fmt.Errorf("parse connections: %w", err)
	}
	for _, sc := range stored {
		c := sc.Connection
		if sc.PasswordEnc != "" {
			if pw, err := openSealed(s.key, sc.PasswordEnc); err == nil {
				c.Password = pw
			}
		}
		if sc.SSHPasswordEnc != "" {
			if pw, err := openSealed(s.key, sc.SSHPasswordEnc); err == nil {
				c.SSH.Password = pw
			}
		}
		c.HasSecret = c.Password != ""
		c.SSH.HasPassword = c.SSH.Password != ""
		s.conns = append(s.conns, c)
	}
	return nil
}

func (s *Store) saveConnectionsLocked() error {
	stored := make([]storedConnection, 0, len(s.conns))
	for _, c := range s.conns {
		sc := storedConnection{Connection: c}
		sc.Connection.Password = ""
		sc.Connection.SSH.Password = ""
		if c.Password != "" {
			enc, err := seal(s.key, c.Password)
			if err != nil {
				return err
			}
			sc.PasswordEnc = enc
		}
		if c.SSH.Password != "" {
			enc, err := seal(s.key, c.SSH.Password)
			if err != nil {
				return err
			}
			sc.SSHPasswordEnc = enc
		}
		stored = append(stored, sc)
	}
	return writeJSONAtomic(filepath.Join(s.home, "connections.json"), stored, 0o600)
}

func (s *Store) loadSettings() error {
	raw, err := os.ReadFile(filepath.Join(s.home, "settings.json"))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	var v Settings
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil // tolerate corrupt settings
	}
	if v.PageSize > 0 {
		s.settings.PageSize = v.PageSize
	}
	if v.MaxRows > 0 {
		s.settings.MaxRows = v.MaxRows
	}
	if v.FontSize >= 11 {
		s.settings.FontSize = v.FontSize
	}
	if v.Theme == "light" {
		s.settings.Theme = "light"
	}
	return nil
}

func (s *Store) loadHistory() error {
	raw, err := os.ReadFile(filepath.Join(s.home, "history.json"))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	_ = json.Unmarshal(raw, &s.history)
	return nil
}

func writeJSONAtomic(path string, v any, mode os.FileMode) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, mode); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func loadOrCreateKey(path string) ([]byte, error) {
	raw, err := os.ReadFile(path)
	if err == nil && len(raw) >= 32 {
		return raw[:32], nil
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("read key: %w", err)
	}
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, key, 0o600); err != nil {
		return nil, fmt.Errorf("write key: %w", err)
	}
	return key, nil
}

func seal(key []byte, plain string) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	out := gcm.Seal(nonce, nonce, []byte(plain), nil)
	return base64.StdEncoding.EncodeToString(out), nil
}

func openSealed(key []byte, enc string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(enc)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(raw) < gcm.NonceSize() {
		return "", errors.New("ciphertext too short")
	}
	nonce, data := raw[:gcm.NonceSize()], raw[gcm.NonceSize():]
	plain, err := gcm.Open(nil, nonce, data, nil)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

func newID() string {
	b := make([]byte, 9)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

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
	Theme       string `json:"theme"`
	Language    string `json:"language"` // en | zh
	PageSize    int    `json:"pageSize"`
	MaxRows     int    `json:"maxRows"`
	FontSize    int    `json:"fontSize"`    // editor font size
	FontFamily  string `json:"fontFamily"`  // system | mono | serif
	Background  string `json:"background"`  // stored image filename, empty = none
	DefaultSort bool   `json:"defaultSort"` // order by primary key DESC when no explicit sort
}

func DefaultSettings() Settings {
	return Settings{Theme: "dark", Language: "en", PageSize: 200, MaxRows: 1000, FontSize: 13, FontFamily: "system", DefaultSort: true}
}

// SavedQuery is a user-named SQL snippet scoped to a connection endpoint.
type SavedQuery struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	SQL       string    `json:"sql"`
	Host      string    `json:"host"` // host:port
	Database  string    `json:"database"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// AIConfig is the public (secret-free) DeepSeek configuration.
type AIConfig struct {
	Enabled     bool     `json:"enabled"`
	BaseURL     string   `json:"baseURL"`
	Model       string   `json:"model"`
	Permission  string   `json:"permission"` // readonly | modify | delete
	Databases   []string `json:"databases"`
	AutoExplain bool     `json:"autoExplain"`
	HasKey      bool     `json:"hasKey"`
}

func DefaultAIConfig() AIConfig {
	return AIConfig{
		BaseURL:     "https://api.deepseek.com",
		Model:       "deepseek-chat",
		Permission:  "readonly",
		AutoExplain: true,
	}
}

// HistoryEntry is one executed statement.
type HistoryEntry struct {
	ID       string    `json:"id"`
	SQL      string    `json:"sql"`
	Source   string    `json:"source,omitempty"`
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
	saved    []SavedQuery
	ai       AIConfig
	aiKey    string
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
	if err := s.loadAI(); err != nil {
		return nil, err
	}
	if err := s.loadSavedQueries(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) Home() string { return s.home }

// ConnectionsForExport returns clones including decrypted passwords, for the
// explicit export flow only.
func (s *Store) ConnectionsForExport(ids []string) []Connection {
	s.mu.RLock()
	defer s.mu.RUnlock()
	want := map[string]bool{}
	for _, id := range ids {
		want[id] = true
	}
	out := make([]Connection, 0, len(s.conns))
	for _, c := range s.conns {
		if len(want) > 0 && !want[c.ID] {
			continue
		}
		cc := c.Clone()
		out = append(out, cc)
	}
	return out
}

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

func (s *Store) DeleteMany(ids []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	drop := make(map[string]bool, len(ids))
	for _, id := range ids {
		drop[id] = true
	}
	out := s.conns[:0]
	for _, c := range s.conns {
		if !drop[c.ID] {
			out = append(out, c)
		}
	}
	s.conns = out
	return s.saveConnectionsLocked()
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
	if v.Language != "zh" {
		v.Language = "en"
	}
	switch v.FontFamily {
	case "mono", "serif":
	default:
		v.FontFamily = "system"
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
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(raw, &probe); err == nil {
		if _, ok := probe["defaultSort"]; ok {
			s.settings.DefaultSort = v.DefaultSort
		}
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
	if v.Theme == "light" || v.Theme == "dark" {
		s.settings.Theme = v.Theme
	}
	if v.Language == "zh" || v.Language == "en" {
		s.settings.Language = v.Language
	}
	if v.FontFamily == "mono" || v.FontFamily == "serif" {
		s.settings.FontFamily = v.FontFamily
	} else {
		s.settings.FontFamily = "system"
	}
	if v.FontSize >= 11 {
		s.settings.FontSize = v.FontSize
	}
	s.settings.Background = v.Background
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

type storedAI struct {
	AIConfig
	APIKeyEnc string `json:"apiKeyEnc,omitempty"`
}

func (s *Store) GetAI() AIConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := s.ai
	out.Databases = append([]string{}, s.ai.Databases...)
	out.HasKey = s.aiKey != ""
	return out
}

func (s *Store) AIKey() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.aiKey
}

func (s *Store) SaveAI(in AIConfig, apiKey string) error {
	base := strings.TrimSpace(in.BaseURL)
	if base == "" {
		base = "https://api.deepseek.com"
	}
	model := strings.TrimSpace(in.Model)
	if model == "" {
		model = "deepseek-chat"
	}
	perm := strings.ToLower(strings.TrimSpace(in.Permission))
	switch perm {
	case "readonly", "modify", "delete":
	default:
		perm = "readonly"
	}
	dbs := make([]string, 0, len(in.Databases))
	seen := map[string]bool{}
	for _, d := range in.Databases {
		d = strings.TrimSpace(d)
		if d != "" && !seen[d] {
			seen[d] = true
			dbs = append(dbs, d)
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.ai = AIConfig{
		Enabled: in.Enabled, BaseURL: base, Model: model, Permission: perm,
		Databases: dbs, AutoExplain: in.AutoExplain,
	}
	if strings.TrimSpace(apiKey) != "" {
		enc, err := seal(s.key, strings.TrimSpace(apiKey))
		if err != nil {
			return err
		}
		s.aiKey = strings.TrimSpace(apiKey)
		s.ai.HasKey = true
		if err := writeJSONAtomic(filepath.Join(s.home, "ai.json"), storedAI{AIConfig: s.ai, APIKeyEnc: enc}, 0o600); err != nil {
			return err
		}
		return nil
	}
	sc := storedAI{AIConfig: s.ai}
	sc.HasKey = false
	if s.aiKey != "" {
		enc, err := seal(s.key, s.aiKey)
		if err != nil {
			return err
		}
		sc.APIKeyEnc = enc
	}
	return writeJSONAtomic(filepath.Join(s.home, "ai.json"), sc, 0o600)
}

func (s *Store) loadAI() error {
	s.ai = DefaultAIConfig()
	path := filepath.Join(s.home, "ai.json")
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	var sc storedAI
	if err := json.Unmarshal(raw, &sc); err != nil {
		return nil
	}
	s.ai = sc.AIConfig
	if s.ai.BaseURL == "" {
		s.ai.BaseURL = "https://api.deepseek.com"
	}
	if s.ai.Model == "" {
		s.ai.Model = "deepseek-chat"
	}
	if s.ai.Permission == "" {
		s.ai.Permission = "readonly"
	}
	if sc.APIKeyEnc != "" {
		if key, err := openSealed(s.key, sc.APIKeyEnc); err == nil {
			s.aiKey = key
		}
	}
	s.ai.HasKey = s.aiKey != ""
	return nil
}

func (s *Store) SavedQueries(host string) []SavedQuery {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]SavedQuery, 0, len(s.saved))
	for _, q := range s.saved {
		if host == "" || q.Host == host {
			out = append(out, q)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].UpdatedAt.After(out[j].UpdatedAt) })
	return out
}

func (s *Store) SaveQuery(q SavedQuery) (SavedQuery, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	q.Name = strings.TrimSpace(q.Name)
	q.SQL = strings.TrimSpace(q.SQL)
	q.Host = strings.TrimSpace(q.Host)
	if q.Name == "" {
		q.Name = "Query"
	}
	if q.SQL == "" {
		return SavedQuery{}, errors.New("SQL is empty")
	}
	idx := -1
	for i := range s.saved {
		if q.ID != "" && s.saved[i].ID == q.ID {
			idx = i
			break
		}
	}
	if idx >= 0 {
		q.CreatedAt = s.saved[idx].CreatedAt
		q.UpdatedAt = now
		s.saved[idx] = q
	} else {
		q.ID = newID()
		q.CreatedAt = now
		q.UpdatedAt = now
		s.saved = append(s.saved, q)
	}
	if len(s.saved) > 500 {
		s.saved = s.saved[len(s.saved)-500:]
	}
	return q, writeJSONAtomic(filepath.Join(s.home, "saved_queries.json"), s.saved, 0o600)
}

func (s *Store) DeleteSavedQuery(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := s.saved[:0]
	for _, q := range s.saved {
		if q.ID != id {
			out = append(out, q)
		}
	}
	s.saved = out
	return writeJSONAtomic(filepath.Join(s.home, "saved_queries.json"), s.saved, 0o600)
}

func (s *Store) loadSavedQueries() error {
	raw, err := os.ReadFile(filepath.Join(s.home, "saved_queries.json"))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, &s.saved)
}

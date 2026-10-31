package mysqlx

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/go-sql-driver/mysql"

	"navmini/internal/config"
)

type ServerInfo struct {
	Version        string `json:"version"`
	Edition        string `json:"edition"`
	Hostname       string `json:"hostname"`
	Port           int    `json:"port"`
	Charset        string `json:"charset"`
	ConnectionID   int64  `json:"connectionId"`
	CurrentUser    string `json:"currentUser"`
	SSHFingerprint string `json:"sshFingerprint,omitempty"`
}

type Session struct {
	ID        string
	Conn      config.Connection
	CreatedAt time.Time

	db     *sql.DB
	admin  *sql.DB
	conn   *sql.Conn
	tunnel *Tunnel
	info   ServerInfo

	mu        sync.Mutex
	runMu     sync.Mutex
	closed    bool
	lastUsed  time.Time
	currentDB string
	cancels   map[string]context.CancelFunc
}

type SessionView struct {
	ID           string     `json:"id"`
	ConnectionID string     `json:"connectionId"`
	Name         string     `json:"name"`
	Database     string     `json:"database"`
	Server       ServerInfo `json:"server"`
	OpenedAt     time.Time  `json:"openedAt"`
	Tunnel       string     `json:"tunnel,omitempty"`
}

type Manager struct {
	mu       sync.RWMutex
	sessions map[string]*Session
}

func NewManager() *Manager {
	return &Manager{sessions: make(map[string]*Session)}
}

func (m *Manager) Open(conn config.Connection, database string) (*Session, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()

	addr := conn.Addr()
	var tunnel *Tunnel
	if conn.SSH.Enabled {
		t, err := StartTunnel(conn.SSH, addr)
		if err != nil {
			return nil, err
		}
		tunnel = t
		addr = t.Addr()
	}

	mc := mysql.NewConfig()
	mc.User = conn.User
	mc.Passwd = conn.Password
	mc.Net = "tcp"
	mc.Addr = addr
	mc.DBName = ""
	mc.Timeout = time.Duration(nonZero(conn.Timeout, 10)) * time.Second
	mc.TLSConfig = sslMode(conn.SSLMode)
	connector, err := mysql.NewConnector(mc)
	if err != nil {
		if tunnel != nil {
			tunnel.Close()
		}
		return nil, fmt.Errorf("build MySQL connector: %w", err)
	}
	db := sql.OpenDB(connector)
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)
	admin := sql.OpenDB(connector)
	admin.SetMaxOpenConns(2)
	admin.SetMaxIdleConns(1)
	admin.SetConnMaxLifetime(0)

	rawConn, err := db.Conn(ctx)
	if err != nil {
		_ = db.Close()
		if tunnel != nil {
			tunnel.Close()
		}
		return nil, fmt.Errorf("connect to MySQL: %w", err)
	}
	if err := rawConn.PingContext(ctx); err != nil {
		_ = rawConn.Close()
		_ = db.Close()
		if tunnel != nil {
			tunnel.Close()
		}
		return nil, fmt.Errorf("ping MySQL: %w", err)
	}
	if conn.Charset != "" {
		cs, ok := safeCharset(conn.Charset)
		if !ok {
			_ = rawConn.Close()
			_ = db.Close()
			if tunnel != nil {
				tunnel.Close()
			}
			return nil, fmt.Errorf("invalid charset %q", conn.Charset)
		}
		if _, err := rawConn.ExecContext(ctx, "SET NAMES "+cs); err != nil {
			_ = rawConn.Close()
			_ = db.Close()
			if tunnel != nil {
				tunnel.Close()
			}
			return nil, fmt.Errorf("set charset: %w", err)
		}
	}

	s := &Session{
		ID:        newSessionID(),
		Conn:      conn,
		CreatedAt: time.Now(),
		db:        db,
		admin:     admin,
		conn:      rawConn,
		tunnel:    tunnel,
		currentDB: "",
		lastUsed:  time.Now(),
		cancels:   map[string]context.CancelFunc{},
	}
	if tunnel != nil {
		s.info.SSHFingerprint = tunnel.Fingerprint()
	}
	if database != "" {
		if err := s.use(ctx, database); err != nil {
			s.closeResources()
			return nil, err
		}
	}
	if err := s.loadInfo(ctx); err != nil {
		// Server info is helpful but not required to use the session.
		s.info.Version = "unknown"
	}

	m.mu.Lock()
	m.sessions[s.ID] = s
	m.mu.Unlock()
	return s, nil
}

func (m *Manager) Get(id string) (*Session, bool) {
	m.mu.RLock()
	s, ok := m.sessions[id]
	m.mu.RUnlock()
	if ok {
		s.touch()
	}
	return s, ok
}

// Reap closes sessions that have been idle longer than ttl.
func (m *Manager) Reap(ttl, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for range ticker.C {
		m.reapOnce(ttl)
	}
}

func (m *Manager) reapOnce(ttl time.Duration) {
	var victims []*Session
	m.mu.Lock()
	for id, s := range m.sessions {
		s.mu.Lock()
		idle := time.Since(s.lastUsed)
		s.mu.Unlock()
		if idle > ttl {
			delete(m.sessions, id)
			victims = append(victims, s)
		}
	}
	m.mu.Unlock()
	for _, s := range victims {
		s.Close()
	}
}

func (s *Session) touch() {
	s.mu.Lock()
	s.lastUsed = time.Now()
	s.mu.Unlock()
}

func (m *Manager) Close(id string) {
	m.mu.Lock()
	s, ok := m.sessions[id]
	delete(m.sessions, id)
	m.mu.Unlock()
	if ok {
		s.Close()
	}
}

func (m *Manager) CloseAll() {
	m.mu.Lock()
	list := m.sessions
	m.sessions = make(map[string]*Session)
	m.mu.Unlock()
	for _, s := range list {
		s.Close()
	}
}

func (s *Session) View() SessionView {
	s.mu.Lock()
	defer s.mu.Unlock()
	v := SessionView{
		ID:           s.ID,
		ConnectionID: s.Conn.ID,
		Name:         s.Conn.DisplayName(),
		Database:     s.currentDB,
		Server:       s.info,
		OpenedAt:     s.CreatedAt,
	}
	if s.tunnel != nil {
		v.Tunnel = s.tunnel.Addr()
	}
	return v
}

func (s *Session) ServerInfo() ServerInfo {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.info
}

func (s *Session) Close() {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.closed = true
	for _, cancel := range s.cancels {
		cancel()
	}
	s.cancels = map[string]context.CancelFunc{}
	s.mu.Unlock()
	s.closeResources()
}

func (s *Session) closeResources() {
	if s.conn != nil {
		_ = s.conn.Close()
	}
	if s.db != nil {
		_ = s.db.Close()
	}
	if s.admin != nil {
		_ = s.admin.Close()
	}
	if s.tunnel != nil {
		s.tunnel.Close()
	}
}

func (s *Session) use(ctx context.Context, database string) error {
	if database == "" {
		return nil
	}
	if _, err := s.conn.ExecContext(ctx, "USE "+QuoteIdent(database)); err != nil {
		return fmt.Errorf("select database %s: %w", database, err)
	}
	s.mu.Lock()
	s.currentDB = database
	s.mu.Unlock()
	return nil
}

func (s *Session) loadInfo(ctx context.Context) error {
	var (
		version  sql.NullString
		edition  sql.NullString
		hostname sql.NullString
		port     sql.NullInt64
		charset  sql.NullString
		connID   sql.NullInt64
		user     sql.NullString
	)
	err := s.conn.QueryRowContext(ctx,
		"SELECT VERSION(), @@version_comment, @@hostname, @@port, @@character_set_server, CONNECTION_ID(), CURRENT_USER()",
	).Scan(&version, &edition, &hostname, &port, &charset, &connID, &user)
	if err != nil {
		return err
	}
	s.info = ServerInfo{
		Version:        version.String,
		Edition:        edition.String,
		Hostname:       hostname.String,
		Port:           int(port.Int64),
		Charset:        charset.String,
		ConnectionID:   connID.Int64,
		CurrentUser:    user.String,
		SSHFingerprint: s.info.SSHFingerprint,
	}
	return nil
}

// Run executes one or more statements sequentially on the session's single
// pinned connection. It holds the session lock for the whole batch so that a
// transaction (BEGIN ... COMMIT) sent as one batch behaves as expected.
func (s *Session) Run(ctx context.Context, queryID, database, text string, maxRows int) ([]ResultSet, error) {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil, errors.New("session is closed")
	}
	runCtx, cancel := context.WithCancel(ctx)
	if queryID != "" {
		s.cancels[queryID] = cancel
	}
	s.mu.Unlock()

	defer func() {
		cancel()
		s.mu.Lock()
		delete(s.cancels, queryID)
		s.mu.Unlock()
	}()

	// The pinned connection is not safe for concurrent use; serialize batches.
	s.runMu.Lock()
	defer s.runMu.Unlock()

	s.mu.Lock()
	curDB := s.currentDB
	s.mu.Unlock()
	if database != "" && database != curDB {
		if err := s.use(runCtx, database); err != nil {
			return nil, err
		}
	}

	stmts := SplitStatements(text)
	if len(stmts) == 0 {
		return nil, errors.New("no SQL statement to execute")
	}
	if maxRows <= 0 {
		maxRows = 1000
	}
	if maxRows > 10000 {
		maxRows = 10000
	}

	results := make([]ResultSet, 0, len(stmts))
	for idx, stmt := range stmts {
		start := time.Now()
		rs, err := s.runOne(runCtx, stmt, maxRows)
		rs.Index = idx + 1
		rs.Statement = stmt
		rs.DurationMS = time.Since(start).Milliseconds()
		if err != nil {
			return results, fmt.Errorf("statement %d: %w", idx+1, err)
		}
		results = append(results, rs)
	}
	return results, nil
}

// Cancel stops a running statement on the server with KILL QUERY, so the
// session's connection (and any transaction state) survives the cancel.
func (s *Session) Cancel(queryID string) bool {
	s.mu.Lock()
	cancel, ok := s.cancels[queryID]
	connID := s.info.ConnectionID
	admin := s.admin
	s.mu.Unlock()
	if !ok {
		return false
	}
	if admin != nil && connID > 0 {
		ctx, done := context.WithTimeout(context.Background(), 5*time.Second)
		defer done()
		if _, err := admin.ExecContext(ctx, fmt.Sprintf("KILL QUERY %d", connID)); err == nil {
			return true
		}
	}
	cancel()
	return true
}

func (s *Session) runOne(ctx context.Context, stmt string, maxRows int) (ResultSet, error) {
	if IsReadStatement(stmt) {
		return s.queryOne(ctx, stmt, maxRows)
	}
	res, err := s.conn.ExecContext(ctx, stmt)
	if err != nil {
		return ResultSet{}, err
	}
	affected, _ := res.RowsAffected()
	lastID, _ := res.LastInsertId()
	msg := fmt.Sprintf("%d row(s) affected", affected)
	if lastID > 0 {
		msg += fmt.Sprintf(", last insert id %d", lastID)
	}
	return ResultSet{
		Affected:     affected,
		LastInsertID: lastID,
		Message:      msg,
	}, nil
}

func (s *Session) queryOne(ctx context.Context, stmt string, maxRows int) (ResultSet, error) {
	rows, err := s.conn.QueryContext(ctx, stmt)
	if err != nil {
		return ResultSet{}, err
	}
	defer rows.Close()

	rs := ResultSet{}
	for {
		columns, data, truncated, err := scanRows(rows, maxRows)
		if err != nil {
			return rs, err
		}
		rs.Columns = columns
		rs.Rows = data
		rs.RowCount = len(data)
		rs.Truncated = truncated
		if !rows.NextResultSet() {
			break
		}
	}
	return rs, nil
}

func nonZero(v, fallback int) int {
	if v > 0 {
		return v
	}
	return fallback
}

func sslMode(v string) string {
	switch v {
	case "disabled", "false":
		return "false"
	case "required", "true":
		return "true"
	case "skip-verify":
		return "skip-verify"
	default:
		return "preferred"
	}
}

var sessionSeq struct {
	sync.Mutex
	n int64
}

func newSessionID() string {
	sessionSeq.Lock()
	sessionSeq.n++
	n := sessionSeq.n
	sessionSeq.Unlock()
	return fmt.Sprintf("s%d-%d", time.Now().UnixNano()%1e9, n)
}

func QuoteIdent(v string) string {
	return "`" + strings.ReplaceAll(v, "`", "``") + "`"
}

func safeCharset(v string) (string, bool) {
	if v == "" {
		return "", false
	}
	for _, r := range v {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_':
		default:
			return "", false
		}
	}
	return v, true
}

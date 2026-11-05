package server

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"strings"
	"time"

	"navmini/internal/config"
	"navmini/internal/mysqlx"
	"navmini/web"
)

type Server struct {
	store   *config.Store
	mgr     *mysqlx.Manager
	token   string
	version string
	started time.Time
	imports *importStore
}

func New(store *config.Store, mgr *mysqlx.Manager, token, version string) *Server {
	return &Server{store: store, mgr: mgr, token: token, version: version, started: time.Now(), imports: newImportStore()}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/", s.handleIndex)
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(web.FS))))

	routes := map[string]http.HandlerFunc{
		"/api/ping":                      s.handlePing,
		"/api/state":                     s.handleState,
		"/api/settings":                  s.handleSettings,
		"/api/connections/save":          s.handleSaveConnection,
		"/api/connections/delete":        s.handleDeleteConnection,
		"/api/connections/delete-bulk":   s.handleDeleteConnections,
		"/api/connections/test":          s.handleTestConnection,
		"/api/connections/import-ncx":    s.handleImportNCX,
		"/api/connections/export-ncx":    s.handleExportNCX,
		"/api/session/open":              s.handleOpenSession,
		"/api/session/close":             s.handleCloseSession,
		"/api/databases":                 s.handleDatabases,
		"/api/objects":                   s.handleObjects,
		"/api/table":                     s.handleTable,
		"/api/query":                     s.handleQuery,
		"/api/query/cancel":              s.handleCancel,
		"/api/rows":                      s.handleRows,
		"/api/rows/update":               s.handleUpdateRow,
		"/api/rows/insert":               s.handleInsertRow,
		"/api/rows/delete":               s.handleDeleteRow,
		"/api/rows/sql":                  s.handleRowSQL,
		"/api/cell":                      s.handleCell,
		"/api/table/backup":              s.handleBackupTable,
		"/api/ai/state":                  s.handleAIState,
		"/api/ai/config":                 s.handleAIConfig,
		"/api/ai/test":                   s.handleAITest,
		"/api/ai/models":                 s.handleAIModels,
		"/api/ai/detect":                 s.handleAIDetect,
		"/api/ai/attachment":             s.handleAIAttachment,
		"/api/ai/chat":                   s.handleAIChat,
		"/api/ai/context":                s.handleAIContext,
		"/api/ai/execute":                s.handleAIExecute,
		"/api/processlist":               s.handleProcesslist,
		"/api/processlist/kill":          s.handleKill,
		"/api/status":                    s.handleStatus,
		"/api/history":                   s.handleHistory,
		"/api/export":                    s.handleExport,
		"/api/export/xlsx":               s.handleExportXLSX,
		"/api/database/export":           s.handleDatabaseExport,
		"/api/import/upload":             s.handleImportUpload,
		"/api/import/preview":            s.handleImportPreview,
		"/api/import/execute":            s.handleImportExecute,
		"/api/saved-queries":             s.handleSavedQueries,
		"/api/saved-queries/save":        s.handleSaveQuery,
		"/api/saved-queries/delete":      s.handleDeleteSavedQuery,
		"/api/settings/background":       s.handleUploadBackground,
		"/api/settings/background/clear": s.handleClearBackground,
		"/api/background":                s.handleBackground,
	}
	for path, h := range routes {
		mux.HandleFunc(path, s.auth(h))
	}
	return s.recoverer(s.logRequests(mux))
}

func (s *Server) logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/") {
			next.ServeHTTP(w, r)
			return
		}
		started := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %dms", r.Method, r.URL.Path, time.Since(started).Milliseconds())
	})
}

func (s *Server) recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if v := recover(); v != nil {
				log.Printf("panic serving %s %s: %v", r.Method, r.URL.Path, v)
				fail(w, http.StatusInternalServerError, errors.New("internal error"))
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func (s *Server) auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("X-NavMini-Token")
		if token == "" {
			token = r.URL.Query().Get("token")
		}
		if token == "" {
			if c, err := r.Cookie("navmini_token"); err == nil {
				token = c.Value
			}
		}
		if subtle.ConstantTimeCompare([]byte(token), []byte(s.token)) != 1 {
			fail(w, http.StatusUnauthorized, errors.New("invalid session token"))
			return
		}
		next(w, r)
	}
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	queryToken := r.URL.Query().Get("token")
	cookieToken := ""
	if c, err := r.Cookie("navmini_token"); err == nil {
		cookieToken = c.Value
	}
	validQuery := subtle.ConstantTimeCompare([]byte(queryToken), []byte(s.token)) == 1
	validCookie := subtle.ConstantTimeCompare([]byte(cookieToken), []byte(s.token)) == 1
	if queryToken != "" && !validQuery {
		http.Error(w, "invalid token", http.StatusForbidden)
		return
	}
	if queryToken == "" && !validCookie {
		http.Error(w, "missing token: open NavMini from the launcher", http.StatusForbidden)
		return
	}
	if validQuery {
		http.SetCookie(w, &http.Cookie{
			Name:     "navmini_token",
			Value:    s.token,
			Path:     "/",
			HttpOnly: true,
			SameSite: http.SameSiteStrictMode,
		})
	}

	raw, err := fs.ReadFile(web.FS, "index.html")
	if err != nil {
		http.Error(w, "ui not embedded", http.StatusInternalServerError)
		return
	}
	page := strings.ReplaceAll(string(raw), "{{TOKEN}}", s.token)
	page = strings.ReplaceAll(page, "{{VERSION}}", s.version)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = io.WriteString(w, page)
}

func (s *Server) sessionFromQuery(w http.ResponseWriter, r *http.Request) (*mysqlx.Session, bool) {
	id := r.URL.Query().Get("session")
	if id == "" {
		fail(w, http.StatusBadRequest, errors.New("session is required"))
		return nil, false
	}
	sess, found := s.mgr.Get(id)
	if !found {
		fail(w, http.StatusBadRequest, errors.New("session not found or closed"))
		return nil, false
	}
	return sess, true
}

func sessionByID(s *Server, w http.ResponseWriter, id string) (*mysqlx.Session, bool) {
	if id == "" {
		fail(w, http.StatusBadRequest, errors.New("session is required"))
		return nil, false
	}
	sess, found := s.mgr.Get(id)
	if !found {
		fail(w, http.StatusBadRequest, errors.New("session not found or closed"))
		return nil, false
	}
	return sess, true
}

func decodeJSON(r *http.Request, v any) error {
	defer r.Body.Close()
	dec := json.NewDecoder(io.LimitReader(r.Body, 4<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return fmt.Errorf("invalid request body: %w", err)
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func fail(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]any{"error": err.Error()})
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

func requirePost(w http.ResponseWriter, r *http.Request) bool {
	if r.Method != http.MethodPost {
		fail(w, http.StatusMethodNotAllowed, errors.New("POST required"))
		return false
	}
	return true
}

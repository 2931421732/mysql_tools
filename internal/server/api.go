package server

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"navmini/internal/config"
	"navmini/internal/mysqlx"
)

func (s *Server) handlePing(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"version": s.version,
		"uptime":  time.Since(s.started).Seconds(),
	})
}

func (s *Server) handleState(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"app": map[string]any{
			"name":    "NavMini",
			"version": s.version,
			"home":    s.store.Home(),
		},
		"connections": s.store.Connections(),
		"settings":    s.store.GetSettings(),
	})
}

func (s *Server) handleSettings(w http.ResponseWriter, r *http.Request) {
	if !requirePost(w, r) {
		return
	}
	var v config.Settings
	if err := decodeJSON(r, &v); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	if err := s.store.SaveSettings(v); err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, s.store.GetSettings())
}

type connectionPayload struct {
	config.Connection
	Password    string `json:"password"`
	SSHPassword string `json:"sshPassword"`
}

func (p connectionPayload) toConnection() config.Connection {
	c := p.Connection
	if p.Password != "" {
		c.Password = p.Password
	}
	if p.SSHPassword != "" {
		c.SSH.Password = p.SSHPassword
	}
	return c
}

func (s *Server) handleSaveConnection(w http.ResponseWriter, r *http.Request) {
	if !requirePost(w, r) {
		return
	}
	var payload connectionPayload
	if err := decodeJSON(r, &payload); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	out, err := s.store.Upsert(payload.toConnection())
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleDeleteConnection(w http.ResponseWriter, r *http.Request) {
	if !requirePost(w, r) {
		return
	}
	var req struct {
		ID string `json:"id"`
	}
	if err := decodeJSON(r, &req); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	if err := s.store.Delete(req.ID); err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleTestConnection(w http.ResponseWriter, r *http.Request) {
	if !requirePost(w, r) {
		return
	}
	var payload connectionPayload
	if err := decodeJSON(r, &payload); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	in := payload.toConnection()
	if in.Password == "" && in.ID != "" {
		if saved, ok := s.store.Raw(in.ID); ok {
			in.Password = saved.Password
			if in.SSH.Password == "" {
				in.SSH.Password = saved.SSH.Password
			}
		}
	}
	started := time.Now()
	sess, err := s.mgr.Open(in, "")
	if err != nil {
		fail(w, http.StatusBadGateway, err)
		return
	}
	info := sess.ServerInfo()
	sess.Close()
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"server":  info,
		"latency": time.Since(started).Milliseconds(),
	})
}

func (s *Server) handleOpenSession(w http.ResponseWriter, r *http.Request) {
	if !requirePost(w, r) {
		return
	}
	var req struct {
		ConnectionID string `json:"connectionId"`
		Database     string `json:"database"`
	}
	if err := decodeJSON(r, &req); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	conn, ok := s.store.Raw(req.ConnectionID)
	if !ok {
		fail(w, http.StatusNotFound, errors.New("connection not found"))
		return
	}
	sess, err := s.mgr.Open(conn, req.Database)
	if err != nil {
		fail(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, sess.View())
}

func (s *Server) handleCloseSession(w http.ResponseWriter, r *http.Request) {
	if !requirePost(w, r) {
		return
	}
	var req struct {
		Session string `json:"session"`
	}
	if err := decodeJSON(r, &req); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	s.mgr.Close(req.Session)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleDatabases(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.sessionFromQuery(w, r)
	if !ok {
		return
	}
	dbs, err := sess.Databases(r.Context())
	if err != nil {
		fail(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, dbs)
}

func (s *Server) handleObjects(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.sessionFromQuery(w, r)
	if !ok {
		return
	}
	db := r.URL.Query().Get("db")
	if db == "" {
		fail(w, http.StatusBadRequest, errors.New("db is required"))
		return
	}
	objs, err := sess.Objects(r.Context(), db)
	if err != nil {
		fail(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, objs)
}

func (s *Server) handleTable(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.sessionFromQuery(w, r)
	if !ok {
		return
	}
	db, table := r.URL.Query().Get("db"), r.URL.Query().Get("table")
	if db == "" || table == "" {
		fail(w, http.StatusBadRequest, errors.New("db and table are required"))
		return
	}
	detail, err := sess.TableDetail(r.Context(), db, table)
	if err != nil {
		fail(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

func (s *Server) handleQuery(w http.ResponseWriter, r *http.Request) {
	if !requirePost(w, r) {
		return
	}
	var req struct {
		Session  string `json:"session"`
		SQL      string `json:"sql"`
		Database string `json:"database"`
		MaxRows  int    `json:"maxRows"`
		QueryID  string `json:"queryId"`
	}
	if err := decodeJSON(r, &req); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	sess, ok := sessionByID(s, w, req.Session)
	if !ok {
		return
	}
	if req.MaxRows <= 0 {
		req.MaxRows = s.store.GetSettings().MaxRows
	}
	started := time.Now()
	results, runErr := sess.Run(r.Context(), req.QueryID, req.Database, req.SQL, req.MaxRows)
	elapsed := time.Since(started).Milliseconds()
	if results == nil {
		results = []mysqlx.ResultSet{}
	}

	rows := 0
	for _, rs := range results {
		rows += rs.RowCount
	}
	if strings.TrimSpace(req.SQL) != "" {
		s.store.AddHistory(config.HistoryEntry{
			SQL:      truncate(req.SQL, 4000),
			ConnName: sess.Conn.DisplayName(),
			Database: req.Database,
			Success:  runErr == nil,
			Rows:     rows,
			Millis:   elapsed,
		})
	}

	payload := map[string]any{
		"results":  results,
		"duration": elapsed,
	}
	if runErr != nil {
		payload["error"] = runErr.Error()
	}
	writeJSON(w, http.StatusOK, payload)
}

func (s *Server) handleCancel(w http.ResponseWriter, r *http.Request) {
	if !requirePost(w, r) {
		return
	}
	var req struct {
		Session string `json:"session"`
		QueryID string `json:"queryId"`
	}
	if err := decodeJSON(r, &req); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	sess, ok := sessionByID(s, w, req.Session)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"cancelled": sess.Cancel(req.QueryID)})
}

type rowsRequest struct {
	Session string `json:"session"`
	mysqlx.RowsRequest
}

func (s *Server) handleRows(w http.ResponseWriter, r *http.Request) {
	if !requirePost(w, r) {
		return
	}
	var req rowsRequest
	if err := decodeJSON(r, &req); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	sess, ok := sessionByID(s, w, req.Session)
	if !ok {
		return
	}
	res, err := sess.Rows(r.Context(), req.RowsRequest)
	if err != nil {
		fail(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) handleUpdateRow(w http.ResponseWriter, r *http.Request) {
	if !requirePost(w, r) {
		return
	}
	var req struct {
		Session string `json:"session"`
		mysqlx.UpdateRowRequest
	}
	if err := decodeJSON(r, &req); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	sess, ok := sessionByID(s, w, req.Session)
	if !ok {
		return
	}
	n, err := sess.UpdateRow(r.Context(), req.UpdateRowRequest)
	if err != nil {
		fail(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"affected": n})
}

func (s *Server) handleInsertRow(w http.ResponseWriter, r *http.Request) {
	if !requirePost(w, r) {
		return
	}
	var req struct {
		Session string `json:"session"`
		mysqlx.InsertRowRequest
	}
	if err := decodeJSON(r, &req); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	sess, ok := sessionByID(s, w, req.Session)
	if !ok {
		return
	}
	id, err := sess.InsertRow(r.Context(), req.InsertRowRequest)
	if err != nil {
		fail(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"insertId": id})
}

func (s *Server) handleDeleteRow(w http.ResponseWriter, r *http.Request) {
	if !requirePost(w, r) {
		return
	}
	var req struct {
		Session string `json:"session"`
		mysqlx.DeleteRowRequest
	}
	if err := decodeJSON(r, &req); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	sess, ok := sessionByID(s, w, req.Session)
	if !ok {
		return
	}
	n, err := sess.DeleteRow(r.Context(), req.DeleteRowRequest)
	if err != nil {
		fail(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"affected": n})
}

func (s *Server) handleRowSQL(w http.ResponseWriter, r *http.Request) {
	if !requirePost(w, r) {
		return
	}
	var req struct {
		Session string `json:"session"`
		mysqlx.RowSQLRequest
	}
	if err := decodeJSON(r, &req); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	sess, ok := sessionByID(s, w, req.Session)
	if !ok {
		return
	}
	stmt, err := sess.RowSQL(r.Context(), req.RowSQLRequest)
	if err != nil {
		fail(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"sql": stmt})
}

func (s *Server) handleBackupTable(w http.ResponseWriter, r *http.Request) {
	if !requirePost(w, r) {
		return
	}
	var req struct {
		Session string `json:"session"`
		mysqlx.BackupRequest
	}
	if err := decodeJSON(r, &req); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	sess, ok := sessionByID(s, w, req.Session)
	if !ok {
		return
	}
	res, err := sess.BackupTable(r.Context(), req.BackupRequest)
	if err != nil {
		fail(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) handleProcesslist(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.sessionFromQuery(w, r)
	if !ok {
		return
	}
	list, err := sess.Processlist(r.Context())
	if err != nil {
		fail(w, http.StatusBadGateway, err)
		return
	}
	if list == nil {
		list = []mysqlx.Process{}
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) handleKill(w http.ResponseWriter, r *http.Request) {
	if !requirePost(w, r) {
		return
	}
	var req struct {
		Session string `json:"session"`
		ID      int64  `json:"id"`
	}
	if err := decodeJSON(r, &req); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	sess, ok := sessionByID(s, w, req.Session)
	if !ok {
		return
	}
	if err := sess.Kill(r.Context(), req.ID); err != nil {
		fail(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.sessionFromQuery(w, r)
	if !ok {
		return
	}
	status, err := sess.Status(r.Context())
	if err != nil {
		fail(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (s *Server) handleHistory(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, s.store.History())
	case http.MethodDelete:
		if err := s.store.ClearHistory(); err != nil {
			fail(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	default:
		fail(w, http.StatusMethodNotAllowed, errors.New("GET or DELETE required"))
	}
}

func (s *Server) handleExport(w http.ResponseWriter, r *http.Request) {
	if !requirePost(w, r) {
		return
	}
	var req rowsRequest
	if err := decodeJSON(r, &req); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	sess, ok := sessionByID(s, w, req.Session)
	if !ok {
		return
	}
	if req.Database == "" || req.Table == "" {
		fail(w, http.StatusBadRequest, errors.New("database and table are required"))
		return
	}
	exists, err := sess.TableExists(r.Context(), req.Database, req.Table)
	if err != nil {
		fail(w, http.StatusBadGateway, err)
		return
	}
	if !exists {
		fail(w, http.StatusNotFound, fmt.Errorf("table %s.%s not found", req.Database, req.Table))
		return
	}
	name := safeFileName(req.Database + "." + req.Table + ".csv")
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
	w.Header().Set("Cache-Control", "no-store")
	if _, err := sess.ExportTable(r.Context(), req.RowsRequest, w); err != nil {
		log.Printf("export %s.%s: %v", req.Database, req.Table, err)
	}
}

func safeFileName(name string) string {
	var b strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '.', r == '-', r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	return b.String()
}

func (s *Server) handleSavedQueries(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, s.store.SavedQueries(r.URL.Query().Get("host")))
}

func (s *Server) handleSaveQuery(w http.ResponseWriter, r *http.Request) {
	if !requirePost(w, r) {
		return
	}
	var q config.SavedQuery
	if err := decodeJSON(r, &q); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	if q.Host == "" {
		fail(w, http.StatusBadRequest, errors.New("host is required"))
		return
	}
	saved, err := s.store.SaveQuery(q)
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, saved)
}

func (s *Server) handleDeleteSavedQuery(w http.ResponseWriter, r *http.Request) {
	if !requirePost(w, r) {
		return
	}
	var req struct {
		ID string `json:"id"`
	}
	if err := decodeJSON(r, &req); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	if err := s.store.DeleteSavedQuery(req.ID); err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleBackground(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	name := filepath.Base(s.store.GetSettings().Background)
	if name == "" || name == "." {
		http.NotFound(w, r)
		return
	}
	p := filepath.Join(s.store.Home(), name)
	f, err := os.Open(p)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()
	w.Header().Set("Cache-Control", "no-store")
	http.ServeContent(w, r, name, time.Now(), f)
}

func (s *Server) handleUploadBackground(w http.ResponseWriter, r *http.Request) {
	if !requirePost(w, r) {
		return
	}
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	file, hdr, err := r.FormFile("file")
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	defer file.Close()
	ext := strings.ToLower(filepath.Ext(hdr.Filename))
	switch ext {
	case ".png", ".jpg", ".jpeg", ".webp":
	default:
		fail(w, http.StatusBadRequest, errors.New("only png, jpg, jpeg or webp images are supported"))
		return
	}
	data, err := io.ReadAll(io.LimitReader(file, 8<<20+1))
	if err != nil || len(data) > 8<<20 {
		fail(w, http.StatusBadRequest, errors.New("image is larger than 8 MB"))
		return
	}
	// remove old background files
	for _, old := range []string{"background.png", "background.jpg", "background.jpeg", "background.webp"} {
		_ = os.Remove(filepath.Join(s.store.Home(), old))
	}
	name := "background" + ext
	if err := os.WriteFile(filepath.Join(s.store.Home(), name), data, 0o600); err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	settings := s.store.GetSettings()
	settings.Background = name
	if err := s.store.SaveSettings(settings); err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"background": name})
}

func (s *Server) handleClearBackground(w http.ResponseWriter, r *http.Request) {
	if !requirePost(w, r) {
		return
	}
	for _, old := range []string{"background.png", "background.jpg", "background.jpeg", "background.webp"} {
		_ = os.Remove(filepath.Join(s.store.Home(), old))
	}
	settings := s.store.GetSettings()
	settings.Background = ""
	if err := s.store.SaveSettings(settings); err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

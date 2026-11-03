package server

import (
	"fmt"
	"net/http"
	"strings"
)

func (s *Server) handleExportNCX(w http.ResponseWriter, r *http.Request) {
	if !requirePost(w, r) {
		return
	}
	var req struct {
		IDs             []string `json:"ids"`
		IncludePassword bool     `json:"includePassword"`
	}
	if err := decodeJSON(r, &req); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	conns := s.store.ConnectionsForExport(req.IDs)
	if len(conns) == 0 {
		fail(w, http.StatusBadRequest, fmt.Errorf("no connections to export"))
		return
	}
	var b strings.Builder
	b.WriteString("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n")
	b.WriteString("<Connections Ver=\"1.5\">\n")
	for _, c := range conns {
		password := ""
		if req.IncludePassword && c.Password != "" {
			enc, err := encryptNCXPassword(c.Password)
			if err != nil {
				fail(w, http.StatusInternalServerError, err)
				return
			}
			password = enc
		}
		sshPassword := ""
		if req.IncludePassword && c.SSH.Password != "" {
			enc, err := encryptNCXPassword(c.SSH.Password)
			if err != nil {
				fail(w, http.StatusInternalServerError, err)
				return
			}
			sshPassword = enc
		}
		ssl := c.SSLMode != "disabled" && c.SSLMode != ""
		auth := "PASSWORD"
		if c.SSH.Auth == "key" || c.SSH.Auth == "agent" {
			auth = "PUBLICKEY"
		}
		b.WriteString("  <Connection")
		b.WriteString(ncxAttr("ConnectionName", c.DisplayName()))
		b.WriteString(ncxAttr("ConnType", "MYSQL"))
		b.WriteString(ncxAttr("OraConnType", "BASIC"))
		b.WriteString(ncxAttr("Host", c.Host))
		b.WriteString(ncxAttr("Port", fmt.Sprint(c.Port)))
		b.WriteString(ncxAttr("Database", c.Database))
		b.WriteString(ncxAttr("DatabaseFileName", ""))
		b.WriteString(ncxAttr("UserName", c.User))
		b.WriteString(ncxAttr("Password", password))
		b.WriteString(ncxAttr("SavePassword", fmt.Sprint(req.IncludePassword && c.Password != "")))
		b.WriteString(ncxAttr("SSL", fmt.Sprint(ssl)))
		b.WriteString(ncxAttr("SSH", fmt.Sprint(c.SSH.Enabled)))
		b.WriteString(ncxAttr("SSH_Host", c.SSH.Host))
		port := c.SSH.Port
		if port == 0 {
			port = 22
		}
		b.WriteString(ncxAttr("SSH_Port", fmt.Sprint(port)))
		b.WriteString(ncxAttr("SSH_UserName", c.SSH.User))
		b.WriteString(ncxAttr("SSH_AuthenMethod", auth))
		b.WriteString(ncxAttr("SSH_Password", sshPassword))
		b.WriteString(ncxAttr("SSH_SavePassword", fmt.Sprint(req.IncludePassword && c.SSH.Password != "")))
		b.WriteString(ncxAttr("SSH_PrivateKey", c.SSH.KeyPath))
		b.WriteString(ncxAttr("HTTP", "false"))
		b.WriteString("/>\n")
	}
	b.WriteString("</Connections>\n")
	name := "navmini-connections.ncx"
	w.Header().Set("Content-Type", "application/x-ncx; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write([]byte(b.String()))
}

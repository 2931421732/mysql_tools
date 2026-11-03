package server

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"navmini/internal/config"
)

type ncxNode struct {
	Name  string
	Attrs map[string]string
	Text  string
	Kids  []*ncxNode
}

func (s *Server) handleImportNCX(w http.ResponseWriter, r *http.Request) {
	if !requirePost(w, r) {
		return
	}
	if err := r.ParseMultipartForm(16 << 20); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	file, hdr, err := r.FormFile("file")
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	defer file.Close()
	if !strings.HasSuffix(strings.ToLower(hdr.Filename), ".ncx") {
		fail(w, http.StatusBadRequest, errors.New("please choose a .ncx file"))
		return
	}
	data, err := io.ReadAll(io.LimitReader(file, 10<<20+1))
	if err != nil || len(data) > 10<<20 {
		fail(w, http.StatusBadRequest, errors.New("NCX file is larger than 10 MB"))
		return
	}
	data = ncxInnerXML(data)
	conns, err := parseNCX(data)
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	imported := make([]string, 0, len(conns))
	warnings := []string{}
	for _, c := range conns {
		if c.Password != "" {
			pw, err := decryptNCXPassword(c.Password)
			if err != nil {
				warnings = append(warnings, c.Name+": 密码解密失败，已清空，请手动填写")
				c.Password = ""
			} else {
				c.Password = pw
			}
		}
		if c.SSH.Password != "" {
			pw, err := decryptNCXPassword(c.SSH.Password)
			if err != nil {
				warnings = append(warnings, c.Name+": SSH 密码解密失败，已清空")
				c.SSH.Password = ""
			} else {
				c.SSH.Password = pw
			}
		}
		out, err := s.store.Upsert(c)
		if err != nil {
			warnings = append(warnings, c.Name+": "+err.Error())
			continue
		}
		imported = append(imported, out.Name)
	}
	writeJSON(w, http.StatusOK, map[string]any{"imported": imported, "warnings": warnings, "count": len(imported)})
}

func ncxInnerXML(data []byte) []byte {
	if !bytes.HasPrefix(data, []byte("PK")) {
		return data
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return data
	}
	for _, f := range zr.File {
		name := strings.ToLower(f.Name)
		if strings.HasSuffix(name, ".xml") || strings.HasSuffix(name, ".ncx") {
			rc, err := f.Open()
			if err != nil {
				continue
			}
			b, _ := io.ReadAll(io.LimitReader(rc, 10<<20))
			rc.Close()
			return b
		}
	}
	return data
}

func parseNCX(data []byte) ([]config.Connection, error) {
	trim := bytes.TrimSpace(data)
	if len(trim) == 0 {
		return nil, errors.New("empty NCX file")
	}
	if trim[0] == '{' || trim[0] == '[' {
		var raw any
		if err := json.Unmarshal(trim, &raw); err != nil {
			return nil, fmt.Errorf("cannot parse NCX JSON: %w", err)
		}
		// JSON exports are rare; accept a top-level object or array with host fields.
		if arr, ok := raw.([]any); ok {
			var out []config.Connection
			for _, item := range arr {
				if m, ok := item.(map[string]any); ok {
					out = append(out, connFromMap(m))
				}
			}
			return filterConns(out), nil
		}
		if m, ok := raw.(map[string]any); ok {
			return filterConns([]config.Connection{connFromMap(m)}), nil
		}
	}
	dec := xml.NewDecoder(bytes.NewReader(trim))
	var root *ncxNode
	var stack []*ncxNode
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("cannot parse NCX XML: %w", err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			n := &ncxNode{Name: strings.ToLower(t.Name.Local), Attrs: map[string]string{}}
			for _, a := range t.Attr {
				n.Attrs[strings.ToLower(a.Name.Local)] = a.Value
			}
			if len(stack) == 0 {
				root = n
			} else {
				parent := stack[len(stack)-1]
				parent.Kids = append(parent.Kids, n)
			}
			stack = append(stack, n)
		case xml.CharData:
			if len(stack) > 0 {
				stack[len(stack)-1].Text += string(t)
			}
		case xml.EndElement:
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
		}
	}
	if root == nil {
		return nil, errors.New("no XML content found in NCX")
	}
	var out []config.Connection
	var visit func(*ncxNode)
	seen := map[string]bool{}
	visit = func(n *ncxNode) {
		if c, ok := connFromNode(n); ok {
			key := c.Host + ":" + strconv.Itoa(c.Port) + "/" + c.User + "/" + c.Name
			if !seen[key] {
				seen[key] = true
				out = append(out, c)
			}
		}
		for _, k := range n.Kids {
			visit(k)
		}
	}
	visit(root)
	if len(out) == 0 {
		return nil, errors.New("no connection records found; this NCX may be encrypted or use an unsupported format")
	}
	return filterConns(out), nil
}

func filterConns(in []config.Connection) []config.Connection {
	out := in[:0]
	for _, c := range in {
		if strings.TrimSpace(c.Host) != "" {
			out = append(out, c)
		}
	}
	return out
}

func normKey(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

var ncxAliases = map[string][]string{
	"host":     {"host", "hostname", "host_name", "server", "address", "ip", "serverhost"},
	"port":     {"port", "portnumber", "serverport"},
	"user":     {"user", "username", "user_name", "uid", "login", "userid"},
	"password": {"password", "passwd", "pwd", "pass"},
	"name":     {"name", "connectionname", "connection_name", "title", "alias", "connname"},
	"database": {"database", "db", "defaultdatabase", "default_database", "schema", "databasename"},
	"sslmode":  {"sslmode", "ssl_mode", "ssl", "usessl"},
	"sshhost":  {"sshhost", "ssh_host", "sshtunnelhost"},
	"sshport":  {"sshport", "ssh_port"},
	"sshuser":  {"sshuser", "ssh_user", "sshusername", "ssh_username"},
	"sshpass":  {"sshpassword", "ssh_password", "sshpass"},
	"sshkey":   {"ssh_privatekey", "sshprivatekey", "privatekey", "keypath"},
	"sshauth":  {"ssh_authenmethod", "sshauthenmethod", "sshauth"},
}

func nodeValue(n *ncxNode, aliases []string, depth int) string {
	if n == nil {
		return ""
	}
	for _, a := range aliases {
		if v, ok := n.Attrs[a]; ok && strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	for _, k := range n.Kids {
		for _, a := range aliases {
			if normKey(k.Name) == normKey(a) && strings.TrimSpace(k.Text) != "" {
				return strings.TrimSpace(k.Text)
			}
		}
	}
	if depth > 0 {
		for _, k := range n.Kids {
			if v := nodeValue(k, aliases, depth-1); v != "" {
				return v
			}
		}
	}
	return ""
}

func connFromNode(n *ncxNode) (config.Connection, bool) {
	connType := strings.ToUpper(n.Attrs["conntype"])
	if connType != "" && connType != "MYSQL" && connType != "MARIADB" {
		return config.Connection{}, false
	}
	host := nodeValue(n, ncxAliases["host"], 1)
	if host == "" {
		return config.Connection{}, false
	}
	port, _ := strconv.Atoi(nodeValue(n, ncxAliases["port"], 1))
	if port <= 0 || port > 65535 {
		port = 3306
	}
	name := nodeValue(n, ncxAliases["name"], 1)
	if name == "" {
		name = host + ":" + strconv.Itoa(port)
	}
	user := nodeValue(n, ncxAliases["user"], 1)
	ssl := strings.EqualFold(n.Attrs["ssl"], "true")
	sslMode := "preferred"
	if ssl {
		sslMode = "required"
	}
	c := config.Connection{
		Name: name, Host: host, Port: port, User: user,
		Password: nodeValue(n, ncxAliases["password"], 1),
		Database: nodeValue(n, ncxAliases["database"], 1),
		Charset:  "utf8mb4", SSLMode: sslMode, Timeout: 10,
	}
	sshHost := nodeValue(n, ncxAliases["sshhost"], 1)
	sshOn := strings.EqualFold(n.Attrs["ssh"], "true") || sshHost != ""
	if sshOn {
		sshPort, _ := strconv.Atoi(nodeValue(n, ncxAliases["sshport"], 1))
		if sshPort == 0 {
			sshPort = 22
		}
		auth := "password"
		if strings.EqualFold(nodeValue(n, ncxAliases["sshauth"], 1), "PUBLICKEY") {
			auth = "key"
		}
		c.SSH = config.SSHConfig{
			Enabled: true, Host: sshHost, Port: sshPort,
			User: nodeValue(n, ncxAliases["sshuser"], 1), Auth: auth,
			Password: nodeValue(n, ncxAliases["sshpass"], 1),
			KeyPath:  nodeValue(n, ncxAliases["sshkey"], 1),
		}
	}
	return c, true
}

func connFromMap(m map[string]any) config.Connection {
	get := func(keys ...string) string {
		for k, v := range m {
			for _, key := range keys {
				if normKey(k) == normKey(key) && v != nil {
					return strings.TrimSpace(fmt.Sprint(v))
				}
			}
		}
		return ""
	}
	port, _ := strconv.Atoi(get("port"))
	if port <= 0 {
		port = 3306
	}
	return config.Connection{
		Name: get("name", "connectionName", "title"), Host: get("host", "hostname", "server"),
		Port: port, User: get("user", "username"), Password: get("password", "passwd"),
		Database: get("database", "db"), Charset: "utf8mb4", SSLMode: "preferred", Timeout: 10,
	}
}

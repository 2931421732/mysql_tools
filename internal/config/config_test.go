package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStoreEncryptsPasswords(t *testing.T) {
	dir := t.TempDir()
	store, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	saved, err := store.Upsert(Connection{
		Name: "demo", Host: "127.0.0.1", Port: 3306, User: "root", Password: "s3cret!@/",
		SSH: SSHConfig{Enabled: true, Host: "ssh", User: "u", Password: "sshpass"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if saved.ID == "" || !saved.HasSecret {
		t.Fatalf("unexpected saved connection: %#v", saved)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "connections.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "s3cret") || strings.Contains(string(raw), "sshpass") {
		t.Fatal("plaintext password leaked to disk")
	}

	reopened, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	conn, ok := reopened.Raw(saved.ID)
	if !ok || conn.Password != "s3cret!@/" || conn.SSH.Password != "sshpass" {
		t.Fatalf("password did not round-trip: %#v ok=%v", conn, ok)
	}
	public := reopened.Connections()
	if len(public) != 1 || public[0].Password != "" || public[0].SSH.Password != "" {
		t.Fatal("public listing leaked secrets")
	}

	if err := reopened.Delete(saved.ID); err != nil {
		t.Fatal(err)
	}
	if len(reopened.Connections()) != 0 {
		t.Fatal("delete did not persist")
	}
}

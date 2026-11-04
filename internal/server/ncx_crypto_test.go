package server

import (
	"testing"

	"navmini/internal/config"
)

func TestNCXPasswordRoundTrip(t *testing.T) {
	for _, plain := range []string{"secret", "p@ss word!", "中文密码", "1234567890123456"} {
		enc, err := encryptNCXPassword(plain)
		if err != nil {
			t.Fatal(err)
		}
		got, err := decryptNCXPassword(enc)
		if err != nil {
			t.Fatalf("%q: %v", plain, err)
		}
		if got != plain {
			t.Fatalf("roundtrip %q => %q (enc %s)", plain, got, enc)
		}
	}
	if _, err := decryptNCXPassword("not-hex"); err == nil {
		t.Fatal("expected invalid hex error")
	}
}

func TestNCXRealShape(t *testing.T) {
	xml := `<?xml version="1.0"?><Connections Ver="1.5">
<Connection ConnectionName="Shop MySQL" ConnType="MYSQL" Host="db.example.com" Port="3306" Database="shop" UserName="shop_app" Password="00000000000000000000000000000000" SavePassword="true" SSL="false" SSH="true" SSH_Host="bastion" SSH_Port="2222" SSH_UserName="deploy" SSH_AuthenMethod="PASSWORD" SSH_Password="" SSH_PrivateKey="" HTTP="false"/>
<Connection ConnectionName="Notes" ConnType="SQLITE" Host="" Port="0" UserName="" Database="" />
</Connections>`
	conns, err := parseNCX([]byte(xml))
	if err != nil {
		t.Fatal(err)
	}
	if len(conns) != 1 || conns[0].Host != "db.example.com" || conns[0].User != "shop_app" || !conns[0].SSH.Enabled || conns[0].SSH.User != "deploy" {
		t.Fatalf("conns = %#v", conns)
	}
}

func TestNCXConflictAndCopyName(t *testing.T) {
	base := config.Connection{Name: "A", Host: "h", Port: 3306, User: "u", Database: "d"}
	other := base
	other.ID = "id1"
	if got, ok := ncxFindConflict(base, []config.Connection{other}); !ok || got.ID != "id1" {
		t.Fatalf("expected conflict, got %#v %v", got, ok)
	}
	different := other
	different.Port = 3307
	if _, ok := ncxFindConflict(base, []config.Connection{different}); ok {
		t.Fatal("different port must not conflict")
	}
	names := map[string]bool{"a_copy": true}
	if got := ncxCopyName("A", names); got != "A_copy2" {
		t.Fatalf("copy name = %q", got)
	}
}

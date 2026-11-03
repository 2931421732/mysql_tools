package server

import "testing"

func TestParseNCXSimpleXML(t *testing.T) {
	conns, err := parseNCX([]byte(`<?xml version="1.0"?><Connections><Connection><Name>A</Name><Host>10.0.0.1</Host><Port>3307</Port><User>root</User><Password>secret</Password><Database>demo</Database></Connection></Connections>`))
	if err != nil {
		t.Fatal(err)
	}
	if len(conns) != 1 || conns[0].Host != "10.0.0.1" || conns[0].Port != 3307 || conns[0].User != "root" || conns[0].Password != "secret" {
		t.Fatalf("connections = %#v", conns)
	}
}

func TestParseNCXRejectsGarbage(t *testing.T) {
	if _, err := parseNCX([]byte{0x00, 0x01, 0x02}); err == nil {
		t.Fatal("expected error")
	}
}

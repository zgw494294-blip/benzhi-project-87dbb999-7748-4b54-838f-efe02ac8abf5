package main

import "testing"

func TestDefaultAndExplicitAddress(t *testing.T) {
	t.Setenv("PORT", "")
	cfg, err := parseConfig(nil)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Address != "127.0.0.1:19081" {
		t.Fatalf("address=%s", cfg.Address)
	}
	cfg, err = parseConfig([]string{"-addr=127.0.0.1:19991", "-selfcheck"})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Address != "127.0.0.1:19991" || !cfg.Selfcheck {
		t.Fatalf("config=%+v", cfg)
	}
}

func TestPortEnvironmentAndInvalidAddress(t *testing.T) {
	t.Setenv("PORT", "19888")
	cfg, err := parseConfig(nil)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Address != "127.0.0.1:19888" {
		t.Fatalf("address=%s", cfg.Address)
	}
	if _, err := parseConfig([]string{"-addr=0.0.0.0:not-a-port"}); err == nil {
		t.Fatal("invalid address should fail")
	}
}

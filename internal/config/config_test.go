package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultAndValidation(t *testing.T) {
	cfg := Default()
	if cfg.Port != 17081 || cfg.UIPort != 17082 {
		t.Fatalf("default ports = relay %d UI %d, want 17081 and 17082", cfg.Port, cfg.UIPort)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("default config invalid: %v", err)
	}
	cfg.Port = 0
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected invalid port")
	}
	cfg = Default()
	cfg.AllowedCIDRs = []string{"not-a-cidr"}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected invalid CIDR")
	}
}

func TestLoadMigratesMissingPortsAndPreservesSavedPorts(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	legacy := []byte(`{"key":"","bind":"0.0.0.0","language":"bn","allowed_cidrs":["127.0.0.0/8"],"upstream_timeout":15,"max_body_kb":1024,"tls_verify":true,"http_proxy":"","log_level":"info","ui_pin_hash":""}`)
	if err := os.WriteFile(path, legacy, 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Port != 17081 || cfg.UIPort != 17082 {
		t.Fatalf("missing ports did not migrate to defaults: %+v", cfg)
	}
	saved := cfg
	saved.Port, saved.UIPort = 18081, 18082
	data, err := json.Marshal(saved)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err = Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Port != 18081 || cfg.UIPort != 18082 {
		t.Fatalf("saved existing ports changed during load: %+v", cfg)
	}
}

func TestAtomicSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "config.json")
	cfg := Default()
	cfg.Port = 8089
	if err := Save(path, cfg); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Port != 8089 || got.Language != "bn" || !got.TLSVerify {
		t.Fatalf("unexpected loaded config: %+v", got)
	}
	cfg.Port = 8090
	if err := Save(path, cfg); err != nil {
		t.Fatal(err)
	}
	got, err = Load(path)
	if err != nil || got.Port != 8090 {
		t.Fatalf("overwrite did not persist: port=%d err=%v", got.Port, err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("config permissions = %o, want 600", info.Mode().Perm())
	}
}

func TestSaveRejectsInvalidConfigWithoutReplacingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	cfg := Default()
	if err := Save(path, cfg); err != nil {
		t.Fatal(err)
	}
	cfg.Port = 70000
	if err := Save(path, cfg); err == nil {
		t.Fatal("expected validation error")
	}
	got, err := Load(path)
	if err != nil || got.Port != 17081 {
		t.Fatalf("existing file changed after failed save: config=%+v err=%v", got, err)
	}
}

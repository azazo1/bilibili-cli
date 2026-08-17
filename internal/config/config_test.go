package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadMissingReturnsDefaultsWithoutCreatingFile(t *testing.T) {
	dir := t.TempDir()
	store := &Store{Dir: dir, File: filepath.Join(dir, "config.toml")}
	config, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if config.Version != CurrentVersion || config.Safety.ReadOnly || config.Output.Format != "auto" {
		t.Fatalf("unexpected defaults: %#v", config)
	}
	if _, err := os.Stat(store.File); !os.IsNotExist(err) {
		t.Fatalf("Load created config file: %v", err)
	}
}

func TestEnsureWritesDefaultTOML(t *testing.T) {
	dir := t.TempDir()
	store := &Store{Dir: dir, File: filepath.Join(dir, "config.toml")}
	config, err := store.Ensure()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(store.File)
	if err != nil {
		t.Fatal(err)
	}
	if config.Safety.ConfirmDangerousActions != true || len(data) == 0 {
		t.Fatalf("unexpected generated config: %#v", config)
	}
}

func TestLoadMigratesVersionZeroAndKeepsReadOnly(t *testing.T) {
	dir := t.TempDir()
	store := &Store{Dir: dir, File: filepath.Join(dir, "config.toml")}
	if err := os.WriteFile(store.File, []byte("[safety]\nread_only = true\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	config, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if config.Version != CurrentVersion || !config.Safety.ReadOnly || !config.Safety.ConfirmDangerousActions {
		t.Fatalf("migration lost safety settings: %#v", config)
	}
}

func TestLoadRejectsUnknownOutputMode(t *testing.T) {
	dir := t.TempDir()
	store := &Store{Dir: dir, File: filepath.Join(dir, "config.toml")}
	if err := os.WriteFile(store.File, []byte("version = 1\n[output]\nformat = \"xml\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Load(); err == nil {
		t.Fatal("Load accepted unknown output mode")
	}
}

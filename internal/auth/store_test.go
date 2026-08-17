package auth

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/azazo1/bilibili-cli/internal/api"
)

func TestStoreSaveLoadAndOptionalMode(t *testing.T) {
	dir := t.TempDir()
	store := &Store{
		Dir:  dir,
		File: filepath.Join(dir, "auth.json"),
		Now:  func() time.Time { return time.Unix(1700000000, 0) },
	}
	credential := &api.Credential{Sessdata: "session", BiliJct: "csrf", Buvid3: "device", AccessKey: "access", RefreshToken: "refresh"}
	if err := store.Save(credential); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.LoadSaved()
	if err != nil || loaded == nil || loaded.Sessdata != "session" || loaded.BiliJct != "csrf" || loaded.AccessKey != "access" || loaded.RefreshToken != "refresh" {
		t.Fatalf("LoadSaved() = %#v, %v", loaded, err)
	}
	got, err := store.GetCredential(context.Background(), ModeOptional)
	if err != nil || got == nil || got.Sessdata != credential.Sessdata {
		t.Fatalf("optional credential = %#v, %v", got, err)
	}
}

func TestCredentialCookieHeaderPreservesEscapedSession(t *testing.T) {
	credential := &api.Credential{Sessdata: "abc%2Cdef", BiliJct: "csrf"}
	if got := credential.CookieHeader(); got != "SESSDATA=abc%2Cdef; bili_jct=csrf; opus-goback=1" {
		t.Fatalf("CookieHeader() = %q", got)
	}
}

func TestWriteModeKeepsReadOnlySavedCredential(t *testing.T) {
	dir := t.TempDir()
	store := &Store{Dir: dir, File: filepath.Join(dir, "auth.json"), Now: time.Now}
	if err := store.Save(&api.Credential{Sessdata: "session"}); err != nil {
		t.Fatal(err)
	}
	credential, err := store.GetCredential(context.Background(), ModeWrite)
	if err != nil || credential != nil {
		t.Fatalf("write credential = %#v, %v", credential, err)
	}
	if saved, loadErr := store.LoadSaved(); loadErr != nil || saved == nil || saved.Sessdata != "session" {
		t.Fatalf("read-only credential was lost: %#v, %v", saved, loadErr)
	}
}

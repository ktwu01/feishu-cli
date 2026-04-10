package auth

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestResolveUserAccessToken_FlagValue(t *testing.T) {
	token, err := ResolveUserAccessToken("flag-token", "", "", "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token != "flag-token" {
		t.Errorf("got %q, want %q", token, "flag-token")
	}
}

func TestResolveUserAccessToken_EnvVar(t *testing.T) {
	t.Setenv("FEISHU_USER_ACCESS_TOKEN", "env-token")

	token, err := ResolveUserAccessToken("", "", "", "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token != "env-token" {
		t.Errorf("got %q, want %q", token, "env-token")
	}
}

func TestResolveUserAccessToken_TokenFile(t *testing.T) {
	// Clear env var
	t.Setenv("FEISHU_USER_ACCESS_TOKEN", "")

	tmpDir := t.TempDir()
	tokenFile := filepath.Join(tmpDir, "token.json")
	tokenPathFunc = func() (string, error) { return tokenFile, nil }
	defer func() { tokenPathFunc = originalTokenPath }()

	// Save a valid token
	store := &TokenStore{
		AccessToken: "file-token",
		ExpiresAt:   time.Now().Add(1 * time.Hour),
	}
	if err := SaveToken(store); err != nil {
		t.Fatalf("SaveToken error: %v", err)
	}

	token, err := ResolveUserAccessToken("", "", "", "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token != "file-token" {
		t.Errorf("got %q, want %q", token, "file-token")
	}
}

func TestResolveUserAccessToken_ConfigValue(t *testing.T) {
	t.Setenv("FEISHU_USER_ACCESS_TOKEN", "")

	// Point to nonexistent file
	tokenPathFunc = func() (string, error) { return filepath.Join(t.TempDir(), "nope.json"), nil }
	defer func() { tokenPathFunc = originalTokenPath }()

	token, err := ResolveUserAccessToken("", "config-token", "", "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token != "config-token" {
		t.Errorf("got %q, want %q", token, "config-token")
	}
}

func TestResolveUserAccessToken_AllEmpty(t *testing.T) {
	t.Setenv("FEISHU_USER_ACCESS_TOKEN", "")
	tokenPathFunc = func() (string, error) { return filepath.Join(t.TempDir(), "nope.json"), nil }
	defer func() { tokenPathFunc = originalTokenPath }()

	_, err := ResolveUserAccessToken("", "", "", "", "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestResolveUserAccessToken_Priority(t *testing.T) {
	// Flag > env > file > config
	t.Setenv("FEISHU_USER_ACCESS_TOKEN", "env-token")

	tmpDir := t.TempDir()
	tokenFile := filepath.Join(tmpDir, "token.json")
	tokenPathFunc = func() (string, error) { return tokenFile, nil }
	defer func() { tokenPathFunc = originalTokenPath }()

	store := &TokenStore{
		AccessToken: "file-token",
		ExpiresAt:   time.Now().Add(1 * time.Hour),
	}
	_ = SaveToken(store)

	// Flag wins
	token, _ := ResolveUserAccessToken("flag-token", "config-token", "", "", "")
	if token != "flag-token" {
		t.Errorf("got %q, want flag-token", token)
	}

	// Env wins when no flag
	token, _ = ResolveUserAccessToken("", "config-token", "", "", "")
	if token != "env-token" {
		t.Errorf("got %q, want env-token", token)
	}

	// File wins when no flag/env
	os.Unsetenv("FEISHU_USER_ACCESS_TOKEN")
	token, _ = ResolveUserAccessToken("", "config-token", "", "", "")
	if token != "file-token" {
		t.Errorf("got %q, want file-token", token)
	}
}


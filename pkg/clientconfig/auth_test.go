package clientconfig

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestTokenStoreLifecycle(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "config.yml")
	if token, err := Token(path, "https://one.example"); err != nil || token != "" {
		t.Fatalf("Token missing config = %q, %v", token, err)
	}
	if err := SaveToken(path, "https://two.example", "token-two"); err != nil {
		t.Fatalf("SaveToken two returned error: %v", err)
	}
	if err := SaveToken(path, "https://one.example", "token-one"); err != nil {
		t.Fatalf("SaveToken one returned error: %v", err)
	}
	if token, err := Token(path, "https://one.example"); err != nil || token != "token-one" {
		t.Fatalf("Token = %q, %v", token, err)
	}
	hosts, err := Hosts(path)
	if err != nil {
		t.Fatalf("Hosts returned error: %v", err)
	}
	if want := []string{"https://one.example", "https://two.example"}; !reflect.DeepEqual(hosts, want) {
		t.Fatalf("Hosts = %#v, want %#v", hosts, want)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat returned error: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("config mode = %o, want 600", got)
	}
	removed, err := RemoveToken(path, "https://one.example")
	if err != nil || !removed {
		t.Fatalf("RemoveToken = %v, %v", removed, err)
	}
	removed, err = RemoveToken(path, "https://missing.example")
	if err != nil || removed {
		t.Fatalf("RemoveToken missing = %v, %v", removed, err)
	}
}

func TestDefaultPathHonorsOverride(t *testing.T) {
	want := filepath.Join(t.TempDir(), "client.yml")
	t.Setenv("AGENT_COMPOSE_CONFIG", want)
	got, err := DefaultPath()
	if err != nil || got != want {
		t.Fatalf("DefaultPath = %q, %v, want %q", got, err, want)
	}
}

func TestLoadRejectsUnsupportedVersion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yml")
	if err := os.WriteFile(path, []byte("version: 2\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Hosts(path); err == nil {
		t.Fatal("Hosts returned nil error for unsupported version")
	}
}

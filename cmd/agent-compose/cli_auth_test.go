package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCLIAuthLoginPersistsAndReusesToken(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yml")
	t.Setenv("AGENT_COMPOSE_CONFIG", configPath)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer secret-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = io.WriteString(w, `{"err":null,"msg":"OK","data":{"timestamp":1783501631,"version":"test"}}`)
	}))
	defer server.Close()

	stdout, stderr, _, exitCode := executeCLICommand("auth", "login", "--host", server.URL, "--token", "secret-token")
	if exitCode != 0 || stderr != "" || !strings.Contains(stdout, "Authenticated "+server.URL) {
		t.Fatalf("login stdout/stderr/code = %q / %q / %d", stdout, stderr, exitCode)
	}
	info, err := os.Stat(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("config mode = %o, want 600", info.Mode().Perm())
	}

	stdout, stderr, _, exitCode = executeCLICommand("status", "--host", server.URL)
	if exitCode != 0 || stderr != "" || !strings.Contains(stdout, "test") {
		t.Fatalf("status stdout/stderr/code = %q / %q / %d", stdout, stderr, exitCode)
	}

	stdout, stderr, _, exitCode = executeCLICommand("auth", "list")
	if exitCode != 0 || stderr != "" || strings.TrimSpace(stdout) != server.URL {
		t.Fatalf("list stdout/stderr/code = %q / %q / %d", stdout, stderr, exitCode)
	}

	stdout, stderr, _, exitCode = executeCLICommand("auth", "logout", "--host", server.URL)
	if exitCode != 0 || stderr != "" || !strings.Contains(stdout, "Logged out "+server.URL) {
		t.Fatalf("logout stdout/stderr/code = %q / %q / %d", stdout, stderr, exitCode)
	}
}

func TestCLIAuthLoginFailureDoesNotPersistToken(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yml")
	t.Setenv("AGENT_COMPOSE_CONFIG", configPath)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	_, _, _, exitCode := executeCLICommand("auth", "login", "--host", server.URL, "--token", "wrong")
	if exitCode == 0 {
		t.Fatal("login exit code = 0, want failure")
	}
	if _, err := os.Stat(configPath); !os.IsNotExist(err) {
		t.Fatalf("config stat error = %v, want not exist", err)
	}
}

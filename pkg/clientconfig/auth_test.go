package clientconfig

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sync"
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

func TestConcurrentSaveTokenPreservesEveryHost(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yml")
	const count = 64
	start := make(chan struct{})
	errors := make(chan error, count)
	var workers sync.WaitGroup
	for index := range count {
		workers.Add(1)
		go func() {
			defer workers.Done()
			<-start
			host := fmt.Sprintf("https://host-%02d.example", index)
			errors <- SaveToken(path, host, fmt.Sprintf("token-%02d", index))
		}()
	}
	close(start)
	workers.Wait()
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatalf("SaveToken returned error: %v", err)
		}
	}
	hosts, err := Hosts(path)
	if err != nil {
		t.Fatalf("Hosts returned error: %v", err)
	}
	if len(hosts) != count {
		t.Fatalf("Hosts count = %d, want %d", len(hosts), count)
	}
	for index := range count {
		host := fmt.Sprintf("https://host-%02d.example", index)
		token, err := Token(path, host)
		if err != nil || token != fmt.Sprintf("token-%02d", index) {
			t.Fatalf("Token(%q) = %q, %v", host, token, err)
		}
	}
	lockInfo, err := os.Stat(path + ".lock")
	if err != nil {
		t.Fatalf("Stat lock returned error: %v", err)
	}
	if got := lockInfo.Mode().Perm(); got != 0o600 {
		t.Fatalf("lock mode = %o, want 600", got)
	}
}

func TestConcurrentProcessesPreserveEveryHost(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yml")
	const count = 16
	commands := make([]*exec.Cmd, 0, count)
	for index := range count {
		host := fmt.Sprintf("https://process-%02d.example", index)
		command := exec.Command(os.Args[0], "-test.run=^TestSaveTokenSubprocess$")
		command.Env = append(os.Environ(),
			"AGENT_COMPOSE_TEST_CONFIG_PATH="+path,
			"AGENT_COMPOSE_TEST_CONFIG_HOST="+host,
			"AGENT_COMPOSE_TEST_CONFIG_TOKEN="+fmt.Sprintf("token-%02d", index),
		)
		if err := command.Start(); err != nil {
			t.Fatalf("start subprocess %d: %v", index, err)
		}
		commands = append(commands, command)
	}
	for index, command := range commands {
		if err := command.Wait(); err != nil {
			t.Fatalf("subprocess %d failed: %v", index, err)
		}
	}
	hosts, err := Hosts(path)
	if err != nil {
		t.Fatalf("Hosts returned error: %v", err)
	}
	if len(hosts) != count {
		t.Fatalf("Hosts count = %d, want %d", len(hosts), count)
	}
}

func TestSaveTokenSubprocess(t *testing.T) {
	path := os.Getenv("AGENT_COMPOSE_TEST_CONFIG_PATH")
	if path == "" {
		t.Skip("subprocess helper")
	}
	if err := SaveToken(path, os.Getenv("AGENT_COMPOSE_TEST_CONFIG_HOST"), os.Getenv("AGENT_COMPOSE_TEST_CONFIG_TOKEN")); err != nil {
		t.Fatalf("SaveToken returned error: %v", err)
	}
}

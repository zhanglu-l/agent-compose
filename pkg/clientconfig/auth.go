// Package clientconfig persists CLI configuration that belongs to a local user.
package clientconfig

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

const currentVersion = 1

type fileConfig struct {
	Version int                 `yaml:"version"`
	Hosts   map[string]hostAuth `yaml:"hosts,omitempty"`
}

type hostAuth struct {
	Token string `yaml:"token"`
}

func DefaultPath() (string, error) {
	if configured := strings.TrimSpace(os.Getenv("AGENT_COMPOSE_CONFIG")); configured != "" {
		return configured, nil
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve user config directory: %w", err)
	}
	return filepath.Join(dir, "agent-compose", "config.yml"), nil
}

func Token(path, host string) (string, error) {
	config, err := load(path)
	if err != nil {
		return "", err
	}
	return config.Hosts[host].Token, nil
}

func SaveToken(path, host, token string) error {
	if strings.TrimSpace(host) == "" {
		return errors.New("host is required")
	}
	if token == "" {
		return errors.New("token is required")
	}
	return withConfigLock(path, func() error {
		config, err := load(path)
		if err != nil {
			return err
		}
		if config.Hosts == nil {
			config.Hosts = make(map[string]hostAuth)
		}
		config.Hosts[host] = hostAuth{Token: token}
		return write(path, config)
	})
}

func RemoveToken(path, host string) (bool, error) {
	removed := false
	err := withConfigLock(path, func() error {
		config, err := load(path)
		if err != nil {
			return err
		}
		if _, ok := config.Hosts[host]; !ok {
			return nil
		}
		delete(config.Hosts, host)
		if err := write(path, config); err != nil {
			return err
		}
		removed = true
		return nil
	})
	return removed, err
}

func Hosts(path string) ([]string, error) {
	config, err := load(path)
	if err != nil {
		return nil, err
	}
	hosts := make([]string, 0, len(config.Hosts))
	for host := range config.Hosts {
		hosts = append(hosts, host)
	}
	sort.Strings(hosts)
	return hosts, nil
}

func load(path string) (fileConfig, error) {
	content, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return fileConfig{Version: currentVersion, Hosts: make(map[string]hostAuth)}, nil
	}
	if err != nil {
		return fileConfig{}, fmt.Errorf("read client config %s: %w", path, err)
	}
	var config fileConfig
	if err := yaml.Unmarshal(content, &config); err != nil {
		return fileConfig{}, fmt.Errorf("parse client config %s: %w", path, err)
	}
	if config.Version != currentVersion {
		return fileConfig{}, fmt.Errorf("unsupported client config version %d", config.Version)
	}
	if config.Hosts == nil {
		config.Hosts = make(map[string]hostAuth)
	}
	return config, nil
}

func write(path string, config fileConfig) error {
	content, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("encode client config: %w", err)
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create client config directory %s: %w", dir, err)
	}
	temporary, err := os.CreateTemp(dir, ".config-*.yml")
	if err != nil {
		return fmt.Errorf("create temporary client config: %w", err)
	}
	temporaryPath := temporary.Name()
	defer func() { _ = os.Remove(temporaryPath) }()
	if err := temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("secure temporary client config: %w", err)
	}
	if _, err := temporary.Write(content); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("write temporary client config: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("sync temporary client config: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close temporary client config: %w", err)
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("replace client config %s: %w", path, err)
	}
	return nil
}

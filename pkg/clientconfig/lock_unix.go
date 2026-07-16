//go:build linux || darwin

package clientconfig

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

func withConfigLock(path string, operation func() error) (resultErr error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create client config directory %s: %w", dir, err)
	}
	lockPath := path + ".lock"
	lock, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR|syscall.O_NOFOLLOW, 0o600)
	if err != nil {
		return fmt.Errorf("open client config lock %s: %w", lockPath, err)
	}
	defer func() {
		resultErr = errors.Join(resultErr, lock.Close())
	}()
	if err := lock.Chmod(0o600); err != nil {
		return fmt.Errorf("secure client config lock %s: %w", lockPath, err)
	}
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX); err != nil {
		return fmt.Errorf("lock client config %s: %w", path, err)
	}
	defer func() {
		if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_UN); err != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("unlock client config %s: %w", path, err))
		}
	}()
	return operation()
}

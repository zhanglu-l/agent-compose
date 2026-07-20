package driver

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

const microsandboxEtcStatePath = "microsandbox-rootfs/etc"

// prepareMicrosandboxEtc copies the image's /etc into sandbox-owned state.
// Microsandbox bind rootfs directories are shared by every sandbox using the
// same materialized image. Its guest init writes instance-specific hosts and
// resolver configuration under /etc, so /etc must not remain in that shared
// directory.
func prepareMicrosandboxEtc(rootfsPath, sandboxStateDir string) (string, error) {
	source := filepath.Join(rootfsPath, "etc")
	info, err := os.Stat(source)
	if err != nil {
		return "", fmt.Errorf("stat microsandbox image /etc: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("microsandbox image /etc is not a directory")
	}

	target := filepath.Join(sandboxStateDir, filepath.FromSlash(microsandboxEtcStatePath))
	if _, err := os.Stat(target); err == nil {
		return target, nil
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("stat sandbox-owned microsandbox /etc: %w", err)
	}

	parent := filepath.Dir(target)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return "", fmt.Errorf("create sandbox-owned microsandbox rootfs state: %w", err)
	}
	temporary, err := os.MkdirTemp(parent, ".etc-")
	if err != nil {
		return "", fmt.Errorf("create temporary sandbox-owned microsandbox /etc: %w", err)
	}
	defer func() { _ = os.RemoveAll(temporary) }()

	if err := copyMicrosandboxEtc(source, temporary); err != nil {
		return "", err
	}
	if err := os.Rename(temporary, target); err != nil {
		if _, statErr := os.Stat(target); statErr == nil {
			return target, nil
		}
		return "", fmt.Errorf("publish sandbox-owned microsandbox /etc: %w", err)
	}
	return target, nil
}

func copyMicrosandboxEtc(source, target string) error {
	return filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		if relative == "." {
			return nil
		}

		destination := filepath.Join(target, relative)
		info, err := entry.Info()
		if err != nil {
			return err
		}
		switch {
		case entry.IsDir():
			if err := os.Mkdir(destination, info.Mode().Perm()); err != nil && !os.IsExist(err) {
				return err
			}
			return nil
		case info.Mode()&os.ModeSymlink != 0:
			linkTarget, err := os.Readlink(path)
			if err != nil {
				return err
			}
			return os.Symlink(linkTarget, destination)
		case info.Mode().IsRegular():
			return copyMicrosandboxEtcFile(path, destination, info.Mode().Perm())
		default:
			return fmt.Errorf("copy microsandbox image /etc: unsupported entry %s (%s)", path, info.Mode().Type())
		}
	})
}

func copyMicrosandboxEtcFile(source, target string, mode fs.FileMode) (err error) {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer func() { _ = input.Close() }()

	output, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := output.Close(); err == nil {
			err = closeErr
		}
	}()
	_, err = io.Copy(output, input)
	return err
}

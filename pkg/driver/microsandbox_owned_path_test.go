//go:build linux && cgo && microsandboxcgo

package driver

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateMicrosandboxRootfsOwnedPathRejectsIntermediateSymlinkEscape(t *testing.T) {
	home := t.TempDir()
	diskRoot := filepath.Join(home, "rootfs-disks")
	if err := os.MkdirAll(diskRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	outsideDisk := filepath.Join(outside, "outside.qcow2")
	if err := os.WriteFile(outsideDisk, []byte("outside"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(diskRoot, "escape")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("create symlink: %v", err)
	}

	err := validateMicrosandboxRootfsOwnedPath(home, filepath.Join(link, filepath.Base(outsideDisk)))
	if err == nil || !strings.Contains(err.Error(), "through a symlink") {
		t.Fatalf("validateMicrosandboxRootfsOwnedPath error = %v, want symlink escape rejection", err)
	}
	if data, readErr := os.ReadFile(outsideDisk); readErr != nil || string(data) != "outside" {
		t.Fatalf("outside disk changed: data=%q err=%v", data, readErr)
	}
}

func TestValidateMicrosandboxRootfsOwnedPathAcceptsRegularFile(t *testing.T) {
	home := t.TempDir()
	diskRoot := filepath.Join(home, "rootfs-disks")
	if err := os.MkdirAll(diskRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	disk := filepath.Join(diskRoot, "sandbox.qcow2")
	if err := os.WriteFile(disk, []byte("disk"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := validateMicrosandboxRootfsOwnedPath(home, disk); err != nil {
		t.Fatalf("validateMicrosandboxRootfsOwnedPath: %v", err)
	}
}

func TestValidateMicrosandboxAnyOwnedPathRejectsLegacyDockerDisk(t *testing.T) {
	home := t.TempDir()
	diskRoot := filepath.Join(home, "docker-disks")
	if err := os.MkdirAll(diskRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	disk := filepath.Join(diskRoot, "sandbox.raw")
	if err := os.WriteFile(disk, []byte("disk"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := validateMicrosandboxAnyOwnedPath(home, disk); err == nil || !strings.Contains(err.Error(), "outside rootfs-disks") {
		t.Fatalf("validateMicrosandboxAnyOwnedPath error = %v, want legacy disk rejection", err)
	}
}

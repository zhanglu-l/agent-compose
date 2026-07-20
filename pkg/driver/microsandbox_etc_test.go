package driver

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPrepareMicrosandboxEtcIsolatesSandboxesFromSharedRootfs(t *testing.T) {
	root := t.TempDir()
	rootfs := filepath.Join(root, "image", "rootfs")
	mustWriteMicrosandboxEtcTestFile(t, filepath.Join(rootfs, "etc", "hosts"), "image-hosts\n", 0o644)
	mustWriteMicrosandboxEtcTestFile(t, filepath.Join(rootfs, "etc", "resolv.conf"), "nameserver image\n", 0o644)
	if err := os.Symlink("../run/resolv.conf", filepath.Join(rootfs, "etc", "resolv-link")); err != nil {
		t.Fatalf("create image /etc symlink: %v", err)
	}

	first, err := prepareMicrosandboxEtc(rootfs, filepath.Join(root, "sandbox-a", "state"))
	if err != nil {
		t.Fatalf("prepare first sandbox /etc: %v", err)
	}
	second, err := prepareMicrosandboxEtc(rootfs, filepath.Join(root, "sandbox-b", "state"))
	if err != nil {
		t.Fatalf("prepare second sandbox /etc: %v", err)
	}
	if first == second {
		t.Fatalf("sandbox /etc paths alias: %q", first)
	}

	mustWriteMicrosandboxEtcTestFile(t, filepath.Join(first, "hosts"), "sandbox-a\n", 0o644)
	assertMicrosandboxEtcTestFile(t, filepath.Join(second, "hosts"), "image-hosts\n")
	assertMicrosandboxEtcTestFile(t, filepath.Join(rootfs, "etc", "hosts"), "image-hosts\n")
	linkTarget, err := os.Readlink(filepath.Join(first, "resolv-link"))
	if err != nil {
		t.Fatalf("read copied /etc symlink: %v", err)
	}
	if linkTarget != "../run/resolv.conf" {
		t.Fatalf("copied /etc symlink target = %q", linkTarget)
	}
}

func TestPrepareMicrosandboxEtcReusesSandboxState(t *testing.T) {
	root := t.TempDir()
	rootfs := filepath.Join(root, "image", "rootfs")
	mustWriteMicrosandboxEtcTestFile(t, filepath.Join(rootfs, "etc", "hosts"), "image-hosts\n", 0o644)
	state := filepath.Join(root, "sandbox", "state")

	first, err := prepareMicrosandboxEtc(rootfs, state)
	if err != nil {
		t.Fatalf("prepare sandbox /etc: %v", err)
	}
	mustWriteMicrosandboxEtcTestFile(t, filepath.Join(first, "hosts"), "runtime-hosts\n", 0o644)
	second, err := prepareMicrosandboxEtc(rootfs, state)
	if err != nil {
		t.Fatalf("reuse sandbox /etc: %v", err)
	}
	if second != first {
		t.Fatalf("reused sandbox /etc = %q, want %q", second, first)
	}
	assertMicrosandboxEtcTestFile(t, filepath.Join(second, "hosts"), "runtime-hosts\n")
}

func TestPrepareMicrosandboxEtcRejectsMissingImageEtc(t *testing.T) {
	_, err := prepareMicrosandboxEtc(filepath.Join(t.TempDir(), "rootfs"), t.TempDir())
	if err == nil {
		t.Fatal("prepare sandbox /etc returned nil error")
	}
}

func mustWriteMicrosandboxEtcTestFile(t *testing.T, path, content string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create test file parent: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatalf("write test file: %v", err)
	}
}

func assertMicrosandboxEtcTestFile(t *testing.T, path, want string) {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if string(content) != want {
		t.Fatalf("%s = %q, want %q", path, content, want)
	}
}

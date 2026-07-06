//go:build cgo

package driver

import (
	appconfig "agent-compose/pkg/config"
	"os"
	"path/filepath"
	"testing"
)

func TestMicrosandboxExecCollectorMapsStdioStreams(t *testing.T) {
	var streamed []ExecChunk
	collector := &microsandboxExecCollector{stream: func(chunk ExecChunk) {
		streamed = append(streamed, chunk)
	}}
	collector.writeChunk(ExecChunk{Text: "out"})
	collector.writeChunk(ExecChunk{Text: "err", Stream: StdioStderr})

	if collector.stdout.String() != "out" {
		t.Fatalf("stdout = %q", collector.stdout.String())
	}
	if collector.stderr.String() != "err" {
		t.Fatalf("stderr = %q", collector.stderr.String())
	}
	if collector.output.String() != "outerr" {
		t.Fatalf("output = %q", collector.output.String())
	}
	want := []ExecChunk{{Text: "out"}, {Text: "err", Stream: StdioStderr}}
	if len(streamed) != len(want) {
		t.Fatalf("streamed chunks = %#v", streamed)
	}
	for i := range want {
		if streamed[i] != want[i] {
			t.Fatalf("streamed[%d] = %#v, want %#v", i, streamed[i], want[i])
		}
	}
}

func TestMicrosandboxResolveLibkrunfwPrefersVersionedRealFile(t *testing.T) {
	libDir := t.TempDir()
	versioned := filepath.Join(libDir, "libkrunfw.so.5.5.0")
	if err := os.WriteFile(versioned, []byte("krun"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("libkrunfw.so.5.5.0", filepath.Join(libDir, "libkrunfw.so.5")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("libkrunfw.so.5", filepath.Join(libDir, "libkrunfw.so")); err != nil {
		t.Fatal(err)
	}

	runtime := &microsandboxRuntime{config: &appconfig.Config{
		MicrosandboxLibPath: filepath.Join(libDir, "libmicrosandbox_go_ffi.so"),
	}}
	if got := runtime.resolveLibkrunfwPath(); got != versioned {
		t.Fatalf("resolveLibkrunfwPath() = %q, want %q", got, versioned)
	}
}

func TestMicrosandboxResolveLibkrunfwUsesNumericVersionOrder(t *testing.T) {
	libDir := t.TempDir()
	for _, name := range []string{
		"libkrunfw.so.5.2.1",
		"libkrunfw.so.5.10.0",
		"libkrunfw.so.5.99.0.bak",
	} {
		if err := os.WriteFile(filepath.Join(libDir, name), []byte("krun"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	runtime := &microsandboxRuntime{config: &appconfig.Config{
		MicrosandboxLibPath: filepath.Join(libDir, "libmicrosandbox_go_ffi.so"),
	}}
	want := filepath.Join(libDir, "libkrunfw.so.5.10.0")
	if got := runtime.resolveLibkrunfwPath(); got != want {
		t.Fatalf("resolveLibkrunfwPath() = %q, want %q", got, want)
	}
}

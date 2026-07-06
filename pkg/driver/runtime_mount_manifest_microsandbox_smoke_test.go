//go:build cgo

package driver

import (
	"context"
	"testing"
	"time"

	microsandbox "github.com/superradcompany/microsandbox/sdk/go"
)

func TestSmokeMicrosandboxRuntimeMountManifestDirectoryOnlyStarts(t *testing.T) {
	runtimeSmokeEnabled(t, RuntimeDriverMicrosandbox)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	config := newRuntimeSmokeConfig(t, RuntimeDriverMicrosandbox)
	session, vmState, proxyState := newRuntimeSmokeSession(t, ctx, config, RuntimeDriverMicrosandbox)
	assertDirectoryOnlyRuntimeSmokeManifest(t, session, RuntimeDriverMicrosandbox)

	runtime := &microsandboxRuntime{config: config, lifecycleHandles: map[string]*microsandbox.Sandbox{}}
	info, err := runtime.EnsureSession(ctx, session, vmState, proxyState)
	if err != nil {
		t.Fatalf("EnsureSession returned error: %v", err)
	}
	vmState.BoxID = info.BoxID
	t.Cleanup(func() {
		if t.Failed() && runtimeSmokeKeepTmp() {
			return
		}
		stopCtx, stopCancel := context.WithTimeout(context.Background(), config.SessionStopTimeout)
		defer stopCancel()
		_, _ = runtime.StopSession(stopCtx, session, vmState)
	})
	assertMicrosandboxRuntimeSmokeGuestPaths(t, ctx, runtime, session, vmState)
	assertRuntimeSmokeHomeFiles(t, ctx, runtime, session, vmState)
}

func TestSmokeMicrosandboxUsesGoContainerRegistryOCIImage(t *testing.T) {
	runtimeSmokeEnabled(t, RuntimeDriverMicrosandbox)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	config := newRuntimeSmokeConfig(t, RuntimeDriverMicrosandbox)
	config.MicrosandboxDefaultImage = prepareRuntimeSmokeGoContainerRegistryOCIImage(t, ctx, config)
	session, vmState, proxyState := newRuntimeSmokeSession(t, ctx, config, RuntimeDriverMicrosandbox)
	assertDirectoryOnlyRuntimeSmokeManifest(t, session, RuntimeDriverMicrosandbox)

	runtime := &microsandboxRuntime{config: config, lifecycleHandles: map[string]*microsandbox.Sandbox{}}
	info, err := runtime.EnsureSession(ctx, session, vmState, proxyState)
	if err != nil {
		t.Fatalf("EnsureSession returned error: %v", err)
	}
	vmState.BoxID = info.BoxID
	t.Cleanup(func() {
		if t.Failed() && runtimeSmokeKeepTmp() {
			return
		}
		stopCtx, stopCancel := context.WithTimeout(context.Background(), config.SessionStopTimeout)
		defer stopCancel()
		_, _ = runtime.StopSession(stopCtx, session, vmState)
	})
	assertMicrosandboxRuntimeSmokeGuestPaths(t, ctx, runtime, session, vmState)
	assertRuntimeSmokeHomeFiles(t, ctx, runtime, session, vmState)
}

func assertMicrosandboxRuntimeSmokeGuestPaths(t *testing.T, ctx context.Context, runtime *microsandboxRuntime, session *Session, vmState VMState) {
	t.Helper()
	result, err := runtime.Exec(ctx, session, vmState, ExecSpec{Command: "sh", Args: []string{"-lc", runtimeSmokeGuestPathAssertionScript()}, Cwd: "/"})
	if err != nil {
		t.Fatalf("microsandbox guest path assertion command returned error: %v", err)
	}
	if !result.Success {
		t.Fatalf("microsandbox guest path assertion failed: exit=%d stdout=%q stderr=%q", result.ExitCode, result.Stdout, result.Stderr)
	}
}

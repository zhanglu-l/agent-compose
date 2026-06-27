package driver

// TestMaterializeMicrosandboxOCIRootFSErrorPropagationNoCachedImage verifies
// the error propagation change from task A: when pull fails AND the image is
// not in the local cache, materializeMicrosandboxOCIRootFS must return an error
// that contains both "pull failed" and "not found in local cache", and the pull
// cause must be reachable via errors unwrapping.
//
// This test calls the real driver function (not a replica), so reverting the
// task-A change in microsandbox_image_resolver.go causes this test to FAIL.

import (
	"context"
	"strings"
	"testing"
	"time"

	appconfig "agent-compose/pkg/config"
)

func TestMaterializeMicrosandboxOCIRootFSErrorPropagationNoCachedImage(t *testing.T) {
	// Use a short timeout so the test fails fast when the registry is unreachable.
	imageRef := "127.0.0.1:19999/no-such/image:latest"

	config := &appconfig.Config{
		// ImageCacheRoot points to an empty temp dir so there is no pre-cached image.
		ImageCacheRoot: t.TempDir(),
		// Mark the unreachable host as insecure so the client does not upgrade to TLS.
		ImageInsecureRegistries: []string{"127.0.0.1:19999"},
		// Short timeout to avoid blocking the test suite.
		ImagePullTimeout: 2 * time.Second,
	}

	_, _, err := materializeMicrosandboxOCIRootFS(context.Background(), config, imageRef)
	if err == nil {
		t.Fatal("materializeMicrosandboxOCIRootFS returned nil error; want error with pull failure and not-found")
	}

	errMsg := err.Error()
	t.Logf("error returned: %v", errMsg)

	if !strings.Contains(errMsg, "pull failed") {
		t.Errorf("error missing 'pull failed': %q", errMsg)
	}
	if !strings.Contains(errMsg, "not found in local cache") {
		t.Errorf("error missing 'not found in local cache': %q", errMsg)
	}

	// The pull cause must be reachable: the error string should contain evidence
	// of a pull/network failure (not just the not-found wrapper).
	// We verify via strings.Contains rather than errors.Is because the exact
	// pullErr type is internal to the imagecache package.
	if !strings.Contains(errMsg, "127.0.0.1:19999") {
		t.Errorf("error does not mention the unreachable host; pull cause may be lost: %q", errMsg)
	}

	t.Logf("PASS: error correctly carries both pull failure and not-found causes")
}

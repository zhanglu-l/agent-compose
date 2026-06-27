package imagecache

// Integration tests for the always-pull mechanism using an in-process httptest
// registry. These tests do not require a Docker daemon.
//
// Two mechanisms are tested:
//   1. Always-pull detects same-tag content update (imageID changes after push of new digest).
//   2. Pull failure degrades to local cache (materialize still succeeds).

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/go-containerregistry/pkg/registry"
)

// TestIntegrationAlwaysPullDetectsSameTagUpdate verifies that after pushing new
// content under the same tag, a second Pull+MaterializeRootFS call returns a
// different ImageID, proving that always-pull actually picked up the updated
// image rather than serving the stale local cache.
func TestIntegrationAlwaysPullDetectsSameTagUpdate(t *testing.T) {
	ctx := context.Background()
	server := httptest.NewServer(registry.New())
	t.Cleanup(server.Close)
	host := strings.TrimPrefix(server.URL, "http://")
	refString := host + "/always-pull/app:latest"

	cache := newPullTestCache(t, host)

	// Push version A.
	imgA := newRegistryTestImage(t, Platform{OS: "linux", Architecture: "amd64"}, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), map[string]string{"version": "A"})
	pushTestImage(t, ctx, refString, imgA)

	// Pull and materialize version A.
	_, err := cache.Pull(ctx, PullRequest{Reference: refString, Platform: Platform{OS: "linux", Architecture: "amd64"}})
	if err != nil {
		t.Fatalf("Pull (version A) returned error: %v", err)
	}
	resultA, err := cache.MaterializeRootFS(ctx, refString)
	if err != nil {
		t.Fatalf("MaterializeRootFS (version A) returned error: %v", err)
	}
	imageIDA := resultA.ImageID
	if imageIDA == "" {
		t.Fatalf("MaterializeRootFS (version A) returned empty ImageID")
	}
	t.Logf("version A ImageID: %s", imageIDA)

	// Push version B (different content, same tag — simulates registry update).
	imgB := newRegistryTestImage(t, Platform{OS: "linux", Architecture: "amd64"}, time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC), map[string]string{"version": "B"})
	pushTestImage(t, ctx, refString, imgB)

	// Pull again — always-pull must fetch updated digest.
	_, err = cache.Pull(ctx, PullRequest{Reference: refString, Platform: Platform{OS: "linux", Architecture: "amd64"}})
	if err != nil {
		t.Fatalf("Pull (version B) returned error: %v", err)
	}
	resultB, err := cache.MaterializeRootFS(ctx, refString)
	if err != nil {
		t.Fatalf("MaterializeRootFS (version B) returned error: %v", err)
	}
	imageIDB := resultB.ImageID
	if imageIDB == "" {
		t.Fatalf("MaterializeRootFS (version B) returned empty ImageID")
	}
	t.Logf("version B ImageID: %s", imageIDB)

	if imageIDA == imageIDB {
		t.Fatalf("always-pull did not detect content update: ImageID unchanged %s", imageIDA)
	}
	t.Logf("PASS: ImageID changed from %s to %s — always-pull replaced local cache", imageIDA, imageIDB)
}

// TestIntegrationAlwaysPullFallsBackToLocalOnRegistryFailure verifies that
// when the registry becomes unreachable after a successful first pull (i.e.
// the image exists in local cache), pull failure is tolerated and
// MaterializeRootFS still succeeds using the cached image.
func TestIntegrationAlwaysPullFallsBackToLocalOnRegistryFailure(t *testing.T) {
	ctx := context.Background()
	server := httptest.NewServer(registry.New())
	host := strings.TrimPrefix(server.URL, "http://")
	refString := host + "/fallback/app:latest"

	cache := newPullTestCache(t, host)

	// Push an image and pull it once so the local cache is populated.
	img := newRegistryTestImage(t, Platform{OS: "linux", Architecture: "amd64"}, time.Now().UTC(), nil)
	pushTestImage(t, ctx, refString, img)

	_, err := cache.Pull(ctx, PullRequest{Reference: refString, Platform: Platform{OS: "linux", Architecture: "amd64"}})
	if err != nil {
		t.Fatalf("initial Pull returned error: %v", err)
	}
	firstResult, err := cache.MaterializeRootFS(ctx, refString)
	if err != nil {
		t.Fatalf("initial MaterializeRootFS returned error: %v", err)
	}
	t.Logf("initial ImageID: %s", firstResult.ImageID)

	// Shut down the registry to simulate unavailability.
	server.Close()

	// Pull should fail (registry down); we ignore the pull error because the
	// always-pull mechanism treats it as best-effort.
	_, pullErr := cache.Pull(ctx, PullRequest{Reference: refString, Platform: Platform{OS: "linux", Architecture: "amd64"}})
	if pullErr == nil {
		t.Logf("Pull after server close returned nil (possibly cached by transport); continuing")
	} else {
		t.Logf("Pull after server close returned error (expected): %v", pullErr)
	}

	// MaterializeRootFS must still succeed using the local cache.
	fallbackResult, err := cache.MaterializeRootFS(ctx, refString)
	if err != nil {
		t.Fatalf("MaterializeRootFS after registry failure returned error: %v", err)
	}
	if fallbackResult.ImageID == "" {
		t.Fatalf("MaterializeRootFS returned empty ImageID after registry failure")
	}
	t.Logf("PASS: MaterializeRootFS succeeded with ImageID %s after registry became unreachable", fallbackResult.ImageID)
}


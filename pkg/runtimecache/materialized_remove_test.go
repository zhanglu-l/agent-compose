package runtimecache

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"agent-compose/pkg/imagecache"
)

func TestMaterializedPruneSkipsReferencedByDefault(t *testing.T) {
	cache, image := materializedRemovalFixture(t)
	layoutPath := cache.MaterializedOCILayoutPath(image.ConfigDigest)
	items := scanMaterializedItems(t, cache)
	layout := requireItem(t, items, layoutPath)

	result, err := PruneItems(context.Background(), []Item{layout}, PruneRequest{Force: true}, time.Now(), MaterializedRemover{Cache: cache}.Remove)
	if err != nil {
		t.Fatalf("PruneItems returned error: %v", err)
	}
	if len(result.Removed) != 0 || len(result.Skipped) != 1 {
		t.Fatalf("result = %#v, want referenced skipped", result)
	}
	assertPathExists(t, layoutPath)
}

func TestMaterializedPruneIncludeReferencedDeletesLayoutAndReadyOnly(t *testing.T) {
	cache, image := materializedRemovalFixture(t)
	imageDir := cache.MaterializedImageDir(image.ConfigDigest)
	layoutPath := cache.MaterializedOCILayoutPath(image.ConfigDigest)
	rootfsPath := cache.MaterializedRootFSPath(image.ConfigDigest)
	layoutReady := filepath.Join(imageDir, ".ready")
	items := scanMaterializedItems(t, cache)
	layout := requireItem(t, items, layoutPath)

	result, err := PruneItems(context.Background(), []Item{layout}, PruneRequest{Force: true, IncludeReferenced: true}, time.Now(), MaterializedRemover{Cache: cache}.Remove)
	if err != nil {
		t.Fatalf("PruneItems returned error: %v", err)
	}
	if !reflect.DeepEqual(result.Removed, []string{layout.CacheID}) {
		t.Fatalf("Removed = %#v, want layout id", result.Removed)
	}
	assertPathMissing(t, layoutPath)
	assertPathMissing(t, layoutReady)
	assertPathExists(t, rootfsPath)
}

func TestMaterializedRemoveRootFSDeletesRootFSReady(t *testing.T) {
	cache, image := materializedRemovalFixture(t)
	imageDir := cache.MaterializedImageDir(image.ConfigDigest)
	rootfsPath := cache.MaterializedRootFSPath(image.ConfigDigest)
	rootfsReady := filepath.Join(imageDir, ".rootfs.ready")
	items := scanMaterializedItems(t, cache)
	rootfs := requireItem(t, items, rootfsPath)

	result, err := PruneItems(context.Background(), []Item{rootfs}, PruneRequest{Force: true, IncludeReferenced: true}, time.Now(), MaterializedRemover{Cache: cache}.Remove)
	if err != nil {
		t.Fatalf("PruneItems returned error: %v", err)
	}
	if !reflect.DeepEqual(result.Removed, []string{rootfs.CacheID}) {
		t.Fatalf("Removed = %#v, want rootfs id", result.Removed)
	}
	assertPathMissing(t, rootfsPath)
	assertPathMissing(t, rootfsReady)
}

func TestMaterializedRemoveTempDirDoesNotDeleteSiblings(t *testing.T) {
	cache, image := materializedRemovalFixture(t)
	imageDir := cache.MaterializedImageDir(image.ConfigDigest)
	tmpPath := filepath.Join(imageDir, "rootfs.tmp")
	layoutPath := cache.MaterializedOCILayoutPath(image.ConfigDigest)
	rootfsPath := cache.MaterializedRootFSPath(image.ConfigDigest)
	items := scanMaterializedItems(t, cache)
	tmp := requireItem(t, items, tmpPath)

	result, err := PruneItems(context.Background(), []Item{tmp}, PruneRequest{Force: true}, time.Now(), MaterializedRemover{Cache: cache}.Remove)
	if err != nil {
		t.Fatalf("PruneItems returned error: %v", err)
	}
	if !reflect.DeepEqual(result.Removed, []string{tmp.CacheID}) {
		t.Fatalf("Removed = %#v, want temp id", result.Removed)
	}
	assertPathMissing(t, tmpPath)
	assertPathExists(t, layoutPath)
	assertPathExists(t, rootfsPath)
}

func TestMaterializedRemoveOrphanedAndExpiredItems(t *testing.T) {
	cache := newRuntimeCacheImageCache(t)
	orphanPath := filepath.Join(cache.MaterializationRoot(), "orphaned-image", "oci")
	if err := os.MkdirAll(orphanPath, 0o755); err != nil {
		t.Fatalf("mkdir orphan: %v", err)
	}
	orphan := requireItem(t, scanMaterializedItems(t, cache), orphanPath)
	result, err := PruneItems(context.Background(), []Item{orphan}, PruneRequest{Force: true}, time.Now(), MaterializedRemover{Cache: cache}.Remove)
	if err != nil {
		t.Fatalf("PruneItems orphan returned error: %v", err)
	}
	if !reflect.DeepEqual(result.Removed, []string{orphan.CacheID}) {
		t.Fatalf("orphan Removed = %#v, want orphan id", result.Removed)
	}
	assertPathMissing(t, orphanPath)

	expiredPath := filepath.Join(cache.MaterializationRoot(), "expired-image", "oci")
	if err := os.MkdirAll(expiredPath, 0o755); err != nil {
		t.Fatalf("mkdir expired: %v", err)
	}
	expired := requireItem(t, scanMaterializedItems(t, cache), expiredPath)
	expired.Status = StatusExpired
	result, err = PruneItems(context.Background(), []Item{expired}, PruneRequest{Force: true}, time.Now(), MaterializedRemover{Cache: cache}.Remove)
	if err != nil {
		t.Fatalf("PruneItems expired returned error: %v", err)
	}
	if !reflect.DeepEqual(result.Removed, []string{expired.CacheID}) {
		t.Fatalf("expired Removed = %#v, want expired id", result.Removed)
	}
	assertPathMissing(t, expiredPath)
}

func TestMaterializedRemoverRejectsMismatchedCacheID(t *testing.T) {
	cache, image := materializedRemovalFixture(t)
	item := requireItem(t, scanMaterializedItems(t, cache), cache.MaterializedOCILayoutPath(image.ConfigDigest))
	item.CacheID = "materialized-image-cache:all:materialized-oci-layout:0123456789abcdef"
	if err := (MaterializedRemover{Cache: cache}).Remove(context.Background(), item); err == nil {
		t.Fatal("Remove returned nil error for mismatched cache id")
	}
}

func materializedRemovalFixture(t *testing.T) (*imagecache.Cache, imagecache.ImageMetadata) {
	t.Helper()
	cache := newRuntimeCacheImageCache(t)
	image := imagecache.ImageMetadata{
		CacheKey:       "sha256:config-remove",
		RequestedRef:   "remove",
		NormalizedRef:  "registry.example/remove:latest",
		ManifestDigest: "sha256:manifest-remove",
		ConfigDigest:   "sha256:config-remove",
	}
	if err := cache.SaveMetadata(imagecache.MetadataFile{Images: []imagecache.ImageMetadata{image}}); err != nil {
		t.Fatalf("SaveMetadata returned error: %v", err)
	}
	imageDir := cache.MaterializedImageDir(image.ConfigDigest)
	for _, dir := range []string{
		cache.MaterializedOCILayoutPath(image.ConfigDigest),
		cache.MaterializedRootFSPath(image.ConfigDigest),
		filepath.Join(imageDir, "rootfs.tmp", "rootfs"),
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}
	for _, path := range []string{
		filepath.Join(cache.MaterializedOCILayoutPath(image.ConfigDigest), "index.json"),
		filepath.Join(cache.MaterializedRootFSPath(image.ConfigDigest), "bin"),
		filepath.Join(imageDir, ".ready"),
		filepath.Join(imageDir, ".rootfs.ready"),
	} {
		if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}
	return cache, image
}

func scanMaterializedItems(t *testing.T, cache *imagecache.Cache) []Item {
	t.Helper()
	result, err := (MaterializedScanner{Cache: cache}).List(context.Background())
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	return result.Items
}

func assertPathExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected %s to exist: %v", path, err)
	}
}

func assertPathMissing(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected %s to be missing, stat err=%v", path, err)
	}
}

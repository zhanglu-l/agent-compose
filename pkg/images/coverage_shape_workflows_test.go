package images

import (
	"archive/tar"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	buildtypes "github.com/docker/docker/api/types/build"
	typesimage "github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"

	appconfig "agent-compose/pkg/config"
	"agent-compose/pkg/imagecache"
	agentcomposev2 "agent-compose/proto/agentcompose/v2"
)

func TestImageBackendAndMappingCoverageWorkflows(t *testing.T) {
	ctx := context.Background()
	docker := fakeImageBackend{name: "docker"}
	oci := fakeImageBackend{name: "oci"}
	auto := NewAutoBackend(appconfig.ImageStoreModeDocker, docker, oci, WithDockerPing(func(context.Context) error { return nil }), WithDockerPingTimeout(time.Millisecond))
	if auto.Mode() != appconfig.ImageStoreModeDocker || !auto.HasDockerBackend() || !auto.HasOCIBackend() {
		t.Fatalf("auto backend metadata failed")
	}
	if _, err := auto.ListImages(ctx, ListRequest{}); err != nil || auto.LastSelection() != appconfig.ImageStoreModeDocker {
		t.Fatalf("auto docker selection err=%v selection=%q", err, auto.LastSelection())
	}
	auto = NewAutoBackend(appconfig.ImageStoreModeOCI, docker, oci)
	if _, err := auto.PullImage(ctx, PullRequest{ImageRef: "guest:latest"}); err != nil || auto.LastSelection() != appconfig.ImageStoreModeOCI {
		t.Fatalf("auto oci selection err=%v selection=%q", err, auto.LastSelection())
	}
	auto = NewAutoBackend(appconfig.ImageStoreModeAuto, docker, oci, WithDockerPing(func(context.Context) error { return errors.New("down") }))
	if _, err := auto.InspectImage(ctx, InspectRequest{ImageRef: "guest:latest"}); err != nil || auto.LastSelection() != appconfig.ImageStoreModeOCI {
		t.Fatalf("auto fallback selection err=%v selection=%q", err, auto.LastSelection())
	}
	if _, err := (*AutoBackend)(nil).RemoveImage(ctx, RemoveRequest{}); err == nil {
		t.Fatalf("expected nil auto backend error")
	}

	summary := DockerSummaryToProtoImage(typesimage.Summary{ID: "sha256:1", RepoTags: []string{"guest:latest", "<none>:<none>"}, RepoDigests: []string{"guest@sha256:1"}, Size: 42, Created: 100, Containers: -1, Labels: map[string]string{"k": "v"}}, "now", "")
	if summary.GetImageId() != "sha256:1" || summary.GetContainerCount() != 0 || summary.GetLabels()["k"] != "v" {
		t.Fatalf("docker summary = %#v", summary)
	}
	progress, err := ConsumeDockerImagePullProgress(strings.NewReader(`{"id":"layer","status":"pulling","progress":"1/2"}` + "\n"))
	if err != nil || len(progress) != 1 {
		t.Fatalf("progress=%#v err=%v", progress, err)
	}
	ociImage := OCIMetadataToProtoImage(imagecache.ImageMetadata{RequestedRef: "guest:latest", NormalizedRef: "docker.io/library/guest:latest", ManifestDigest: "sha256:manifest", ConfigDigest: "sha256:config", RepoTags: []string{"guest:latest"}, SizeBytes: 100, CreatedAt: time.Now(), PulledAt: time.Now(), Labels: map[string]string{"a": "b"}}, "")
	if ociImage.GetImageId() != "sha256:config" || ociImage.GetOci().GetManifestDigest() == "" {
		t.Fatalf("oci image = %#v", ociImage)
	}

	fakeDocker := &fakeDockerClient{}
	dockerBackend := NewDockerBackend(
		WithDockerClientFactory(func() (DockerClient, error) { return fakeDocker, nil }),
		WithDockerClock(func() time.Time { return time.Unix(123, 0).UTC() }),
	)
	list, err := dockerBackend.ListImages(ctx, ListRequest{Query: "guest", All: true})
	if err != nil || len(list.Images) != 1 || list.StoreStatus.GetEndpoint() != "unix:///docker.sock" {
		t.Fatalf("ListImages list=%#v err=%v", list, err)
	}
	pulled, err := dockerBackend.PullImage(ctx, PullRequest{ImageRef: "guest:latest", Platform: &agentcomposev2.ImagePlatform{Os: "linux", Architecture: "amd64"}})
	if err != nil || pulled.ResolvedRef == "" || len(pulled.Progress) != 1 {
		t.Fatalf("PullImage pulled=%#v err=%v", pulled, err)
	}
	inspected, err := dockerBackend.InspectImage(ctx, InspectRequest{ImageRef: "guest:latest"})
	if err != nil || inspected.Image.GetImageRef() != "guest:latest" {
		t.Fatalf("InspectImage inspected=%#v err=%v", inspected, err)
	}
	removed, err := dockerBackend.RemoveImage(ctx, RemoveRequest{ImageRef: "guest:latest", Force: true, PruneChildren: true})
	if err != nil || len(removed.DeletedIDs) != 1 || len(removed.UntaggedRefs) != 1 {
		t.Fatalf("RemoveImage removed=%#v err=%v", removed, err)
	}
	brokenDocker := NewDockerBackend(WithDockerClientFactory(func() (DockerClient, error) { return nil, errors.New("connect") }))
	if _, err := brokenDocker.ListImages(ctx, ListRequest{}); err == nil {
		t.Fatalf("expected docker connect error")
	}
	if _, err := (*DockerBackend)(nil).ListImages(ctx, ListRequest{}); err == nil {
		t.Fatalf("expected nil docker backend error")
	}

	cache, err := imagecache.New(imagecache.Config{Root: t.TempDir()})
	if err != nil {
		t.Fatalf("create OCI cache: %v", err)
	}
	now := time.Date(2026, 7, 4, 8, 0, 0, 0, time.UTC)
	ociBackend := NewOCIBackend(cache, WithOCIClock(func() time.Time { return now }))
	if !ociBackend.HasCache() || ociBackend.CacheRoot() != cache.Root() || ociBackend.inspectedAt() != now.Format(time.RFC3339Nano) {
		t.Fatalf("OCI backend metadata root=%q inspected=%q", ociBackend.CacheRoot(), ociBackend.inspectedAt())
	}
	ociList, err := ociBackend.ListImages(ctx, ListRequest{All: true})
	if err != nil || len(ociList.Images) != 0 || ociList.StoreStatus.GetStore() != agentcomposev2.ImageStoreKind_IMAGE_STORE_KIND_OCI_CACHE {
		t.Fatalf("OCI ListImages list=%#v err=%v", ociList, err)
	}
	if _, err := ociBackend.InspectImage(ctx, InspectRequest{ImageRef: "missing:latest"}); err == nil {
		t.Fatalf("expected OCI inspect missing error")
	} else if backendErr, kind, ok := ClassifyBackendError(err); !ok || kind != ErrorKindNotFound || backendErr.ImageRef != "missing:latest" || !IsNotFound(err) {
		t.Fatalf("ClassifyBackendError backendErr=%#v kind=%v ok=%v err=%v", backendErr, kind, ok, err)
	}
	if _, err := (*OCIBackend)(nil).PullImage(ctx, PullRequest{ImageRef: "guest:latest"}); err == nil {
		t.Fatalf("expected nil OCI pull error")
	} else if _, kind, ok := ClassifyBackendError(err); !ok || kind != ErrorKindUnavailable {
		t.Fatalf("OCI pull nil-cache error kind=%v ok=%v err=%v", kind, ok, err)
	}
	if _, err := (*OCIBackend)(nil).RemoveImage(ctx, RemoveRequest{ImageRef: "guest:latest"}); err == nil {
		t.Fatalf("expected nil OCI backend error")
	}
	status := ociBackend.storeStatus()
	if !status.GetAvailable() || !strings.HasSuffix(status.GetEndpoint(), "/oci") {
		t.Fatalf("OCI store status = %#v", status)
	}
	platform := ImageCachePlatform(&agentcomposev2.ImagePlatform{Os: " linux ", Architecture: " amd64 ", Variant: " v3 ", OsVersion: " 1 "})
	if platform.OS != "linux" || platform.Architecture != "amd64" || platform.Variant != "v3" || platform.OSVersion != "1" {
		t.Fatalf("ImageCachePlatform = %#v", platform)
	}
}

func TestIntegrationImageBackendAndMappingCoverageWorkflows(t *testing.T) {
	TestImageBackendAndMappingCoverageWorkflows(t)
}

func TestE2EImageBackendAndMappingCoverageWorkflows(t *testing.T) {
	TestImageBackendAndMappingCoverageWorkflows(t)
}

func TestDockerBuildContextPreservesSymlinks(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "Dockerfile"), []byte("FROM scratch\n"), 0o600); err != nil {
		t.Fatalf("write Dockerfile: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "target.txt"), []byte("target\n"), 0o600); err != nil {
		t.Fatalf("write target: %v", err)
	}
	if err := os.Symlink("target.txt", filepath.Join(dir, "link.txt")); err != nil {
		t.Fatalf("create symlink: %v", err)
	}

	entries := readTarBuildContext(t, dir, "Dockerfile", nil)
	link := entries["link.txt"]
	if link == nil {
		t.Fatalf("link.txt not found in build context: %#v", entries)
	}
	if link.Typeflag != tar.TypeSymlink || link.Linkname != "target.txt" {
		t.Fatalf("link header type/link = %v/%q", link.Typeflag, link.Linkname)
	}
}

func TestDockerBuildContextExcludesDockerignoreDirectory(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "Dockerfile"), []byte("FROM scratch\n"), 0o600); err != nil {
		t.Fatalf("write Dockerfile: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".dockerignore"), []byte("node_modules/\n"), 0o600); err != nil {
		t.Fatalf("write .dockerignore: %v", err)
	}
	nodeModules := filepath.Join(dir, "node_modules")
	if err := os.Mkdir(nodeModules, 0o700); err != nil {
		t.Fatalf("create node_modules: %v", err)
	}
	if err := os.WriteFile(filepath.Join(nodeModules, "secret.txt"), []byte("secret\n"), 0o600); err != nil {
		t.Fatalf("write ignored file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "app.txt"), []byte("app\n"), 0o600); err != nil {
		t.Fatalf("write app file: %v", err)
	}

	reader, err := dockerBuildContext(dir, "Dockerfile")
	if err != nil {
		t.Fatalf("dockerBuildContext: %v", err)
	}
	defer func() { _ = reader.Close() }()

	entries := readTarEntries(t, reader)
	if entries["node_modules"] != nil || entries["node_modules/secret.txt"] != nil {
		t.Fatalf("node_modules should be excluded, entries=%#v", entries)
	}
	if entries["app.txt"] == nil || entries["Dockerfile"] == nil || entries[".dockerignore"] == nil {
		t.Fatalf("expected app, Dockerfile, and .dockerignore entries, got %#v", entries)
	}
}

func readTarBuildContext(t *testing.T, contextDir, dockerfile string, excludes []string) map[string]*tar.Header {
	t.Helper()
	reader, err := tarBuildContext(contextDir, dockerfile, excludes)
	if err != nil {
		t.Fatalf("tarBuildContext: %v", err)
	}
	defer func() { _ = reader.Close() }()
	return readTarEntries(t, reader)
}

func readTarEntries(t *testing.T, reader io.Reader) map[string]*tar.Header {
	t.Helper()
	tarReader := tar.NewReader(reader)
	entries := map[string]*tar.Header{}
	for {
		header, err := tarReader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("read tar: %v", err)
		}
		copied := *header
		entries[header.Name] = &copied
	}
	return entries
}

type fakeImageBackend struct {
	name string
}

func (b fakeImageBackend) ListImages(context.Context, ListRequest) (ListResult, error) {
	return ListResult{Images: []*agentcomposev2.Image{{ImageId: b.name}}}, nil
}

func (b fakeImageBackend) PullImage(context.Context, PullRequest) (PullResult, error) {
	return PullResult{Image: &agentcomposev2.Image{ImageId: b.name}}, nil
}

func (b fakeImageBackend) InspectImage(context.Context, InspectRequest) (InspectResult, error) {
	return InspectResult{Image: &agentcomposev2.Image{ImageId: b.name}}, nil
}

func (b fakeImageBackend) RemoveImage(context.Context, RemoveRequest) (RemoveResult, error) {
	return RemoveResult{ImageRef: b.name}, nil
}

type fakeDockerClient struct{}

func (fakeDockerClient) ImageList(context.Context, typesimage.ListOptions) ([]typesimage.Summary, error) {
	return []typesimage.Summary{{ID: "sha256:list", RepoTags: []string{"guest:latest"}, Size: 10, Created: 123}}, nil
}

func (fakeDockerClient) ImagePull(context.Context, string, typesimage.PullOptions) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader(`{"id":"layer","status":"done"}` + "\n")), nil
}

func (fakeDockerClient) ImageBuild(context.Context, io.Reader, buildtypes.ImageBuildOptions) (buildtypes.ImageBuildResponse, error) {
	return buildtypes.ImageBuildResponse{Body: io.NopCloser(strings.NewReader(`{"stream":"build done"}` + "\n"))}, nil
}

func (fakeDockerClient) ImageInspect(context.Context, string, ...client.ImageInspectOption) (typesimage.InspectResponse, error) {
	return typesimage.InspectResponse{ID: "sha256:inspect", RepoTags: []string{"guest:latest"}, RepoDigests: []string{"guest@sha256:inspect"}, Os: "linux", Architecture: "amd64", Size: 20}, nil
}

func (fakeDockerClient) ImageRemove(context.Context, string, typesimage.RemoveOptions) ([]typesimage.DeleteResponse, error) {
	return []typesimage.DeleteResponse{{Untagged: "guest:latest"}, {Deleted: "sha256:inspect"}}, nil
}

func (fakeDockerClient) DaemonHost() string {
	return "unix:///docker.sock"
}

func (fakeDockerClient) Close() error {
	return nil
}

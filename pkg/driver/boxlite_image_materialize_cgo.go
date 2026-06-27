//go:build boxlitecgo

package driver

import (
	"context"
	"fmt"
	"log/slog"

	appconfig "agent-compose/pkg/config"
	"agent-compose/pkg/imagecache"
)

func materializeBoxliteOCIImageLayout(ctx context.Context, config *appconfig.Config, imageRef string) (boxliteImageLayoutResult, bool, error) {
	cache, err := imagecache.New(imagecache.Config{
		Root:               imageCacheRootForDriver(config),
		DefaultRegistry:    config.ImageRegistry,
		InsecureRegistries: config.ImageInsecureRegistries,
	})
	if err != nil {
		return boxliteImageLayoutResult{}, false, err
	}
	pullCtx, pullCancel := context.WithTimeout(ctx, config.ImagePullTimeout)
	_, pullErr := cache.Pull(pullCtx, imagecache.PullRequest{Reference: imageRef})
	pullCancel()
	if pullErr != nil {
		slog.Warn("agent-compose boxlite: pull guest image failed, falling back to local cache", "image", imageRef, "error", pullErr)
	}
	result, err := cache.MaterializeOCILayout(ctx, imageRef)
	if err != nil {
		if imagecache.IsKind(err, imagecache.ErrorKindNotFound) && pullErr != nil {
			return boxliteImageLayoutResult{}, false, fmt.Errorf("guest image %s not available: pull failed (%w) and not found in local cache", imageRef, pullErr)
		}
		return boxliteImageLayoutResult{}, false, err
	}
	return boxliteImageLayoutResult{
		ImageID:     result.ImageID,
		ResolvedRef: result.ResolvedRef,
		RootfsPath:  result.LayoutPath,
	}, true, nil
}

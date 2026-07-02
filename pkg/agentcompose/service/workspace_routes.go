package agentcompose

import (
	"agent-compose/pkg/agentcompose/workspaces"
	appconfig "agent-compose/pkg/config"
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/labstack/echo/v4"
)

type workspaceFilesResponse struct {
	WorkspaceID string                 `json:"workspace_id"`
	Files       []workspaces.FileEntry `json:"files"`
}

func registerWorkspaceRoutes(app *echo.Echo, service *Service) {
	base := "/api/agent-compose/workspaces"
	app.GET(base+"/:workspaceID/files", func(c echo.Context) error {
		workspace, content, err := service.loadFileWorkspaceConfig(c.Request().Context(), c.Param("workspaceID"))
		if err != nil {
			return toWorkspaceHTTPError(err)
		}
		defer func() { _ = content.Root.Close() }()
		files, err := workspaces.ListFiles(content.Root)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}
		return c.JSON(http.StatusOK, workspaceFilesResponse{WorkspaceID: workspace.ID, Files: files})
	})
	app.POST(base+"/:workspaceID/upload", func(c echo.Context) error {
		limit := service.config.WorkspaceUploadLimitBytes
		if limit <= 0 {
			limit = appconfig.DefaultWorkspaceUploadLimitBytes
		}
		if c.Request().ContentLength > limit {
			return echo.NewHTTPError(http.StatusRequestEntityTooLarge, "workspace upload exceeds configured limit")
		}
		c.Request().Body = http.MaxBytesReader(c.Response(), c.Request().Body, limit)
		_, content, err := service.loadFileWorkspaceConfig(c.Request().Context(), c.Param("workspaceID"))
		if err != nil {
			return toWorkspaceHTTPError(err)
		}
		defer func() { _ = content.Root.Close() }()
		fileHeader, err := c.FormFile("file")
		if err != nil {
			if isHTTPRequestBodyTooLarge(err) {
				return echo.NewHTTPError(http.StatusRequestEntityTooLarge, "workspace upload exceeds configured limit")
			}
			return echo.NewHTTPError(http.StatusBadRequest, "missing form file \"file\"")
		}
		uploadType := strings.ToLower(strings.TrimSpace(c.FormValue("upload_type")))
		targetPath := strings.TrimSpace(c.FormValue("path"))
		switch uploadType {
		case "", "file":
			if err := workspaces.StoreUploadedFile(fileHeader, content.Root, targetPath); err != nil {
				return toWorkspaceUploadHTTPError(err)
			}
		case "archive":
			if err := workspaces.ExtractUploadedArchive(fileHeader, content.Root); err != nil {
				return toWorkspaceUploadHTTPError(err)
			}
		default:
			return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("unsupported upload_type %q", uploadType))
		}
		files, err := workspaces.ListFiles(content.Root)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}
		return c.JSON(http.StatusOK, workspaceFilesResponse{WorkspaceID: c.Param("workspaceID"), Files: files})
	})
	app.GET(base+"/:workspaceID/download", func(c echo.Context) error {
		_, content, err := service.loadFileWorkspaceConfig(c.Request().Context(), c.Param("workspaceID"))
		if err != nil {
			return toWorkspaceHTTPError(err)
		}
		defer func() { _ = content.Root.Close() }()
		relPath, err := workspaces.CleanRelativePath(c.QueryParam("path"), false)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, err.Error())
		}
		relPath = filepath.ToSlash(relPath)
		info, err := content.Root.Lstat(relPath)
		if err != nil {
			if os.IsNotExist(err) {
				return echo.NewHTTPError(http.StatusNotFound, err.Error())
			}
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return echo.NewHTTPError(http.StatusBadRequest, "download path must not be a symlink")
		}
		if info.IsDir() {
			return echo.NewHTTPError(http.StatusBadRequest, "download path must be a file")
		}
		file, err := content.Root.Open(relPath)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}
		defer func() { _ = file.Close() }()
		c.Response().Header().Set(echo.HeaderContentDisposition, fmt.Sprintf("attachment; filename=%q", filepath.Base(relPath)))
		c.Response().Header().Set(echo.HeaderContentType, "application/octet-stream")
		return c.Stream(http.StatusOK, "application/octet-stream", file)
	})
}

func toWorkspaceUploadHTTPError(err error) error {
	if err == nil {
		return nil
	}
	if isHTTPRequestBodyTooLarge(err) {
		return echo.NewHTTPError(http.StatusRequestEntityTooLarge, "workspace upload exceeds configured limit")
	}
	return echo.NewHTTPError(http.StatusBadRequest, err.Error())
}

func isHTTPRequestBodyTooLarge(err error) bool {
	if err == nil {
		return false
	}
	var maxBytesErr *http.MaxBytesError
	if errors.As(err, &maxBytesErr) {
		return true
	}
	var httpErr *echo.HTTPError
	if errors.As(err, &httpErr) &&
		httpErr.Code == http.StatusBadRequest &&
		httpErr.Message == "http: request body too large" {
		return true
	}
	return err.Error() == "http: request body too large"
}

func (s *Service) loadFileWorkspaceConfig(ctx context.Context, workspaceID string) (WorkspaceConfig, workspaces.FileWorkspaceContent, error) {
	workspace, err := s.configDB.GetWorkspaceConfig(ctx, workspaceID)
	if err != nil {
		return WorkspaceConfig{}, workspaces.FileWorkspaceContent{}, err
	}
	if strings.ToLower(strings.TrimSpace(workspace.Type)) != "file" {
		return WorkspaceConfig{}, workspaces.FileWorkspaceContent{}, classifyError(ErrInvalidArgument, fmt.Sprintf("workspace config %s is not a file workspace", workspace.ID), nil)
	}
	content, err := workspaces.OpenFileWorkspaceContent(s.config, workspace)
	if err != nil {
		return WorkspaceConfig{}, workspaces.FileWorkspaceContent{}, err
	}
	return workspace, content, nil
}

func toWorkspaceHTTPError(err error) error {
	if err == nil {
		return nil
	}
	message := err.Error()
	switch {
	case errors.Is(err, ErrNotFound):
		return echo.NewHTTPError(http.StatusNotFound, message)
	case errors.Is(err, ErrInvalidArgument), errors.Is(err, ErrRequired):
		return echo.NewHTTPError(http.StatusBadRequest, message)
	default:
		return echo.NewHTTPError(http.StatusInternalServerError, message)
	}
}

package agentcompose

import (
	appconfig "agent-compose/pkg/config"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
)

type workspaceFileEntry struct {
	Path      string `json:"path"`
	Dir       bool   `json:"dir"`
	Size      int64  `json:"size"`
	UpdatedAt string `json:"updated_at"`
}

type workspaceFilesResponse struct {
	WorkspaceID string               `json:"workspace_id"`
	Files       []workspaceFileEntry `json:"files"`
}

func registerWorkspaceRoutes(app *echo.Echo, service *Service) {
	base := "/api/agent-compose/workspaces"
	app.GET(base+"/:workspaceID/files", func(c echo.Context) error {
		workspace, content, err := service.loadFileWorkspaceConfig(c.Request().Context(), c.Param("workspaceID"))
		if err != nil {
			return toWorkspaceHTTPError(err)
		}
		defer func() { _ = content.Root.Close() }()
		files, err := listWorkspaceFiles(content.Root)
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
			if err := storeUploadedWorkspaceFile(fileHeader, content.Root, targetPath); err != nil {
				return toWorkspaceUploadHTTPError(err)
			}
		case "archive":
			if err := extractUploadedWorkspaceArchive(fileHeader, content.Root); err != nil {
				return toWorkspaceUploadHTTPError(err)
			}
		default:
			return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("unsupported upload_type %q", uploadType))
		}
		files, err := listWorkspaceFiles(content.Root)
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
		relPath, err := cleanWorkspaceRelativePath(c.QueryParam("path"), false)
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

func (s *Service) loadFileWorkspaceConfig(ctx context.Context, workspaceID string) (WorkspaceConfig, fileWorkspaceContent, error) {
	workspace, err := s.configDB.GetWorkspaceConfig(ctx, workspaceID)
	if err != nil {
		return WorkspaceConfig{}, fileWorkspaceContent{}, err
	}
	if strings.ToLower(strings.TrimSpace(workspace.Type)) != "file" {
		return WorkspaceConfig{}, fileWorkspaceContent{}, classifyError(ErrInvalidArgument, fmt.Sprintf("workspace config %s is not a file workspace", workspace.ID), nil)
	}
	content, err := openFileWorkspaceContent(s.config, workspace)
	if err != nil {
		return WorkspaceConfig{}, fileWorkspaceContent{}, err
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
	case legacyWorkspaceHTTPStatus(message) != 0:
		return echo.NewHTTPError(legacyWorkspaceHTTPStatus(message), message)
	default:
		return echo.NewHTTPError(http.StatusInternalServerError, message)
	}
}

func legacyWorkspaceHTTPStatus(message string) int {
	switch {
	case strings.Index(message, "not found") >= 0:
		return http.StatusNotFound
	case strings.Index(message, "not a file workspace") >= 0,
		strings.Index(message, "invalid") >= 0,
		strings.Index(message, "missing") >= 0:
		return http.StatusBadRequest
	default:
		return 0
	}
}

func listWorkspaceFiles(contentRoot *os.Root) ([]workspaceFileEntry, error) {
	items := make([]workspaceFileEntry, 0)
	err := fs.WalkDir(contentRoot.FS(), ".", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == "." {
			return nil
		}
		relPath := filepath.ToSlash(path)
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("workspace file %s is a symlink", relPath)
		}
		info, err := contentRoot.Lstat(relPath)
		if err != nil {
			return err
		}
		items = append(items, workspaceFileEntry{
			Path:      filepath.ToSlash(relPath),
			Dir:       entry.IsDir(),
			Size:      info.Size(),
			UpdatedAt: info.ModTime().UTC().Format(time.RFC3339Nano),
		})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("list workspace files: %w", err)
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].Path < items[j].Path
	})
	return items, nil
}

func cleanWorkspaceRelativePath(raw string, allowEmpty bool) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		if allowEmpty {
			return "", nil
		}
		return "", fmt.Errorf("workspace path is required")
	}
	if filepath.IsAbs(trimmed) {
		return "", fmt.Errorf("workspace path %q must be relative", trimmed)
	}
	clean := filepath.Clean(trimmed)
	if clean == "." {
		if allowEmpty {
			return "", nil
		}
		return "", fmt.Errorf("workspace path is required")
	}
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("workspace path %q escapes workspace root", trimmed)
	}
	return clean, nil
}

func storeUploadedWorkspaceFile(fileHeader *multipart.FileHeader, contentRoot *os.Root, targetPath string) error {
	if targetPath == "" {
		targetPath = fileHeader.Filename
	}
	cleanTarget, err := cleanWorkspaceRelativePath(targetPath, false)
	if err != nil {
		return err
	}
	src, err := fileHeader.Open()
	if err != nil {
		return fmt.Errorf("open uploaded file: %w", err)
	}
	defer func() { _ = src.Close() }()
	cleanTarget = filepath.ToSlash(cleanTarget)
	if err := ensureRootParentDir(contentRoot, cleanTarget); err != nil {
		return fmt.Errorf("create upload target parent: %w", err)
	}
	if err := contentRoot.RemoveAll(cleanTarget); err != nil {
		return fmt.Errorf("remove upload target file: %w", err)
	}
	dst, err := contentRoot.OpenFile(cleanTarget, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("create upload target file: %w", err)
	}
	defer func() { _ = dst.Close() }()
	if _, err := io.Copy(dst, src); err != nil {
		return fmt.Errorf("write upload target file: %w", err)
	}
	return nil
}

func extractUploadedWorkspaceArchive(fileHeader *multipart.FileHeader, contentRoot *os.Root) error {
	src, err := fileHeader.Open()
	if err != nil {
		return fmt.Errorf("open uploaded archive: %w", err)
	}
	defer func() { _ = src.Close() }()
	return extractWorkspaceTarArchive(src, contentRoot)
}

func defaultFileWorkspaceConfigJSON(config *appconfig.Config, workspaceID string) string {
	root, err := defaultFileWorkspaceContentRoot(config, workspaceID)
	if err != nil {
		root = filepath.Join(config.DataRoot, "workspaces", strings.TrimSpace(workspaceID), fileWorkspaceContentDirName)
	}
	payload, _ := json.Marshal(fileWorkspaceConfig{Root: root})
	return string(payload)
}

func defaultFileWorkspaceContentRoot(config *appconfig.Config, workspaceID string) (string, error) {
	root := filepath.Join(config.DataRoot, "workspaces", strings.TrimSpace(workspaceID), fileWorkspaceContentDirName)
	return filepath.Abs(root)
}

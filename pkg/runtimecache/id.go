package runtimecache

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

const cacheIDHashLength = 16

var (
	ErrInvalidCacheID = errors.New("invalid runtime cache id")
	kindSegmentRE     = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]*$`)
	hashSegmentRE     = regexp.MustCompile(`^[a-f0-9]+$`)
)

type ParsedCacheID struct {
	Domain Domain
	Driver string
	Kind   string
	Hash   string
}

func GenerateCacheID(item Item) (string, error) {
	domain, ok := NormalizeDomain(item.Domain)
	if !ok || domain == "" {
		return "", fmt.Errorf("%w: domain is required", ErrInvalidCacheID)
	}
	driver, ok := NormalizeDriver(item.Driver)
	if !ok {
		return "", fmt.Errorf("%w: unknown driver %q", ErrInvalidCacheID, item.Driver)
	}
	if driver == "" {
		driver = DriverAll
	}
	kind := normalizeKind(item.Kind)
	if kind == "" {
		return "", fmt.Errorf("%w: kind is required", ErrInvalidCacheID)
	}
	identity := cacheIDIdentity(item)
	if identity == "" {
		return "", fmt.Errorf("%w: identity is required", ErrInvalidCacheID)
	}
	hash := cacheIDHash(domain, driver, kind, identity)
	return strings.Join([]string{string(domain), driver, kind, hash}, ":"), nil
}

func ParseCacheID(cacheID string) (ParsedCacheID, error) {
	parts := strings.Split(strings.TrimSpace(cacheID), ":")
	if len(parts) != 4 {
		return ParsedCacheID{}, fmt.Errorf("%w: expected four segments", ErrInvalidCacheID)
	}
	domain, ok := NormalizeDomain(Domain(parts[0]))
	if !ok || domain == "" {
		return ParsedCacheID{}, fmt.Errorf("%w: unknown domain %q", ErrInvalidCacheID, parts[0])
	}
	driver, ok := NormalizeDriver(parts[1])
	if !ok || driver == "" {
		return ParsedCacheID{}, fmt.Errorf("%w: unknown driver %q", ErrInvalidCacheID, parts[1])
	}
	kind := normalizeKind(parts[2])
	if kind == "" {
		return ParsedCacheID{}, fmt.Errorf("%w: invalid kind %q", ErrInvalidCacheID, parts[2])
	}
	hash := strings.ToLower(strings.TrimSpace(parts[3]))
	if len(hash) != cacheIDHashLength || !hashSegmentRE.MatchString(hash) {
		return ParsedCacheID{}, fmt.Errorf("%w: invalid hash %q", ErrInvalidCacheID, parts[3])
	}
	return ParsedCacheID{Domain: domain, Driver: driver, Kind: kind, Hash: hash}, nil
}

func cacheIDIdentity(item Item) string {
	for _, candidate := range []string{
		item.Path,
		item.ImageID,
		item.ResolvedRef,
		item.ImageRef,
		item.SessionID,
		item.SandboxID,
	} {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		if strings.Contains(candidate, string(filepath.Separator)) {
			return filepath.Clean(candidate)
		}
		return candidate
	}
	return ""
}

func cacheIDHash(domain Domain, driver, kind, identity string) string {
	sum := sha256.Sum256([]byte(strings.Join([]string{string(domain), driver, kind, identity}, "\x00")))
	return hex.EncodeToString(sum[:])[:cacheIDHashLength]
}

func normalizeKind(kind string) string {
	kind = strings.ToLower(strings.TrimSpace(kind))
	if !kindSegmentRE.MatchString(kind) {
		return ""
	}
	return kind
}

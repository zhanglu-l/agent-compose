package runs

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	"agent-compose/pkg/capabilities"
	domain "agent-compose/pkg/model"
	"agent-compose/pkg/storage/sessionstore"
)

type stickyProjectRunSandboxConfig struct {
	LoaderConfigHash string                            `json:"loader_config_hash"`
	ProjectID        string                            `json:"project_id"`
	ProjectRevision  int64                             `json:"project_revision"`
	AgentName        string                            `json:"agent_name"`
	ManagedAgentID   string                            `json:"managed_agent_id"`
	Driver           string                            `json:"driver"`
	ImageRef         string                            `json:"image_ref"`
	EnvItems         []domain.SandboxEnvVar            `json:"env_items,omitempty"`
	CapsetIDs        []string                          `json:"capset_ids,omitempty"`
	Workspace        *domain.SandboxWorkspace          `json:"workspace,omitempty"`
	Volumes          []domain.VolumeMountSpec          `json:"volumes,omitempty"`
	Jupyter          sessionstore.CreateSandboxOptions `json:"jupyter"`
}

func stickyProjectRunConfigHash(baseHash string, run domain.ProjectRunRecord, prepared Preparation, request RunAgentRequest, jupyter sessionstore.CreateSandboxOptions) (string, error) {
	baseHash = strings.TrimSpace(baseHash)
	if baseHash == "" {
		return "", nil
	}
	capsetIDs := capabilities.NormalizeCapsetIDs(prepared.CapsetIDs)
	sort.Strings(capsetIDs)
	volumeSpecs := prepared.Volumes
	if len(request.Volumes) > 0 {
		volumeSpecs = request.Volumes
	}
	volumes, err := domain.NormalizeVolumeMountSpecs(volumeSpecs)
	if err != nil {
		return "", err
	}
	sort.Slice(volumes, func(i, j int) bool { return volumes[i].Target < volumes[j].Target })
	payload, err := json.Marshal(stickyProjectRunSandboxConfig{
		LoaderConfigHash: baseHash,
		ProjectID:        run.ProjectID,
		ProjectRevision:  run.ProjectRevision,
		AgentName:        run.AgentName,
		ManagedAgentID:   run.ManagedAgentID,
		Driver:           run.Driver,
		ImageRef:         run.ImageRef,
		EnvItems:         domain.NormalizeEnvItems(prepared.EnvItems),
		CapsetIDs:        capsetIDs,
		Workspace:        prepared.Workspace,
		Volumes:          volumes,
		Jupyter:          jupyter,
	})
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(payload)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

func (c *Controller) resolveStickyLoaderBinding(ctx context.Context, store stickyBindingStore, loaderID, triggerID, configHash string) (string, *domain.LoaderBinding, []string, error) {
	binding, found, err := store.GetLoaderBinding(ctx, loaderID, triggerID)
	if err != nil {
		return "", nil, nil, fmt.Errorf("load sticky sandbox binding: %w", err)
	}
	if !found {
		return "", nil, nil, nil
	}
	if configHash == "" || binding.SandboxConfigHash == configHash {
		return binding.SandboxID, &binding, nil, nil
	}

	unlock := c.lifecycleLocks.Lock(binding.SandboxID)
	defer unlock()
	sandbox, err := c.store.GetSandbox(ctx, binding.SandboxID)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", &binding, []string{fmt.Sprintf("stale sticky sandbox %s is unavailable; creating a replacement", binding.SandboxID)}, nil
		}
		return "", &binding, nil, fmt.Errorf("load stale sticky sandbox %s: %w", binding.SandboxID, err)
	}
	if err := c.stopProjectRunSandbox(ctx, sandbox); err != nil {
		return "", &binding, nil, fmt.Errorf("retire stale sticky sandbox %s: %w", binding.SandboxID, err)
	}
	return "", &binding, []string{fmt.Sprintf("sticky sandbox %s used stale loader configuration; created a replacement", binding.SandboxID)}, nil
}

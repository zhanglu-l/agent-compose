package adapters

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"sort"
	"strings"

	"agent-compose/pkg/loaders"
	domain "agent-compose/pkg/model"
)

func (r *LoaderSandboxRunner) reuseCompatibleLoaderBinding(ctx context.Context, loader domain.Loader, triggerID, configHash string) (*domain.Sandbox, string, bool, *domain.LoaderBinding, error) {
	binding, found, err := r.ConfigDB.GetLoaderBinding(ctx, loader.Summary.ID, triggerID)
	if err != nil || !found {
		return nil, "", false, nil, err
	}
	if binding.SandboxConfigHash != configHash {
		if err := r.Shutdown(ctx, binding.SandboxID); err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, "", false, &binding, err
		}
		slog.Info("retired loader sticky sandbox with stale configuration", "loader_id", loader.Summary.ID, "trigger_id", triggerID, "sandbox_id", binding.SandboxID)
		return nil, "", false, &binding, nil
	}
	session, eventType, err := r.LoadOrResume(ctx, binding.SandboxID)
	if err == nil {
		return session, eventType, true, &binding, nil
	}
	slog.Warn("failed to reuse loader sticky sandbox, creating a new one", "loader_id", loader.Summary.ID, "sandbox_id", binding.SandboxID, "error", err)
	return nil, "", false, &binding, nil
}

func (r *LoaderSandboxRunner) bindLoaderSandbox(ctx context.Context, loader domain.Loader, triggerID, sandboxID, configHash string, expected *domain.LoaderBinding) (bool, error) {
	return r.ConfigDB.CompareAndSwapLoaderBinding(ctx, expected, domain.LoaderBinding{
		LoaderID:          loader.Summary.ID,
		TriggerID:         triggerID,
		SandboxID:         sandboxID,
		SandboxConfigHash: configHash,
	})
}

func loaderSandboxConfigHash(loader domain.Loader) (string, error) {
	return loaders.LoaderSandboxConfigHash(loader)
}

type loaderEffectiveSandboxConfig struct {
	LoaderConfigHash string                      `json:"loader_config_hash"`
	Agent            string                      `json:"agent,omitempty"`
	AgentDefinition  *loaderAgentSandboxConfig   `json:"agent_definition,omitempty"`
	SandboxPolicy    string                      `json:"sandbox_policy,omitempty"`
	PullPolicy       string                      `json:"pull_policy,omitempty"`
	JupyterEnabled   bool                        `json:"jupyter_enabled,omitempty"`
	ProviderEnvItems []domain.SandboxEnvVar      `json:"provider_env_items,omitempty"`
	EnvItems         []domain.SandboxEnvVar      `json:"env_items,omitempty"`
	Workspace        *domain.SandboxWorkspace    `json:"workspace,omitempty"`
	Driver           string                      `json:"driver"`
	GuestImage       string                      `json:"guest_image"`
	VolumeMounts     []domain.SandboxVolumeMount `json:"volume_mounts,omitempty"`
}

type loaderAgentSandboxConfig struct {
	ID                     string                   `json:"id"`
	Provider               string                   `json:"provider"`
	Model                  string                   `json:"model,omitempty"`
	SystemPrompt           string                   `json:"system_prompt,omitempty"`
	Driver                 string                   `json:"driver,omitempty"`
	GuestImage             string                   `json:"guest_image,omitempty"`
	WorkspaceID            string                   `json:"workspace_id,omitempty"`
	EnvItems               []domain.SandboxEnvVar   `json:"env_items,omitempty"`
	Volumes                []domain.VolumeMountSpec `json:"volumes,omitempty"`
	ConfigJSON             string                   `json:"config_json"`
	CapsetIDs              []string                 `json:"capset_ids,omitempty"`
	Skills                 []domain.AgentSkill      `json:"skills,omitempty"`
	ManagedProjectID       string                   `json:"managed_project_id,omitempty"`
	ManagedProjectRevision int64                    `json:"managed_project_revision,omitempty"`
	ManagedAgentName       string                   `json:"managed_agent_name,omitempty"`
}

func loaderRequestSandboxConfigHash(baseHash string, request domain.LoaderAgentRequest, agentDefinition *domain.AgentDefinition, providerEnvItems, envItems []domain.SandboxEnvVar, workspace *domain.SandboxWorkspace, driver, guestImage string, volumeMounts []domain.SandboxVolumeMount) (string, error) {
	var agentConfig *loaderAgentSandboxConfig
	if agentDefinition != nil {
		current, err := domain.NormalizeAgentDefinition(*agentDefinition, true)
		if err != nil {
			return "", err
		}
		capsetIDs := append([]string(nil), current.CapsetIDs...)
		sort.Strings(capsetIDs)
		volumes := append([]domain.VolumeMountSpec(nil), current.Volumes...)
		sort.Slice(volumes, func(i, j int) bool { return volumes[i].Target < volumes[j].Target })
		agentConfig = &loaderAgentSandboxConfig{
			ID:                     current.ID,
			Provider:               current.Provider,
			Model:                  current.Model,
			SystemPrompt:           current.SystemPrompt,
			Driver:                 current.Driver,
			GuestImage:             current.GuestImage,
			WorkspaceID:            current.WorkspaceID,
			EnvItems:               current.EnvItems,
			Volumes:                volumes,
			ConfigJSON:             current.ConfigJSON,
			CapsetIDs:              capsetIDs,
			Skills:                 current.Skills,
			ManagedProjectID:       current.ManagedProjectID,
			ManagedProjectRevision: current.ManagedProjectRevision,
			ManagedAgentName:       current.ManagedAgentName,
		}
	}
	payload, err := json.Marshal(loaderEffectiveSandboxConfig{
		LoaderConfigHash: baseHash,
		Agent:            domain.NormalizeAgentKind(request.Agent),
		AgentDefinition:  agentConfig,
		SandboxPolicy:    strings.TrimSpace(domain.LoaderAgentSandboxPolicy(request)),
		PullPolicy:       strings.TrimSpace(request.PullPolicy),
		JupyterEnabled:   request.JupyterEnabled,
		ProviderEnvItems: domain.NormalizeEnvItems(providerEnvItems),
		EnvItems:         domain.NormalizeEnvItems(envItems),
		Workspace:        workspace,
		Driver:           driver,
		GuestImage:       guestImage,
		VolumeMounts:     domain.NormalizeSandboxVolumeMounts(volumeMounts),
	})
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(payload)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

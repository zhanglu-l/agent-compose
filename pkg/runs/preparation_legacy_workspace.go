package runs

import (
	"context"
	"fmt"
	"strings"

	domain "agent-compose/pkg/model"
	"agent-compose/pkg/projects"
	agentcomposev2 "agent-compose/proto/agentcompose/v2"
)

type legacyWorkspacePreparationStore interface {
	GetWorkspaceConfig(context.Context, string) (domain.WorkspaceConfig, error)
}

// prepareLegacyFileWorkspace restores the v1 uploaded-snapshot binding only
// for the canonical synthetic project. Git and ordinary v2 workspaces remain
// on the project workspace resolver path.
func prepareLegacyFileWorkspace(ctx context.Context, store PreparationStore, project domain.ProjectRecord, spec *agentcomposev2.ProjectSpec, agentSpec *agentcomposev2.AgentSpec, agent domain.AgentDefinition) (*domain.WorkspaceConfig, *domain.SandboxWorkspace, bool, error) {
	workspaceID := strings.TrimSpace(agent.WorkspaceID)
	if workspaceID == "" || !projects.IsLegacyDefaultProject(project) {
		return nil, nil, false, nil
	}
	projectWorkspace, agentWorkspace, err := ProjectRunWorkspaceSpecsFromV2(spec.GetWorkspaces(), agentSpec.GetWorkspace())
	if err != nil {
		return nil, nil, false, err
	}
	effectiveWorkspace := projectWorkspace
	if agentWorkspace != nil {
		effectiveWorkspace = agentWorkspace
	}
	presetID, ok := projects.LegacyFileWorkspacePresetID(effectiveWorkspace)
	if !ok {
		return nil, nil, false, nil
	}
	if presetID != workspaceID {
		return nil, nil, false, fmt.Errorf("legacy file workspace binding mismatch: agent has %s, project spec has %s", workspaceID, presetID)
	}
	workspaceStore, ok := store.(legacyWorkspacePreparationStore)
	if !ok {
		return nil, nil, false, fmt.Errorf("legacy file workspace config store is required")
	}
	workspace, err := workspaceStore.GetWorkspaceConfig(ctx, workspaceID)
	if err != nil {
		return nil, nil, false, fmt.Errorf("resolve managed legacy file workspace %s: %w", workspaceID, err)
	}
	if strings.ToLower(strings.TrimSpace(workspace.Type)) != "file" {
		return nil, nil, false, fmt.Errorf("managed legacy workspace %s type is %q, want file", workspaceID, workspace.Type)
	}
	return &workspace, toSandboxWorkspaceSnapshot(workspace), true, nil
}

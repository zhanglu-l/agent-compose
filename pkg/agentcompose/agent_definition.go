package agentcompose

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"agent-compose/pkg/agentcompose/domain"
	agentcomposev1 "agent-compose/proto/agentcompose/v1"
)

const (
	defaultAgentProvider = domain.DefaultAgentProvider

	agentSessionTagSource    = domain.AgentSessionTagSource
	agentSessionTagSourceVal = domain.AgentSessionTagSourceVal
	agentSessionTagID        = domain.AgentSessionTagID
	agentSessionTagName      = domain.AgentSessionTagName
)

type (
	AgentDefinition            = domain.AgentDefinition
	AgentDefinitionListOptions = domain.AgentDefinitionListOptions
	AgentDefinitionListResult  = domain.AgentDefinitionListResult
	AgentCurrentRunSummary     = domain.AgentCurrentRunSummary
	AgentLatestRunSummary      = domain.AgentLatestRunSummary
)

type AgentValidationResult struct {
	Availability agentcomposev1.AgentAvailabilityStatus
	Health       agentcomposev1.AgentHealthStatus
	Warnings     []string
	Errors       []string
}

func normalizeAgentDefinition(item AgentDefinition, assignDefaults bool) (AgentDefinition, error) {
	return domain.NormalizeAgentDefinition(item, assignDefaults)
}

func agentDefinitionTags(agent AgentDefinition) []*agentcomposev1.SessionTag {
	return []*agentcomposev1.SessionTag{
		{Name: agentSessionTagSource, Value: agentSessionTagSourceVal},
		{Name: agentSessionTagID, Value: agent.ID},
		{Name: agentSessionTagName, Value: agent.Name},
	}
}

func sessionHasAgentTag(session *Session, agentID string) bool {
	return domain.SessionHasAgentTag(session, agentID)
}

func toProtoAgentDefinition(item AgentDefinition, workspace *WorkspaceConfig, validation AgentValidationResult, current AgentCurrentRunSummary, latest *AgentLatestRunSummary) *agentcomposev1.AgentDefinition {
	resp := &agentcomposev1.AgentDefinition{
		AgentId:            item.ID,
		Name:               item.Name,
		Description:        item.Description,
		Enabled:            item.Enabled,
		Provider:           item.Provider,
		Model:              item.Model,
		SystemPrompt:       item.SystemPrompt,
		RuntimeImageId:     "",
		Driver:             item.Driver,
		GuestImage:         item.GuestImage,
		WorkFiles:          toProtoAgentWorkFiles(item.WorkspaceID, workspace),
		EnvItems:           toProtoEnvItems(item.EnvItems),
		ConfigJson:         item.ConfigJSON,
		CapsetIds:          item.CapsetIDs,
		AvailabilityStatus: validation.Availability,
		HealthStatus:       validation.Health,
		CurrentRunSummary:  toProtoAgentCurrentRunSummary(current),
		CreatedAt:          formatProtoTime(item.CreatedAt),
		UpdatedAt:          formatProtoTime(item.UpdatedAt),
		DeletedAt:          formatProtoTime(item.DeletedAt),
	}
	if latest != nil {
		resp.LatestRunSummary = &agentcomposev1.AgentLatestRunSummary{
			RunType: latest.RunType,
			Status:  latest.Status,
			RunId:   latest.RunID,
			Title:   latest.Title,
			At:      formatProtoTime(latest.At),
		}
	}
	return resp
}

func toProtoEnvItems(items []SessionEnvVar) []*agentcomposev1.SessionEnvVar {
	resp := make([]*agentcomposev1.SessionEnvVar, 0, len(items))
	for _, item := range items {
		resp = append(resp, &agentcomposev1.SessionEnvVar{Name: item.Name, Value: item.Value, Secret: item.Secret})
	}
	return resp
}

func toProtoAgentWorkFiles(workspaceID string, workspace *WorkspaceConfig) *agentcomposev1.AgentWorkFiles {
	workspaceID = strings.TrimSpace(workspaceID)
	if workspaceID == "" || workspace == nil {
		return &agentcomposev1.AgentWorkFiles{
			Source:        agentcomposev1.AgentWorkFilesSource_AGENT_WORK_FILES_SOURCE_EMPTY,
			WorkspaceType: "empty",
		}
	}
	source := agentcomposev1.AgentWorkFilesSource_AGENT_WORK_FILES_SOURCE_UNSPECIFIED
	switch strings.ToLower(strings.TrimSpace(workspace.Type)) {
	case "file":
		source = agentcomposev1.AgentWorkFilesSource_AGENT_WORK_FILES_SOURCE_FILE_WORKSPACE
	case "git":
		source = agentcomposev1.AgentWorkFilesSource_AGENT_WORK_FILES_SOURCE_GIT_WORKSPACE
	}
	return &agentcomposev1.AgentWorkFiles{
		Source:        source,
		WorkspaceId:   workspace.ID,
		WorkspaceName: workspace.Name,
		WorkspaceType: workspace.Type,
		Summary:       agentWorkspaceSummary(*workspace),
		ConfigJson:    workspace.ConfigJSON,
	}
}

func agentWorkspaceSummary(workspace WorkspaceConfig) string {
	switch strings.ToLower(strings.TrimSpace(workspace.Type)) {
	case "git":
		var config map[string]any
		if err := json.Unmarshal([]byte(workspace.ConfigJSON), &config); err == nil {
			repo := strings.TrimSpace(fmt.Sprint(config["repo_url"]))
			if repo == "" {
				repo = strings.TrimSpace(fmt.Sprint(config["repoUrl"]))
			}
			branch := strings.TrimSpace(fmt.Sprint(config["branch"]))
			if repo != "" && branch != "" {
				return repo + "#" + branch
			}
			if repo != "" {
				return repo
			}
		}
	case "file":
		if strings.TrimSpace(workspace.Comment) != "" {
			return strings.TrimSpace(workspace.Comment)
		}
	}
	return workspace.Name
}

func toProtoAgentCurrentRunSummary(item AgentCurrentRunSummary) *agentcomposev1.AgentCurrentRunSummary {
	status := agentcomposev1.AgentCurrentRunStatus_AGENT_CURRENT_RUN_STATUS_IDLE
	text := "空闲"
	if item.RunningSessionCount > 0 {
		status = agentcomposev1.AgentCurrentRunStatus_AGENT_CURRENT_RUN_STATUS_HAS_RUNNING_SESSION
		text = "有运行中会话"
	}
	return &agentcomposev1.AgentCurrentRunSummary{
		Status:                status,
		Text:                  text,
		RunningSessionCount:   uint32(item.RunningSessionCount),
		RunningLoaderRunCount: 0,
	}
}

func formatProtoTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339)
}

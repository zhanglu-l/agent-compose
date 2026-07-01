package agentcompose

import (
	"context"

	"agent-compose/pkg/agentcompose/runs"
)

type (
	ProjectRunStartRequest      = runs.StartRequest
	ProjectRunTransitionRequest = runs.TransitionRequest
	RunCoordinator              = runs.Coordinator
)

func NewRunCoordinator(store *ConfigStore) *RunCoordinator {
	return runs.NewCoordinator(store, StableProjectRunID)
}

func projectRunStatusIsTerminal(status string) bool {
	return runs.StatusIsTerminal(status)
}

func normalizeProjectRunSource(source string) string {
	return runs.NormalizeSource(source)
}

func (s *ConfigStore) GetManagedAgentDefinition(ctx context.Context, id string) (runs.ManagedAgentDefinition, error) {
	agent, err := s.GetAgentDefinition(ctx, id)
	if err != nil {
		return runs.ManagedAgentDefinition{}, err
	}
	return runs.ManagedAgentDefinition{
		ID:               agent.ID,
		Enabled:          agent.Enabled,
		DeletedAt:        agent.DeletedAt,
		Driver:           agent.Driver,
		GuestImage:       agent.GuestImage,
		ManagedProjectID: agent.ManagedProjectID,
		ManagedAgentName: agent.ManagedAgentName,
	}, nil
}

package agentcompose

import (
	"context"

	"agent-compose/pkg/agentcompose/events"
	"agent-compose/pkg/agentcompose/loaders"
)

func NewEventDispatcher(rootCtx context.Context, configDB *ConfigStore, bus *loaders.Bus) *events.Dispatcher {
	return events.NewDispatcher(rootCtx, configDB, bus)
}

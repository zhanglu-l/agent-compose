package agentcompose

import (
	"context"

	"agent-compose/pkg/agentcompose/events"
)

type EventDispatcher = events.Dispatcher

func NewEventDispatcher(rootCtx context.Context, configDB *ConfigStore, bus *LoaderBus) *EventDispatcher {
	return events.NewDispatcher(rootCtx, configDB, bus)
}

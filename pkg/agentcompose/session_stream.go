package agentcompose

import (
	"agent-compose/pkg/agentcompose/sessions"

	"github.com/samber/do/v2"
)

type (
	SessionStreamBroker = sessions.StreamBroker
)

func NewSessionStreamBroker(di do.Injector) (*SessionStreamBroker, error) {
	return sessions.NewStreamBroker(di)
}

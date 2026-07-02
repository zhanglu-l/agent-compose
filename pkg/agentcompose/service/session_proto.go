package agentcompose

import (
	"time"

	"agent-compose/pkg/agentcompose/api"
	agentcomposev1 "agent-compose/proto/agentcompose/v1"
)

func sessionListOptionsFromProto(req *agentcomposev1.ListSessionsRequest) (SessionListOptions, error) {
	return api.SessionListOptionsFromProto(req)
}

func parseOptionalRFC3339(raw, field string) (time.Time, error) {
	return api.ParseOptionalRFC3339(raw, field)
}

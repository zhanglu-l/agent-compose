package api

import (
	"fmt"
	"strings"
	"time"

	"agent-compose/pkg/agentcompose/domain"
	agentcomposev1 "agent-compose/proto/agentcompose/v1"
)

func SessionListOptionsFromProto(req *agentcomposev1.ListSessionsRequest) (domain.SessionListOptions, error) {
	if req == nil {
		return domain.SessionListOptions{}, nil
	}
	createdFrom, err := ParseOptionalRFC3339(req.GetCreatedFrom(), "created_from")
	if err != nil {
		return domain.SessionListOptions{}, err
	}
	createdTo, err := ParseOptionalRFC3339(req.GetCreatedTo(), "created_to")
	if err != nil {
		return domain.SessionListOptions{}, err
	}
	updatedFrom, err := ParseOptionalRFC3339(req.GetUpdatedFrom(), "updated_from")
	if err != nil {
		return domain.SessionListOptions{}, err
	}
	updatedTo, err := ParseOptionalRFC3339(req.GetUpdatedTo(), "updated_to")
	if err != nil {
		return domain.SessionListOptions{}, err
	}
	return domain.SessionListOptions{
		SessionType:        req.GetSessionType(),
		TriggerSourceQuery: req.GetTriggerSourceQuery(),
		TitleQuery:         req.GetTitleQuery(),
		WorkspaceQuery:     req.GetWorkspaceQuery(),
		Driver:             req.GetDriver(),
		VMStatus:           req.GetVmStatus(),
		CreatedFrom:        createdFrom,
		CreatedTo:          createdTo,
		UpdatedFrom:        updatedFrom,
		UpdatedTo:          updatedTo,
		Offset:             int(req.GetOffset()),
		Limit:              int(req.GetLimit()),
	}, nil
}

func ParseOptionalRFC3339(raw, field string) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, nil
	}
	value, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid %s: %w", field, err)
	}
	return value.UTC(), nil
}

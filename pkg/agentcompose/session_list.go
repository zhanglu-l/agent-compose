package agentcompose

import "agent-compose/pkg/agentcompose/domain"

const defaultSessionListLimit = domain.DefaultSessionListLimit

func normalizeSessionTriggerSource(value string, tags []SessionTag) string {
	return domain.NormalizeSessionTriggerSource(value, tags)
}

func sessionTypeFromTriggerSource(value string) string {
	return domain.SessionTypeFromTriggerSource(value)
}

func normalizeSessionListBounds(offset, limit int) (int, int) {
	return domain.NormalizeSessionListBounds(offset, limit)
}

func paginateSessions(items []*Session, offset, limit int) []*Session {
	return domain.PaginateSessions(items, offset, limit)
}

func sessionMatchesListOptions(session *Session, options SessionListOptions) bool {
	return domain.SessionMatchesListOptions(session, options)
}

package projects

import (
	"agent-compose/pkg/agentcompose/capabilities"
	"agent-compose/pkg/agentcompose/domain"
)

func SameSessionEnvItems(a, b []domain.SessionEnvVar) bool {
	a = domain.NormalizeEnvItems(a)
	b = domain.NormalizeEnvItems(b)
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func SameCapsetIDs(a, b []string) bool {
	a = capabilities.NormalizeCapsetIDs(a)
	b = capabilities.NormalizeCapsetIDs(b)
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

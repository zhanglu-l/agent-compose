package projects

import (
	"slices"
	"strings"

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

func SameLoaderTriggerSpecs(a, b []domain.LoaderTrigger) bool {
	a = NormalizeComparableLoaderTriggers(a)
	b = NormalizeComparableLoaderTriggers(b)
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].ID != b[i].ID ||
			a[i].Kind != b[i].Kind ||
			a[i].Topic != b[i].Topic ||
			a[i].IntervalMs != b[i].IntervalMs ||
			a[i].AutoID != b[i].AutoID ||
			a[i].SpecJSON != b[i].SpecJSON {
			return false
		}
	}
	return true
}

func NormalizeComparableLoaderTriggers(items []domain.LoaderTrigger) []domain.LoaderTrigger {
	cloned := append([]domain.LoaderTrigger(nil), items...)
	for i := range cloned {
		cloned[i].ID = strings.TrimSpace(cloned[i].ID)
		cloned[i].Kind = strings.TrimSpace(cloned[i].Kind)
		cloned[i].Topic = strings.TrimSpace(cloned[i].Topic)
		cloned[i].SpecJSON = strings.TrimSpace(cloned[i].SpecJSON)
	}
	slices.SortFunc(cloned, func(a, b domain.LoaderTrigger) int {
		if a.Kind != b.Kind {
			return strings.Compare(a.Kind, b.Kind)
		}
		return strings.Compare(a.ID, b.ID)
	})
	return cloned
}

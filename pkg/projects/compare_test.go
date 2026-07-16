package projects

import (
	"testing"

	domain "agent-compose/pkg/model"
)

func TestManagedAgentDefinitionChangeActionComparesEnabledState(t *testing.T) {
	tests := []struct {
		name     string
		existing bool
		current  bool
		want     string
	}{
		{name: "enabled remains enabled", existing: true, current: true, want: ChangeActionUnchanged},
		{name: "disabled remains disabled", existing: false, current: false, want: ChangeActionUnchanged},
		{name: "enabled becomes disabled", existing: true, current: false, want: ChangeActionUpdated},
		{name: "disabled becomes enabled", existing: false, current: true, want: ChangeActionUpdated},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			existing := domain.AgentDefinition{ID: "agent-1", Name: "worker", Enabled: tt.existing}
			current := existing
			current.Enabled = tt.current
			if got := ManagedAgentDefinitionChangeAction(existing, true, current); got != tt.want {
				t.Fatalf("ManagedAgentDefinitionChangeAction() = %q, want %q", got, tt.want)
			}
		})
	}
}

package runs

import (
	"testing"

	domain "agent-compose/pkg/model"
	"agent-compose/pkg/storage/sessionstore"
)

func TestStickyProjectRunConfigHashTracksEffectiveSandboxSpec(t *testing.T) {
	run := domain.ProjectRunRecord{
		ProjectID:       "project-1",
		ProjectRevision: 1,
		AgentName:       "worker",
		ManagedAgentID:  "agent-1",
		Driver:          "docker",
		ImageRef:        "guest:v1",
	}
	prepared := Preparation{
		EnvItems:  []domain.SandboxEnvVar{{Name: "BUG_VALUE", Value: "A"}},
		CapsetIDs: []string{"a", "b"},
		Workspace: &domain.SandboxWorkspace{ID: "workspace-1", ConfigJSON: `{"root":"v1"}`},
	}
	baseHash := "sha256:loader"
	first, err := stickyProjectRunConfigHash(baseHash, run, prepared, RunAgentRequest{}, sessionstore.CreateSandboxOptions{})
	if err != nil {
		t.Fatalf("stickyProjectRunConfigHash returned error: %v", err)
	}

	reordered := prepared
	reordered.CapsetIDs = []string{"b", "a", "a"}
	same, err := stickyProjectRunConfigHash(baseHash, run, reordered, RunAgentRequest{}, sessionstore.CreateSandboxOptions{})
	if err != nil {
		t.Fatalf("stickyProjectRunConfigHash reordered returned error: %v", err)
	}
	if same != first {
		t.Fatalf("capset ordering changed effective hash: got %q want %q", same, first)
	}

	changed := prepared
	changed.EnvItems = []domain.SandboxEnvVar{{Name: "BUG_VALUE", Value: "B"}}
	second, err := stickyProjectRunConfigHash(baseHash, run, changed, RunAgentRequest{}, sessionstore.CreateSandboxOptions{})
	if err != nil {
		t.Fatalf("stickyProjectRunConfigHash changed returned error: %v", err)
	}
	if second == first {
		t.Fatal("effective environment change did not change sticky project sandbox hash")
	}
}

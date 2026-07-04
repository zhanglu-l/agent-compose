package projects

import (
	"encoding/json"
	"testing"

	"agent-compose/pkg/compose"
	domain "agent-compose/pkg/model"
)

func TestNewAgentDefinitionFromSpecPreservesJupyterConfig(t *testing.T) {
	project := domain.ProjectRecord{ID: "project-1", Name: "project"}
	agent := compose.NormalizedAgentSpec{
		Name:     "reviewer",
		Provider: "codex",
		Jupyter:  &compose.JupyterSpec{Enabled: true, GuestPort: 8888},
	}

	definition, err := NewAgentDefinitionFromSpec(project, 1, agent)
	if err != nil {
		t.Fatalf("NewAgentDefinitionFromSpec returned error: %v", err)
	}
	var config struct {
		Jupyter *compose.JupyterSpec `json:"jupyter"`
	}
	if err := json.Unmarshal([]byte(definition.ConfigJSON), &config); err != nil {
		t.Fatalf("unmarshal config json: %v", err)
	}
	if config.Jupyter == nil || !config.Jupyter.Enabled || config.Jupyter.GuestPort != 8888 {
		t.Fatalf("config json = %s, want jupyter enabled guest port 8888", definition.ConfigJSON)
	}
}

func TestNewAgentDefinitionFromSpecKeepsEmptyConfigWithoutJupyter(t *testing.T) {
	project := domain.ProjectRecord{ID: "project-1", Name: "project"}
	agent := compose.NormalizedAgentSpec{Name: "reviewer", Provider: "codex"}

	definition, err := NewAgentDefinitionFromSpec(project, 1, agent)
	if err != nil {
		t.Fatalf("NewAgentDefinitionFromSpec returned error: %v", err)
	}
	if definition.ConfigJSON != "{}" {
		t.Fatalf("config json = %s, want empty object", definition.ConfigJSON)
	}
}

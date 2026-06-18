package agentcompose

import (
	appconfig "agent-compose/pkg/config"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveAgentSystemPromptReturnsEmptyWhenAgentPromptUnset(t *testing.T) {
	ctx := context.Background()
	configDB := newTestConfigStore(t)
	created, err := configDB.CreateAgentDefinition(ctx, AgentDefinition{
		ID:       "agent-empty-prompt",
		Name:     "Empty Prompt",
		Provider: "codex",
		Enabled:  true,
	})
	if err != nil {
		t.Fatalf("CreateAgentDefinition returned error: %v", err)
	}
	executor := &Executor{configDB: configDB}
	session := &Session{
		Summary: SessionSummary{
			Tags: []SessionTag{
				{Name: agentSessionTagSource, Value: agentSessionTagSourceVal},
				{Name: agentSessionTagID, Value: created.ID},
			},
		},
	}

	got, err := executor.resolveAgentSystemPrompt(ctx, session, "")
	if err != nil {
		t.Fatalf("resolveAgentSystemPrompt returned error: %v", err)
	}
	if got != "" {
		t.Fatalf("resolveAgentSystemPrompt = %q, want empty", got)
	}
}

func TestResolveAgentSystemPromptFromProviderAgentID(t *testing.T) {
	ctx := context.Background()
	configDB := newTestConfigStore(t)
	created, err := configDB.CreateAgentDefinition(ctx, AgentDefinition{
		ID:           "agent-explicit-id",
		Name:         "Explicit",
		Provider:     "codex",
		SystemPrompt: "Use explicit agent id",
		Enabled:      true,
	})
	if err != nil {
		t.Fatalf("CreateAgentDefinition returned error: %v", err)
	}
	executor := &Executor{configDB: configDB}
	session := &Session{Summary: SessionSummary{Tags: []SessionTag{}}}

	got, err := executor.resolveAgentSystemPrompt(ctx, session, created.ID)
	if err != nil {
		t.Fatalf("resolveAgentSystemPrompt returned error: %v", err)
	}
	if got != "Use explicit agent id" {
		t.Fatalf("resolveAgentSystemPrompt = %q, want %q", got, "Use explicit agent id")
	}
}

func TestResolveAgentSystemPromptPrefersProviderAgentOverSessionTag(t *testing.T) {
	ctx := context.Background()
	configDB := newTestConfigStore(t)
	tagged, err := configDB.CreateAgentDefinition(ctx, AgentDefinition{
		ID:           "agent-tagged",
		Name:         "Tagged",
		Provider:     "codex",
		SystemPrompt: "From session tag",
		Enabled:      true,
	})
	if err != nil {
		t.Fatalf("CreateAgentDefinition returned error: %v", err)
	}
	explicit, err := configDB.CreateAgentDefinition(ctx, AgentDefinition{
		ID:           "agent-explicit",
		Name:         "Explicit",
		Provider:     "codex",
		SystemPrompt: "From provider agent id",
		Enabled:      true,
	})
	if err != nil {
		t.Fatalf("CreateAgentDefinition returned error: %v", err)
	}
	executor := &Executor{configDB: configDB}
	session := &Session{
		Summary: SessionSummary{
			Tags: []SessionTag{
				{Name: agentSessionTagSource, Value: agentSessionTagSourceVal},
				{Name: agentSessionTagID, Value: tagged.ID},
			},
		},
	}

	got, err := executor.resolveAgentSystemPrompt(ctx, session, explicit.ID)
	if err != nil {
		t.Fatalf("resolveAgentSystemPrompt returned error: %v", err)
	}
	if got != "From provider agent id" {
		t.Fatalf("resolveAgentSystemPrompt = %q, want %q", got, "From provider agent id")
	}
}

func TestResolveAgentSystemPromptReturnsEmptyWhenAgentMissing(t *testing.T) {
	ctx := context.Background()
	executor := &Executor{configDB: newTestConfigStore(t)}
	session := &Session{Summary: SessionSummary{Tags: []SessionTag{}}}

	got, err := executor.resolveAgentSystemPrompt(ctx, session, "missing-agent-id")
	if err != nil {
		t.Fatalf("resolveAgentSystemPrompt returned error: %v", err)
	}
	if got != "" {
		t.Fatalf("resolveAgentSystemPrompt = %q, want empty", got)
	}
}

func TestResolveAgentSystemPromptFromSessionTags(t *testing.T) {
	ctx := context.Background()
	configDB := newTestConfigStore(t)
	created, err := configDB.CreateAgentDefinition(ctx, AgentDefinition{
		ID:           "agent-with-prompt",
		Name:         "Runner",
		Provider:     "codex",
		SystemPrompt: "  Reply only in Chinese  ",
		Enabled:      true,
	})
	if err != nil {
		t.Fatalf("CreateAgentDefinition returned error: %v", err)
	}
	executor := &Executor{configDB: configDB}
	session := &Session{
		Summary: SessionSummary{
			Tags: []SessionTag{
				{Name: agentSessionTagSource, Value: agentSessionTagSourceVal},
				{Name: agentSessionTagID, Value: created.ID},
			},
		},
	}

	got, err := executor.resolveAgentSystemPrompt(ctx, session, "")
	if err != nil {
		t.Fatalf("resolveAgentSystemPrompt returned error: %v", err)
	}
	if got != "Reply only in Chinese" {
		t.Fatalf("resolveAgentSystemPrompt = %q, want %q", got, "Reply only in Chinese")
	}
}

func TestWriteAgentSystemPromptFileWritesGuestPath(t *testing.T) {
	root := t.TempDir()
	cfg := &appconfig.Config{GuestStateRoot: "/data/state"}
	session := &Session{Summary: SessionSummary{WorkspacePath: filepath.Join(root, "workspace")}}

	guestPath, err := writeAgentSystemPromptFile(cfg, session, "Codex", "Reply only in Chinese")
	if err != nil {
		t.Fatalf("writeAgentSystemPromptFile returned error: %v", err)
	}
	if guestPath == "" || !strings.HasPrefix(guestPath, "/data/state/agents/system-prompts/codex-") || !strings.HasSuffix(guestPath, ".txt") {
		t.Fatalf("guestPath = %q", guestPath)
	}
	fileName := filepath.Base(guestPath)
	hostPath := filepath.Join(root, "state", "agents", "system-prompts", fileName)
	content, err := os.ReadFile(hostPath)
	if err != nil {
		t.Fatalf("ReadFile(%s) returned error: %v", hostPath, err)
	}
	if string(content) != "Reply only in Chinese" {
		t.Fatalf("file content = %q", string(content))
	}
}

func TestWriteAgentSystemPromptFileSkipsEmptyPrompt(t *testing.T) {
	root := t.TempDir()
	cfg := &appconfig.Config{GuestStateRoot: "/data/state"}
	session := &Session{Summary: SessionSummary{WorkspacePath: filepath.Join(root, "workspace")}}

	guestPath, err := writeAgentSystemPromptFile(cfg, session, "codex", "   ")
	if err != nil {
		t.Fatalf("writeAgentSystemPromptFile returned error: %v", err)
	}
	if guestPath != "" {
		t.Fatalf("guestPath = %q, want empty", guestPath)
	}
}

func TestBuildAgentExecSpecIncludesSystemPromptFlag(t *testing.T) {
	cfg := &appconfig.Config{
		GuestStateRoot:     "/data/state",
		GuestWorkspacePath: "/workspace",
	}
	session := &Session{Summary: SessionSummary{ID: "session-1"}}

	spec := buildAgentExecSpec(cfg, session, "codex", "/data/state/agents/prompts/prompt.txt", "", "/data/state/agents/system-prompts/sys.txt")
	command := strings.Join(spec.Args, " ")
	if !strings.Contains(command, "--system-prompt-file '/data/state/agents/system-prompts/sys.txt'") {
		t.Fatalf("command missing --system-prompt-file: %q", command)
	}
}

func TestIsUnknownSystemPromptFileFlag(t *testing.T) {
	t.Parallel()
	const flag = "--system-prompt-file"
	tests := []struct {
		name   string
		err    error
		result ExecResult
		want   bool
	}{
		{
			name: "error mentions unknown option",
			err:  fmt.Errorf("unknown option '%s'", flag),
			want: true,
		},
		{
			name: "stderr mentions unknown flag",
			result: ExecResult{
				Stderr: "error: unknown flag " + flag,
			},
			want: true,
		},
		{
			name: "stderr mentions flag without unknown option wording",
			result: ExecResult{
				Stderr: flag + " is not supported",
			},
			want: false,
		},
		{
			name: "unrelated error",
			err:  fmt.Errorf("session is not running"),
			want: false,
		},
		{
			name: "empty result",
			want: false,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := isUnknownSystemPromptFileFlag(tt.err, tt.result); got != tt.want {
				t.Fatalf("isUnknownSystemPromptFileFlag() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuildAgentExecSpecOmitsSystemPromptFlagWhenEmpty(t *testing.T) {
	cfg := &appconfig.Config{
		GuestStateRoot:     "/data/state",
		GuestWorkspacePath: "/workspace",
	}
	session := &Session{Summary: SessionSummary{ID: "session-1"}}

	spec := buildAgentExecSpec(cfg, session, "codex", "/data/state/agents/prompts/prompt.txt", "", "")
	command := strings.Join(spec.Args, " ")
	if strings.Contains(command, "--system-prompt-file") {
		t.Fatalf("command contains unexpected --system-prompt-file: %q", command)
	}
}

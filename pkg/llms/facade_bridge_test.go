package llms

import (
	"encoding/json"
	"testing"

	protocolbridge "github.com/chaitin/ai-api-protocol-bridge"
)

func TestRewriteRuntimeRequestForUpstreamPreservesAssistantTextAfterToolCall(t *testing.T) {
	body := []byte(`{
		"input":[
			{"type":"message","role":"user","content":[{"type":"input_text","text":"Call the demo tool."}]},
			{"type":"message","role":"assistant","content":[{"type":"output_text","text":"I'll call the demo tool."}]},
			{"type":"function_call","name":"search_demo","arguments":"{\"query\":\"demo\"}","call_id":"call_1"},
			{"type":"function_call_output","call_id":"call_1","output":"mcp-tool-ok"}
		]
	}`)

	rewritten, err := RewriteRuntimeRequestForUpstream(body, ResolvedTarget{}, protocolbridge.ProtocolOpenAIResponses)
	if err != nil {
		t.Fatalf("RewriteRuntimeRequestForUpstream() error = %v", err)
	}

	var payload struct {
		Input []struct {
			Type    string `json:"type"`
			Role    string `json:"role"`
			Content []struct {
				Type string `json:"type"`
			} `json:"content"`
		} `json:"input"`
	}
	if err := json.Unmarshal(rewritten, &payload); err != nil {
		t.Fatalf("unmarshal rewritten request: %v", err)
	}
	if got := payload.Input[1].Content[0].Type; got != "output_text" {
		t.Fatalf("assistant preamble text type = %q, want output_text", got)
	}
	if got := payload.Input[0].Content[0].Type; got != "input_text" {
		t.Fatalf("user text type = %q, want input_text", got)
	}
	if got := payload.Input[2].Type; got != "function_call" {
		t.Fatalf("tool call item type = %q, want function_call", got)
	}
	if got := payload.Input[3].Type; got != "function_call_output" {
		t.Fatalf("tool output item type = %q, want function_call_output", got)
	}
}

func TestRewriteRuntimeRequestForUpstreamNormalizesResponsesTextTypesByRole(t *testing.T) {
	tests := []struct {
		name   string
		target ResolvedTarget
		want   []string
	}{
		{
			name: "standard responses",
			want: []string{"input_text", "input_text", "output_text"},
		},
		{
			name: "generic provider",
			target: ResolvedTarget{Provider: Provider{
				UseGenericResponsesTextParts: true,
			}},
			want: []string{"text", "text", "text"},
		},
	}

	body := []byte(`{
		"input":[
			{"role":"developer","content":[{"text":"Follow the instructions."}]},
			{"role":"user","content":[{"type":"output_text","text":"Question"}]},
			{"role":"assistant","content":[{"type":"input_text","text":"Answer"}]}
		]
	}`)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rewritten, err := RewriteRuntimeRequestForUpstream(body, tt.target, protocolbridge.ProtocolOpenAIResponses)
			if err != nil {
				t.Fatalf("RewriteRuntimeRequestForUpstream() error = %v", err)
			}

			var payload struct {
				Input []struct {
					Content []struct {
						Type string `json:"type"`
					} `json:"content"`
				} `json:"input"`
			}
			if err := json.Unmarshal(rewritten, &payload); err != nil {
				t.Fatalf("unmarshal rewritten request: %v", err)
			}
			for i, want := range tt.want {
				if got := payload.Input[i].Content[0].Type; got != want {
					t.Errorf("input[%d] text type = %q, want %q", i, got, want)
				}
			}
		})
	}
}

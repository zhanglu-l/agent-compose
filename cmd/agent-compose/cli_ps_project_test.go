package main

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"connectrpc.com/connect"

	agentcomposev2 "agent-compose/proto/agentcompose/v2"
)

func TestIntegrationCLIPSSelectsStoredProjectByNameWithoutComposeFile(t *testing.T) {
	for _, command := range [][]string{{"ps"}, {"sandbox", "ls"}} {
		t.Run(strings.Join(command, " "), func(t *testing.T) {
			withWorkingDir(t, t.TempDir())
			server := newComposeServiceStubServer(t, composeServiceStubs{
				project: projectServiceStub{getProject: func(_ context.Context, req *connect.Request[agentcomposev2.GetProjectRequest]) (*connect.Response[agentcomposev2.GetProjectResponse], error) {
					if req.Msg.GetProject().GetName() != "legacy-v1-default" || req.Msg.GetProject().GetProjectId() != "" {
						t.Fatalf("project ref = %#v, want name-only legacy-v1-default", req.Msg.GetProject())
					}
					return connect.NewResponse(&agentcomposev2.GetProjectResponse{Project: testCLIProject("legacy-project-id", "legacy-v1-default", "")}), nil
				}},
				run: runServiceStub{listRuns: func(context.Context, *connect.Request[agentcomposev2.ListRunsRequest]) (*connect.Response[agentcomposev2.ListRunsResponse], error) {
					return connect.NewResponse(&agentcomposev2.ListRunsResponse{}), nil
				}},
				sandbox: sandboxServiceStub{listSandboxes: func(context.Context, *connect.Request[agentcomposev2.ListSandboxesRequest]) (*connect.Response[agentcomposev2.ListSandboxesResponse], error) {
					return connect.NewResponse(&agentcomposev2.ListSandboxesResponse{}), nil
				}},
			})
			t.Cleanup(server.Close)

			args := append(append([]string(nil), command...), "--project-name", "legacy-v1-default", "--host", server.URL)
			stdout, stderr, _, exitCode := executeCLICommand(args...)
			if exitCode != 0 || stderr != "" {
				t.Fatalf("%s code/stdout/stderr = %d / %q / %q", strings.Join(command, " "), exitCode, stdout, stderr)
			}
			if !strings.Contains(stdout, "SANDBOX ID") {
				t.Fatalf("%s output = %q, want sandbox table", strings.Join(command, " "), stdout)
			}
		})
	}
}

func TestResolveComposePSProjectKeepsComposeAndExplicitFileSemantics(t *testing.T) {
	t.Run("existing default compose keeps derived id", func(t *testing.T) {
		dir := t.TempDir()
		writeComposeFile(t, dir, "name: original\nagents: {}\n")
		withWorkingDir(t, dir)

		selection, err := resolveComposePSProject(cliOptions{ProjectName: "override"})
		if err != nil {
			t.Fatalf("resolve compose ps project: %v", err)
		}
		if selection.projectRef.GetProjectId() == "" || selection.projectRef.GetName() != "" || selection.projectName != "override" || selection.composePath == "" {
			t.Fatalf("selection = %#v, ref=%#v", selection, selection.projectRef)
		}
	})

	t.Run("explicit missing file does not fall back", func(t *testing.T) {
		missing := filepath.Join(t.TempDir(), "missing.yml")
		_, err := resolveComposePSProject(cliOptions{ComposeFile: missing, ProjectName: "stored-project"})
		if err == nil || !strings.Contains(err.Error(), "no such file") {
			t.Fatalf("resolve explicit missing file error = %v", err)
		}
	})
}

func TestIntegrationCLIPSNameSelectionClassifiesLookupErrors(t *testing.T) {
	tests := []struct {
		name string
		code connect.Code
		want string
	}{
		{name: "not found", code: connect.CodeNotFound, want: `project "missing" was not found on this daemon`},
		{name: "ambiguous", code: connect.CodeInvalidArgument, want: "ambiguous"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			withWorkingDir(t, t.TempDir())
			server := newComposeServiceStubServer(t, composeServiceStubs{project: projectServiceStub{getProject: func(context.Context, *connect.Request[agentcomposev2.GetProjectRequest]) (*connect.Response[agentcomposev2.GetProjectResponse], error) {
				return nil, connect.NewError(tt.code, errors.New(tt.want))
			}}})
			t.Cleanup(server.Close)

			_, stderr, _, exitCode := executeCLICommand("ps", "--project-name", "missing", "--host", server.URL)
			if exitCode != exitCodeUsage || !strings.Contains(stderr, tt.want) {
				t.Fatalf("ps code/stderr = %d / %q, want usage containing %q", exitCode, stderr, tt.want)
			}
		})
	}
}

package agentcompose

import (
	"context"
	"os"
	"strings"
	"testing"

	"connectrpc.com/connect"

	"agent-compose/pkg/agentcompose/domain"
	driverpkg "agent-compose/pkg/driver"
	agentcomposev1 "agent-compose/proto/agentcompose/v1"
	agentcomposev2 "agent-compose/proto/agentcompose/v2"
)

func TestServiceRemoveSandboxDeletesStoppedSession(t *testing.T) {
	ctx := context.Background()
	service, _, driver := newTestServiceAPIHarness(t)
	sessionID := createSandboxServiceTestSession(t, ctx, service)
	if _, err := service.StopSession(ctx, connect.NewRequest(&agentcomposev1.SessionIDRequest{SessionId: sessionID})); err != nil {
		t.Fatalf("StopSession returned error: %v", err)
	}
	sessionDir := service.store.sessionDir(sessionID)

	resp, err := service.RemoveSandbox(ctx, connect.NewRequest(&agentcomposev2.RemoveSandboxRequest{SandboxId: sessionID}))
	if err != nil {
		t.Fatalf("RemoveSandbox returned error: %v", err)
	}
	if resp.Msg.GetSandboxId() != sessionID || resp.Msg.GetStopped() || !resp.Msg.GetRemoved() {
		t.Fatalf("RemoveSandbox response = %#v", resp.Msg)
	}
	if _, err := os.Stat(sessionDir); !os.IsNotExist(err) {
		t.Fatalf("session dir stat err = %v, want not exist", err)
	}
	if len(driver.stopCalls) != 1 {
		t.Fatalf("driver stop calls = %v, want one call from explicit StopSession only", driver.stopCalls)
	}
}

func TestServiceRemoveSandboxRejectsRunningWithoutForce(t *testing.T) {
	ctx := context.Background()
	service, _, driver := newTestServiceAPIHarness(t)
	sessionID := createSandboxServiceTestSession(t, ctx, service)
	sessionDir := service.store.sessionDir(sessionID)

	_, err := service.RemoveSandbox(ctx, connect.NewRequest(&agentcomposev2.RemoveSandboxRequest{SandboxId: sessionID}))
	if connect.CodeOf(err) != connect.CodeFailedPrecondition || !strings.Contains(err.Error(), "is running") {
		t.Fatalf("RemoveSandbox running error = %v, want failed precondition is running", err)
	}
	if _, statErr := os.Stat(sessionDir); statErr != nil {
		t.Fatalf("session dir stat err = %v, want preserved", statErr)
	}
	if len(driver.stopCalls) != 0 {
		t.Fatalf("driver stop calls = %v, want none", driver.stopCalls)
	}
}

func TestServiceRemoveSandboxForceStopsThenDeletes(t *testing.T) {
	ctx := context.Background()
	service, _, driver := newTestServiceAPIHarness(t)
	sessionID := createSandboxServiceTestSession(t, ctx, service)
	sessionDir := service.store.sessionDir(sessionID)

	resp, err := service.RemoveSandbox(ctx, connect.NewRequest(&agentcomposev2.RemoveSandboxRequest{SandboxId: sessionID, Force: true}))
	if err != nil {
		t.Fatalf("RemoveSandbox --force returned error: %v", err)
	}
	if resp.Msg.GetSandboxId() != sessionID || !resp.Msg.GetStopped() || !resp.Msg.GetRemoved() {
		t.Fatalf("RemoveSandbox --force response = %#v", resp.Msg)
	}
	if _, err := os.Stat(sessionDir); !os.IsNotExist(err) {
		t.Fatalf("session dir stat err = %v, want not exist", err)
	}
	if len(driver.stopCalls) != 1 || driver.stopCalls[0] != sessionID {
		t.Fatalf("driver stop calls = %v, want [%s]", driver.stopCalls, sessionID)
	}
}

func TestServiceRemoveSandboxValidationAndMissing(t *testing.T) {
	ctx := context.Background()
	service, _, _ := newTestServiceAPIHarness(t)

	_, err := service.RemoveSandbox(ctx, connect.NewRequest(&agentcomposev2.RemoveSandboxRequest{SandboxId: " "}))
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("RemoveSandbox empty code = %v, want invalid argument; err=%v", connect.CodeOf(err), err)
	}
	_, err = service.RemoveSandbox(ctx, connect.NewRequest(&agentcomposev2.RemoveSandboxRequest{SandboxId: "../outside"}))
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("RemoveSandbox unsafe id code = %v, want invalid argument; err=%v", connect.CodeOf(err), err)
	}
	_, err = service.RemoveSandbox(ctx, connect.NewRequest(&agentcomposev2.RemoveSandboxRequest{SandboxId: "missing-sandbox"}))
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("RemoveSandbox missing code = %v, want not found; err=%v", connect.CodeOf(err), err)
	}
}

func createSandboxServiceTestSession(t *testing.T, ctx context.Context, service *Service) string {
	t.Helper()
	resp, err := service.CreateSession(ctx, connect.NewRequest(&agentcomposev1.CreateSessionRequest{
		Title:      "Sandbox Remove",
		Driver:     driverpkg.RuntimeDriverBoxlite,
		GuestImage: "guest:latest",
		Tags:       []*agentcomposev1.SessionTag{{Name: "project", Value: "project-sandbox-remove"}},
	}))
	if err != nil {
		t.Fatalf("CreateSession returned error: %v", err)
	}
	sessionID := resp.Msg.GetSession().GetSummary().GetSessionId()
	if sessionID == "" || resp.Msg.GetSession().GetSummary().GetVmStatus() != domain.VMStatusRunning {
		t.Fatalf("CreateSession response = %#v", resp.Msg.GetSession().GetSummary())
	}
	return sessionID
}

package agentcompose

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"

	"connectrpc.com/connect"

	"agent-compose/pkg/agentcompose/domain"
	agentcomposev1 "agent-compose/proto/agentcompose/v1"
	agentcomposev2 "agent-compose/proto/agentcompose/v2"
)

func (s *Service) RemoveSandbox(ctx context.Context, req *connect.Request[agentcomposev2.RemoveSandboxRequest]) (*connect.Response[agentcomposev2.RemoveSandboxResponse], error) {
	sandboxID := strings.TrimSpace(req.Msg.GetSandboxId())
	if sandboxID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("sandbox id is required"))
	}
	if sandboxID == "." || sandboxID == ".." || filepath.Base(sandboxID) != sandboxID {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid sandbox id %q", sandboxID))
	}
	session, err := s.store.GetSession(ctx, sandboxID)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	if reconciled, recErr := s.sessions.reconcileSessionRuntimeState(ctx, session); recErr != nil {
		slog.Warn("failed to reconcile sandbox runtime state before remove", "sandbox_id", sandboxID, "error", recErr)
		return nil, connect.NewError(connect.CodeInternal, recErr)
	} else {
		session = reconciled
	}
	stopped := false
	if session.Summary.VMStatus == domain.VMStatusRunning {
		if !req.Msg.GetForce() {
			return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("sandbox %s is running", sandboxID))
		}
		if _, err := s.sessions.stopSession(ctx, connect.NewRequest(&agentcomposev1.SessionIDRequest{SessionId: sandboxID}), domain.SessionTypeManual); err != nil {
			return nil, err
		}
		stopped = true
	}
	if err := s.store.RemoveSession(ctx, sandboxID); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if s.sessions != nil {
		s.sessions.notifyDashboard("session_removed")
	}
	return connect.NewResponse(&agentcomposev2.RemoveSandboxResponse{
		SandboxId: sandboxID,
		Stopped:   stopped,
		Removed:   true,
	}), nil
}

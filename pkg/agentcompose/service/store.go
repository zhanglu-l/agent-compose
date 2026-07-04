package agentcompose

import (
	"context"
	"fmt"

	"github.com/samber/do/v2"

	"agent-compose/pkg/agentcompose/sessionstore"
	appconfig "agent-compose/pkg/config"
)

type Store struct {
	config *appconfig.Config
	inner  *sessionstore.Store
}

func NewStore(di do.Injector) (*Store, error) {
	config := do.MustInvoke[*appconfig.Config](di)
	inner, err := sessionstore.NewWithConfig(config)
	if err != nil {
		return nil, err
	}
	return &Store{config: config, inner: inner}, nil
}

func (s *Store) sessionStore() (*sessionstore.Store, error) {
	if s == nil {
		return nil, fmt.Errorf("session store is required")
	}
	if s.inner != nil {
		return s.inner, nil
	}
	inner, err := sessionstore.NewWithConfig(s.config)
	if err != nil {
		return nil, err
	}
	s.inner = inner
	return inner, nil
}

func (s *Store) CreateSession(ctx context.Context, title, baseWorkspace, driver, guestImage, workspaceID, triggerSource string, workspace *SessionWorkspace, envItems []SessionEnvVar, tags []SessionTag) (*Session, error) {
	store, err := s.sessionStore()
	if err != nil {
		return nil, err
	}
	return store.CreateSession(ctx, title, baseWorkspace, driver, guestImage, workspaceID, triggerSource, workspace, envItems, tags)
}

func (s *Store) GetSession(ctx context.Context, id string) (*Session, error) {
	store, err := s.sessionStore()
	if err != nil {
		return nil, err
	}
	return store.GetSession(ctx, id)
}

func (s *Store) ListSessions(ctx context.Context, options SessionListOptions) (SessionListResult, error) {
	store, err := s.sessionStore()
	if err != nil {
		return SessionListResult{}, err
	}
	return store.ListSessions(ctx, options)
}

func (s *Store) UpdateSession(ctx context.Context, session *Session) error {
	store, err := s.sessionStore()
	if err != nil {
		return err
	}
	return store.UpdateSession(ctx, session)
}

func (s *Store) RemoveSession(ctx context.Context, id string) error {
	store, err := s.sessionStore()
	if err != nil {
		return err
	}
	return store.RemoveSession(ctx, id)
}

func (s *Store) AddCell(ctx context.Context, session *Session, cell NotebookCell) error {
	store, err := s.sessionStore()
	if err != nil {
		return err
	}
	return store.AddCell(ctx, session, cell)
}

func (s *Store) ListCells(ctx context.Context, id string) ([]NotebookCell, error) {
	store, err := s.sessionStore()
	if err != nil {
		return nil, err
	}
	return store.ListCells(ctx, id)
}

func (s *Store) AddAgentRun(ctx context.Context, sessionID string, run AgentRun) error {
	store, err := s.sessionStore()
	if err != nil {
		return err
	}
	return store.AddAgentRun(ctx, sessionID, run)
}

func (s *Store) AddEvent(ctx context.Context, sessionID string, event SessionEvent) error {
	store, err := s.sessionStore()
	if err != nil {
		return err
	}
	return store.AddEvent(ctx, sessionID, event)
}

func (s *Store) ListEvents(ctx context.Context, id string) ([]SessionEvent, error) {
	store, err := s.sessionStore()
	if err != nil {
		return nil, err
	}
	return store.ListEvents(ctx, id)
}

func (s *Store) GetVMState(id string) (VMState, error) {
	store, err := s.sessionStore()
	if err != nil {
		return VMState{}, err
	}
	return store.GetVMState(id)
}

func (s *Store) SaveVMState(id string, state VMState) error {
	store, err := s.sessionStore()
	if err != nil {
		return err
	}
	return store.SaveVMState(id, state)
}

func (s *Store) GetProxyState(id string) (ProxyState, error) {
	store, err := s.sessionStore()
	if err != nil {
		return ProxyState{}, err
	}
	return store.GetProxyState(id)
}

func (s *Store) SaveProxyState(id string, state ProxyState) error {
	store, err := s.sessionStore()
	if err != nil {
		return err
	}
	return store.SaveProxyState(id, state)
}

func (s *Store) sessionDir(id string) string {
	store, err := s.sessionStore()
	if err != nil {
		return ""
	}
	return store.SessionDir(id)
}

func (s *Store) legacyVMStatePath(id string) string {
	store, err := s.sessionStore()
	if err != nil {
		return ""
	}
	return store.LegacyVMStatePath(id)
}

func (s *Store) vmStatePath(id string) string {
	store, err := s.sessionStore()
	if err != nil {
		return ""
	}
	return store.VMStatePath(id)
}

func (s *Store) proxyStatePath(id string) string {
	store, err := s.sessionStore()
	if err != nil {
		return ""
	}
	return store.ProxyStatePath(id)
}

func (s *Store) loadSession(id string) (*Session, error) {
	store, err := s.sessionStore()
	if err != nil {
		return nil, err
	}
	return store.LoadSession(id)
}

func (s *Store) saveSession(session *Session) error {
	store, err := s.sessionStore()
	if err != nil {
		return err
	}
	return store.SaveSession(session)
}

func (s *Store) saveCells(id string, cells []NotebookCell) error {
	store, err := s.sessionStore()
	if err != nil {
		return err
	}
	return store.SaveCells(id, cells)
}

func (s *Store) saveEvents(id string, events []SessionEvent) error {
	store, err := s.sessionStore()
	if err != nil {
		return err
	}
	return store.SaveEvents(id, events)
}

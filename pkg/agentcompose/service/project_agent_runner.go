package agentcompose

import (
	"context"

	agentcomposev2 "agent-compose/proto/agentcompose/v2"
)

type ProjectAgentRunner interface {
	RunProjectAgent(ctx context.Context, msg *agentcomposev2.RunAgentRequest, stream *projectRunStreamSink) (ProjectRunRecord, error, error)
}

type serviceProjectAgentRunner struct {
	service *Service
}

func NewServiceProjectAgentRunner(manager *LoaderManager) ProjectAgentRunner {
	if manager == nil {
		return serviceProjectAgentRunner{service: &Service{}}
	}
	return serviceProjectAgentRunner{service: &Service{
		config:   manager.config,
		store:    manager.store,
		configDB: manager.configDB,
		driver:   manager.driver,
		executor: manager.executor,
		images:   manager.images,
		streams:  manager.streams,
	}}
}

func (r serviceProjectAgentRunner) RunProjectAgent(ctx context.Context, msg *agentcomposev2.RunAgentRequest, stream *projectRunStreamSink) (ProjectRunRecord, error, error) {
	return r.service.runProjectAgent(ctx, msg, stream)
}

func (m *LoaderManager) projectAgentRunnerComponent() ProjectAgentRunner {
	m.initLoaderComponents()
	return m.projectAgentRunner
}

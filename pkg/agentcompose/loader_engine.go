package agentcompose

import (
	"agent-compose/pkg/agentcompose/loaders"
	"context"

	"github.com/samber/do/v2"
)

type (
	LoaderHost             = loaders.LoaderHost
	LoaderValidationResult = loaders.LoaderValidationResult
	LoaderExecutionRequest = loaders.LoaderExecutionRequest
	LoaderExecutionResult  = loaders.LoaderExecutionResult
	LoaderEngine           = loaders.LoaderEngine
	QJSLoaderEngine        = loaders.QJSLoaderEngine
)

func NewLoaderEngine(di do.Injector) (LoaderEngine, error) {
	return loaders.NewLoaderEngine(di)
}

func loaderEngineMaxExecutionTime(ctx context.Context) int {
	return loaders.EngineMaxExecutionTime(ctx)
}

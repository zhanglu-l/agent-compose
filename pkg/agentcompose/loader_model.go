package agentcompose

import "agent-compose/pkg/agentcompose/domain"

const (
	LoaderRuntimeScheduler = domain.LoaderRuntimeScheduler

	LoaderTriggerKindInterval = domain.LoaderTriggerKindInterval
	LoaderTriggerKindEvent    = domain.LoaderTriggerKindEvent
	LoaderTriggerKindTimeout  = domain.LoaderTriggerKindTimeout
	LoaderTriggerKindCron     = domain.LoaderTriggerKindCron

	LoaderSessionPolicySticky = domain.LoaderSessionPolicySticky
	LoaderSessionPolicyNew    = domain.LoaderSessionPolicyNew
	LoaderSessionPolicyReuse  = domain.LoaderSessionPolicyReuse

	LoaderConcurrencyPolicySkip     = domain.LoaderConcurrencyPolicySkip
	LoaderConcurrencyPolicyParallel = domain.LoaderConcurrencyPolicyParallel

	LoaderRunStatusRunning   = domain.LoaderRunStatusRunning
	LoaderRunStatusSucceeded = domain.LoaderRunStatusSucceeded
	LoaderRunStatusFailed    = domain.LoaderRunStatusFailed
	LoaderRunStatusSkipped   = domain.LoaderRunStatusSkipped
)

type (
	LoaderSummary        = domain.LoaderSummary
	Loader               = domain.Loader
	LoaderTrigger        = domain.LoaderTrigger
	LoaderRunSummary     = domain.LoaderRunSummary
	LoaderEvent          = domain.LoaderEvent
	LoaderBinding        = domain.LoaderBinding
	LoaderAgentRequest   = domain.LoaderAgentRequest
	LoaderAgentResult    = domain.LoaderAgentResult
	LoaderCommandRequest = domain.LoaderCommandRequest
	LoaderCommandResult  = domain.LoaderCommandResult
	LoaderLLMRequest     = domain.LoaderLLMRequest
	LoaderLLMResult      = domain.LoaderLLMResult
	LoaderTopicEvent     = domain.LoaderTopicEvent
)

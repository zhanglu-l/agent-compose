package agentcompose

import (
	"time"

	"agent-compose/pkg/agentcompose/loaders"
)

func loaderTriggerNextFireAt(now time.Time, trigger LoaderTrigger, fired bool) (time.Time, error) {
	return loaders.LoaderTriggerNextFireAt(now, trigger, fired)
}

func loaderTriggerSource(trigger LoaderTrigger) string {
	return loaders.LoaderTriggerSource(trigger)
}

func normalizeLoaderCronSpecJSON(raw string) (string, error) {
	return loaders.NormalizeLoaderCronSpecJSON(raw)
}

func loaderCronSpecJSON(expr, timezone string) (string, error) {
	return loaders.LoaderCronSpecJSON(expr, timezone)
}

package loaders

import (
	"agent-compose/pkg/agentcompose/domain"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	cronlib "github.com/robfig/cron/v3"
)

const loaderDefaultCronTimezone = "UTC"

type loaderCronSpec struct {
	Kind     string `json:"kind,omitempty"`
	Expr     string `json:"expr"`
	Timezone string `json:"timezone,omitempty"`
}

var loaderCronParser = cronlib.NewParser(cronlib.SecondOptional | cronlib.Minute | cronlib.Hour | cronlib.Dom | cronlib.Month | cronlib.Dow | cronlib.Descriptor)

func loaderCronSpecJSON(expr, timezone string) (string, error) {
	spec, err := normalizeLoaderCronSpec(loaderCronSpec{
		Kind:     domain.LoaderTriggerKindCron,
		Expr:     expr,
		Timezone: timezone,
	})
	if err != nil {
		return "", err
	}
	return marshalJSONCompact(spec)
}

func normalizeLoaderCronSpec(spec loaderCronSpec) (loaderCronSpec, error) {
	spec.Kind = domain.LoaderTriggerKindCron
	spec.Expr = strings.TrimSpace(spec.Expr)
	spec.Timezone = strings.TrimSpace(spec.Timezone)
	if spec.Expr == "" {
		return loaderCronSpec{}, fmt.Errorf("cron expr is required")
	}
	if spec.Timezone == "" {
		spec.Timezone = loaderDefaultCronTimezone
	}
	if _, err := time.LoadLocation(spec.Timezone); err != nil {
		return loaderCronSpec{}, fmt.Errorf("load cron timezone %q: %w", spec.Timezone, err)
	}
	if _, err := loaderCronParser.Parse(spec.Expr); err != nil {
		return loaderCronSpec{}, fmt.Errorf("parse cron expression %q: %w", spec.Expr, err)
	}
	return spec, nil
}

func normalizeAgentKind(agent string) string {
	return domain.NormalizeAgentKind(agent)
}

func normalizeLoaderSessionPolicy(policy string) string {
	return domain.NormalizeLoaderSessionPolicy(policy)
}

func normalizeEnvItems(items []domain.SessionEnvVar) []domain.SessionEnvVar {
	return domain.NormalizeEnvItems(items)
}

func loaderJSONResult(text, outputSchemaJSON, sourceName string) (any, error) {
	if strings.TrimSpace(outputSchemaJSON) == "" {
		return nil, nil
	}
	var parsed any
	if err := json.Unmarshal([]byte(text), &parsed); err != nil {
		return nil, fmt.Errorf("%s is not valid JSON for outputSchema: %w", sourceName, err)
	}
	return parsed, nil
}

func marshalJSONCompact(value any) (string, error) {
	if value == nil {
		return "", nil
	}
	data, err := json.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("encode json payload: %w", err)
	}
	return string(data), nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

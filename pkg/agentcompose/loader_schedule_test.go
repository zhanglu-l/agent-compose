package agentcompose

import (
	"agent-compose/pkg/agentcompose/domain"
	"strings"
	"testing"
	"time"
)

func TestLoaderScheduleModelWorkflows(t *testing.T) {
	testLoaderScheduleModelWorkflows(t)
}

func testLoaderScheduleModelWorkflows(t *testing.T) {
	t.Helper()
	now := time.Date(2026, 6, 2, 9, 0, 0, 0, time.UTC)

	next, err := loaderTriggerNextFireAt(now, LoaderTrigger{Kind: LoaderTriggerKindInterval, IntervalMs: 1500}, false)
	if err != nil {
		t.Fatalf("interval next fire returned error: %v", err)
	}
	if !next.Equal(now.Add(1500 * time.Millisecond)) {
		t.Fatalf("interval next fire = %s", next)
	}
	next, err = loaderTriggerNextFireAt(now, LoaderTrigger{Kind: LoaderTriggerKindTimeout, IntervalMs: 2000}, true)
	if err != nil {
		t.Fatalf("fired timeout next fire returned error: %v", err)
	}
	if !next.IsZero() {
		t.Fatalf("fired timeout next fire = %s, want zero", next)
	}

	specJSON, err := loaderCronSpecJSON("*/5 * * * *", "Asia/Shanghai")
	if err != nil {
		t.Fatalf("loaderCronSpecJSON returned error: %v", err)
	}
	next, err = loaderTriggerNextFireAt(now, LoaderTrigger{Kind: LoaderTriggerKindCron, SpecJSON: specJSON}, false)
	if err != nil {
		t.Fatalf("cron next fire returned error: %v", err)
	}
	if next.IsZero() || !next.After(now) {
		t.Fatalf("cron next fire = %s, want after %s", next, now)
	}
	if source := loaderTriggerSource(LoaderTrigger{Kind: LoaderTriggerKindCron, SpecJSON: specJSON}); source != "cron:*/5 * * * *@Asia/Shanghai" {
		t.Fatalf("cron source = %q", source)
	}
	if source := loaderTriggerSource(LoaderTrigger{Kind: LoaderTriggerKindInterval, IntervalMs: 1000}); source != "interval:1000" {
		t.Fatalf("interval source = %q", source)
	}
	if source := loaderTriggerSource(LoaderTrigger{Kind: LoaderTriggerKindTimeout, IntervalMs: 2000}); source != "timeout:2000" {
		t.Fatalf("timeout source = %q", source)
	}
	if source := loaderTriggerSource(LoaderTrigger{Kind: LoaderTriggerKindCron, SpecJSON: `{bad json`}); source != "cron" {
		t.Fatalf("invalid cron source = %q", source)
	}

	normalized, err := normalizeLoaderCronSpecJSON(`{"expr":"@hourly"}`)
	if err != nil {
		t.Fatalf("normalizeLoaderCronSpecJSON returned error: %v", err)
	}
	if !strings.Contains(normalized, `"timezone":"UTC"`) {
		t.Fatalf("normalized cron spec = %q", normalized)
	}
	if _, err := normalizeLoaderCronSpecJSON(`{"expr":""}`); err == nil {
		t.Fatalf("empty cron expression returned nil error")
	}
	if _, err := normalizeLoaderCronSpecJSON(`{"expr":"* * * * *","timezone":"No/SuchZone"}`); err == nil {
		t.Fatalf("invalid cron timezone returned nil error")
	}
	if _, err := loaderTriggerNextFireAt(now, LoaderTrigger{Kind: LoaderTriggerKindCron, SpecJSON: `{"expr":"bad cron"}`}, false); err == nil {
		t.Fatalf("invalid cron trigger returned nil error")
	}

	stableID := domain.LoaderTriggerStableID(LoaderTriggerKindEvent, "runtime.*", 0, "function cb() {}", 1)
	if stableID != domain.LoaderTriggerStableID(LoaderTriggerKindEvent, "runtime.*", 0, "function cb() {}", 1) {
		t.Fatalf("stable trigger id was not stable")
	}
	if domain.LoaderSourceSHA("script") == domain.LoaderSourceSHA("other") {
		t.Fatalf("loaderSourceSHA returned identical values for different scripts")
	}
	if !domain.LoaderTriggerTopicMatches("runtime.*", "runtime.test") || !domain.LoaderTriggerTopicMatches("runtime.test", "runtime.test") {
		t.Fatalf("expected topic patterns to match")
	}
	if domain.LoaderTriggerTopicMatches("", "runtime.test") || domain.LoaderTriggerTopicMatches("runtime.*", "") || domain.LoaderTriggerTopicMatches("runtime.test", "runtime.other") {
		t.Fatalf("unexpected topic match")
	}

	if domain.NormalizeLoaderSessionPolicy("new") != LoaderSessionPolicyNew || domain.NormalizeLoaderSessionPolicy("bad") != LoaderSessionPolicySticky {
		t.Fatalf("session policy normalization failed")
	}
	if domain.NormalizeLoaderConcurrencyPolicy("allow") != LoaderConcurrencyPolicyParallel || domain.NormalizeLoaderConcurrencyPolicy("bad") != LoaderConcurrencyPolicySkip {
		t.Fatalf("concurrency policy normalization failed")
	}
	if domain.NormalizeLoaderRunStatus("failed") != LoaderRunStatusFailed || domain.NormalizeLoaderRunStatus("bad") != LoaderRunStatusRunning {
		t.Fatalf("run status normalization failed")
	}
	if !domain.TimeIsSet(now) || domain.NonZeroTimeUnixMilli(time.Time{}) != 0 || domain.NonZeroTimeUnixMilli(now) != now.UnixMilli() {
		t.Fatalf("time helpers returned unexpected values")
	}
	if !domain.LoaderTriggerUsesSchedule(LoaderTriggerKindCron) || domain.LoaderTriggerUsesSchedule(LoaderTriggerKindEvent) {
		t.Fatalf("schedule trigger helper returned unexpected values")
	}
	if !domain.LoaderTriggerScheduledAt(now, 0).IsZero() || !domain.LoaderTriggerScheduledAt(now, 1).Equal(now.Add(time.Millisecond)) {
		t.Fatalf("scheduled at helper returned unexpected values")
	}
	if domain.DefaultLoaderName(now) != "Loader 2026-06-02 09:00" {
		t.Fatalf("default loader name = %q", domain.DefaultLoaderName(now))
	}
	if script := domain.DefaultLoaderScript(); !strings.Contains(script, "function main") || !strings.Contains(script, "scheduler.interval") || !strings.Contains(script, "scheduler.on") {
		t.Fatalf("default loader script missing expected registrations: %s", script)
	}
}

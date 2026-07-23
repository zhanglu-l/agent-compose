package configstore

import (
	"context"
	"testing"
	"time"

	"agent-compose/pkg/loaders"
	domain "agent-compose/pkg/model"
)

func TestLoaderEventPageOnlyReturnsEventsJoinedToTriggerRuns(t *testing.T) {
	ctx := context.Background()
	store := FromDB(newMemoryDB(t))
	if err := store.initSchema(ctx); err != nil {
		t.Fatalf("init schema: %v", err)
	}
	if _, err := store.UpsertManagedLoader(ctx, domain.Loader{Summary: domain.LoaderSummary{ID: "loader-a", Runtime: domain.LoaderRuntimeScheduler, ManagedProjectID: "project-1", ManagedAgentName: "agent-1", ManagedSchedulerID: "scheduler-1"}, Script: "function main() {}"}); err != nil {
		t.Fatalf("upsert loader: %v", err)
	}
	startedAt := time.UnixMilli(1_720_000_000_000).UTC()
	for _, run := range []domain.LoaderRunSummary{
		{ID: "run-a", LoaderID: "loader-a", TriggerID: "trigger-a", Status: domain.LoaderRunStatusSucceeded, StartedAt: startedAt},
		{ID: "run-b", LoaderID: "loader-a", TriggerID: "trigger-b", Status: domain.LoaderRunStatusSucceeded, StartedAt: startedAt},
		{ID: "invoke-old", LoaderID: "loader-a", Status: domain.LoaderRunStatusSucceeded, StartedAt: startedAt},
	} {
		if err := store.CreateLoaderRun(ctx, run); err != nil {
			t.Fatalf("create run: %v", err)
		}
	}
	for _, event := range []domain.LoaderEvent{
		{LoaderID: "loader-a", ID: "event-a2", RunID: "run-a", TriggerID: "wrong-event-trigger", Type: "loader.log", CreatedAt: startedAt.Add(2 * time.Second)},
		{LoaderID: "loader-a", ID: "event-a1", RunID: "run-a", TriggerID: "trigger-a", Type: "loader.run.started", CreatedAt: startedAt.Add(time.Second)},
		{LoaderID: "loader-a", ID: "event-b", RunID: "run-b", TriggerID: "trigger-b", Type: "loader.log", CreatedAt: startedAt},
		{LoaderID: "loader-a", ID: "event-invoke", RunID: "invoke-old", Type: "loader.log", CreatedAt: startedAt.Add(3 * time.Second)},
		{LoaderID: "loader-a", ID: "event-orphan", RunID: "missing", TriggerID: "trigger-a", Type: "loader.log", CreatedAt: startedAt.Add(4 * time.Second)},
	} {
		if err := store.AddLoaderEvent(ctx, event); err != nil {
			t.Fatalf("add event %s: %v", event.ID, err)
		}
	}
	first, err := store.ListLoaderEventsPage(ctx, loaders.LoaderEventPageFilter{LoaderIDs: []string{"loader-a"}, RequireTrigger: true, TriggerID: "trigger-a", Limit: 1})
	if err != nil || len(first) != 1 || first[0].ID != "event-a2" || first[0].TriggerID != "trigger-a" {
		t.Fatalf("first page=%#v err=%v", first, err)
	}
	second, err := store.ListLoaderEventsPage(ctx, loaders.LoaderEventPageFilter{
		LoaderIDs: []string{"loader-a"}, RequireTrigger: true, TriggerID: "trigger-a", BeforeCreatedAt: first[0].CreatedAt,
		BeforeLoaderID: first[0].LoaderID, BeforeEventID: first[0].ID, Limit: 10,
	})
	if err != nil || len(second) != 1 || second[0].ID != "event-a1" {
		t.Fatalf("second page=%#v err=%v", second, err)
	}
	byRun, err := store.ListLoaderEventsPage(ctx, loaders.LoaderEventPageFilter{LoaderIDs: []string{"loader-a"}, RequireTrigger: true, RunID: "run-b", Limit: 10})
	if err != nil || len(byRun) != 1 || byRun[0].ID != "event-b" {
		t.Fatalf("run filter=%#v err=%v", byRun, err)
	}
}

func TestLoaderEventPageSupportsTailBoundaryAndAscendingRange(t *testing.T) {
	ctx := context.Background()
	store := FromDB(newMemoryDB(t))
	if err := store.initSchema(ctx); err != nil {
		t.Fatalf("init schema: %v", err)
	}
	if _, err := store.UpsertManagedLoader(ctx, domain.Loader{Summary: domain.LoaderSummary{ID: "loader-a", Runtime: domain.LoaderRuntimeScheduler, ManagedProjectID: "project-1", ManagedAgentName: "agent-1", ManagedSchedulerID: "scheduler-1"}, Script: "function main() {}"}); err != nil {
		t.Fatalf("upsert loader: %v", err)
	}
	startedAt := time.UnixMilli(1_730_000_000_000).UTC()
	if err := store.CreateLoaderRun(ctx, domain.LoaderRunSummary{ID: "run-a", LoaderID: "loader-a", TriggerID: "trigger-a", Status: domain.LoaderRunStatusSucceeded, StartedAt: startedAt}); err != nil {
		t.Fatalf("create run: %v", err)
	}
	for index, id := range []string{"event-1", "event-2", "event-3", "event-4"} {
		if err := store.AddLoaderEvent(ctx, domain.LoaderEvent{LoaderID: "loader-a", ID: id, RunID: "run-a", TriggerID: "trigger-a", Type: "loader.log", CreatedAt: startedAt.Add(time.Duration(index) * time.Second)}); err != nil {
			t.Fatalf("add event %s: %v", id, err)
		}
	}
	boundary, err := store.ListLoaderEventsPage(ctx, loaders.LoaderEventPageFilter{LoaderIDs: []string{"loader-a"}, RequireTrigger: true, Limit: 1, Offset: 1})
	if err != nil || len(boundary) != 1 || boundary[0].ID != "event-3" {
		t.Fatalf("tail boundary = %#v, err=%v", boundary, err)
	}
	page, err := store.ListLoaderEventsPage(ctx, loaders.LoaderEventPageFilter{
		LoaderIDs: []string{"loader-a"}, RequireTrigger: true, Ascending: true, Limit: 10,
		AfterCreatedAt: boundary[0].CreatedAt, AfterLoaderID: boundary[0].LoaderID, AfterEventID: boundary[0].ID,
		ThroughCreatedAt: startedAt.Add(3 * time.Second), ThroughLoaderID: "loader-a", ThroughEventID: "event-4",
	})
	if err != nil || len(page) != 1 || page[0].ID != "event-4" {
		t.Fatalf("ascending bounded page = %#v, err=%v", page, err)
	}
}

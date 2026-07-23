package api

import (
	"context"
	"fmt"
	"strings"

	"connectrpc.com/connect"

	"agent-compose/pkg/loaders"
	domain "agent-compose/pkg/model"
	agentcomposev2 "agent-compose/proto/agentcompose/v2"
)

const defaultSchedulerStreamBatchSize = 100

type schedulerEventStreamSelection struct {
	project    domain.ProjectRecord
	schedulers map[string]domain.ProjectSchedulerRecord
	loaderIDs  []string
	agentName  string
	triggerID  string
	runID      string
	store      ProjectSchedulerEventStore
}

// StreamProjectSchedulerEvents sends a finite scheduler-event scan in display
// order. Database rows are fully scanned and released before each network send.
func (h *ProjectHandler) StreamProjectSchedulerEvents(ctx context.Context, req *connect.Request[agentcomposev2.StreamProjectSchedulerEventsRequest], stream *connect.ServerStream[agentcomposev2.StreamProjectSchedulerEventsResponse]) error {
	selection, err := h.schedulerEventStreamSelection(ctx, req.Msg)
	if err != nil {
		return err
	}
	batchSize, err := schedulerStreamBatchSize(req.Msg.GetBatchSize())
	if err != nil {
		return ConnectErrorForDomain(err)
	}
	PrepareStreamingHeaders(stream.ResponseHeader())
	baseFilter := loaders.LoaderEventPageFilter{
		LoaderIDs:      selection.loaderIDs,
		RequireTrigger: true,
		TriggerID:      selection.triggerID,
		RunID:          selection.runID,
	}
	upper, err := selection.store.ListLoaderEventsPage(ctx, loaderEventPage(baseFilter, 1, 0, false))
	if err != nil {
		return ConnectErrorForDomain(err)
	}
	if len(upper) == 0 {
		return stream.Send(&agentcomposev2.StreamProjectSchedulerEventsResponse{Complete: true})
	}
	var lower *domain.LoaderEvent
	if tail := req.Msg.GetTail(); tail > 0 {
		boundary, queryErr := selection.store.ListLoaderEventsPage(ctx, loaderEventPage(baseFilter, 1, int(tail-1), false))
		if queryErr != nil {
			return ConnectErrorForDomain(queryErr)
		}
		if len(boundary) > 0 {
			lower = &boundary[0]
		}
	}
	return h.sendSchedulerEventPages(ctx, stream, selection, baseFilter, lower, upper[0], batchSize)
}

func (h *ProjectHandler) sendSchedulerEventPages(ctx context.Context, stream *connect.ServerStream[agentcomposev2.StreamProjectSchedulerEventsResponse], selection schedulerEventStreamSelection, base loaders.LoaderEventPageFilter, lower *domain.LoaderEvent, upper domain.LoaderEvent, batchSize int) error {
	var after *domain.LoaderEvent
	var emitted uint64
	checkpoint := ""
	for {
		filter := loaderEventPage(base, batchSize, 0, true)
		filter.ThroughCreatedAt, filter.ThroughLoaderID, filter.ThroughEventID = upper.CreatedAt, upper.LoaderID, upper.ID
		if lower != nil {
			filter.FromCreatedAt, filter.FromLoaderID, filter.FromEventID = lower.CreatedAt, lower.LoaderID, lower.ID
		}
		if after != nil {
			filter.AfterCreatedAt, filter.AfterLoaderID, filter.AfterEventID = after.CreatedAt, after.LoaderID, after.ID
		}
		events, err := selection.store.ListLoaderEventsPage(ctx, filter)
		if err != nil {
			return ConnectErrorForDomain(err)
		}
		if len(events) == 0 {
			break
		}
		items := make([]*agentcomposev2.SchedulerEvent, 0, len(events))
		for _, event := range events {
			scheduler, ok := selection.schedulers[event.LoaderID]
			if ok {
				items = append(items, schedulerEventToProto(event, scheduler))
			}
		}
		last := events[len(events)-1]
		after = &last
		emitted += uint64(len(items))
		checkpoint = encodeProjectSchedulerEventCursor(selection.project.ID, selection.project.CurrentRevision, selection.agentName, selection.triggerID, selection.runID, last)
		if err := stream.Send(&agentcomposev2.StreamProjectSchedulerEventsResponse{Events: items, Checkpoint: checkpoint, EmittedCount: emitted}); err != nil {
			return err
		}
		if len(events) < batchSize || sameLoaderEventKey(last, upper) {
			break
		}
	}
	return stream.Send(&agentcomposev2.StreamProjectSchedulerEventsResponse{Complete: true, Checkpoint: checkpoint, EmittedCount: emitted})
}

func (h *ProjectHandler) schedulerEventStreamSelection(ctx context.Context, req *agentcomposev2.StreamProjectSchedulerEventsRequest) (schedulerEventStreamSelection, error) {
	project, schedulers, err := h.resolveProjectSchedulerRunTargets(ctx, req.GetProject(), req.GetAgentName())
	if err != nil {
		return schedulerEventStreamSelection{}, ConnectErrorForDomain(err)
	}
	agentName := strings.TrimSpace(req.GetAgentName())
	triggerID := strings.TrimSpace(req.GetTriggerId())
	runID := strings.TrimSpace(req.GetRunId())
	if runID != "" {
		_, runScheduler, run, resolveErr := h.resolveProjectSchedulerRun(ctx, req.GetProject(), runID)
		if resolveErr != nil {
			return schedulerEventStreamSelection{}, ConnectErrorForDomain(resolveErr)
		}
		if agentName != "" && agentName != runScheduler.AgentName {
			return schedulerEventStreamSelection{}, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("scheduler run does not belong to scheduler %q", agentName))
		}
		if triggerID != "" && triggerID != run.TriggerID {
			return schedulerEventStreamSelection{}, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("scheduler run does not belong to trigger %q", triggerID))
		}
		agentName, triggerID, runID = runScheduler.AgentName, run.TriggerID, run.ID
		schedulers = []domain.ProjectSchedulerRecord{runScheduler}
	}
	loaderIDs, byLoaderID := schedulerLoaderIndex(schedulers)
	store, ok := h.store.(ProjectSchedulerEventStore)
	if !ok {
		return schedulerEventStreamSelection{}, connect.NewError(connect.CodeInternal, fmt.Errorf("scheduler event store is required"))
	}
	return schedulerEventStreamSelection{project: project, schedulers: byLoaderID, loaderIDs: loaderIDs, agentName: agentName, triggerID: triggerID, runID: runID, store: store}, nil
}

// StreamSchedulerRuns sends bounded run batches until the finite scan or the
// caller's final result limit is reached.
func (h *ProjectHandler) StreamSchedulerRuns(ctx context.Context, req *connect.Request[agentcomposev2.StreamSchedulerRunsRequest], stream *connect.ServerStream[agentcomposev2.StreamSchedulerRunsResponse]) error {
	project, schedulers, err := h.resolveProjectSchedulerRunTargets(ctx, req.Msg.GetProject(), req.Msg.GetAgentName())
	if err != nil {
		return ConnectErrorForDomain(err)
	}
	status, err := schedulerRunStatusFilter(req.Msg.GetStatus())
	if err != nil {
		return ConnectErrorForDomain(err)
	}
	batchSize, err := schedulerStreamBatchSize(req.Msg.GetBatchSize())
	if err != nil {
		return ConnectErrorForDomain(err)
	}
	store, err := h.schedulerRunStore()
	if err != nil {
		return ConnectErrorForDomain(err)
	}
	loaderIDs, byLoaderID := schedulerLoaderIndex(schedulers)
	PrepareStreamingHeaders(stream.ResponseHeader())
	return h.sendSchedulerRunPages(ctx, stream, schedulerRunStreamSelection{
		project: project, schedulers: byLoaderID, loaderIDs: loaderIDs, agentName: strings.TrimSpace(req.Msg.GetAgentName()),
		triggerID: strings.TrimSpace(req.Msg.GetTriggerId()), status: status, store: store, limit: req.Msg.GetLimit(), batchSize: batchSize,
	})
}

type schedulerRunStreamSelection struct {
	project    domain.ProjectRecord
	schedulers map[string]domain.ProjectSchedulerRecord
	loaderIDs  []string
	agentName  string
	triggerID  string
	status     string
	store      ProjectSchedulerRunStore
	limit      uint32
	batchSize  int
}

func (h *ProjectHandler) sendSchedulerRunPages(ctx context.Context, stream *connect.ServerStream[agentcomposev2.StreamSchedulerRunsResponse], selection schedulerRunStreamSelection) error {
	var before *domain.LoaderRunSummary
	var emitted uint64
	checkpoint := ""
	for {
		pageSize := selection.batchSize
		if selection.limit > 0 && uint64(pageSize) > uint64(selection.limit)-emitted {
			pageSize = int(uint64(selection.limit) - emitted)
		}
		if pageSize == 0 {
			return stream.Send(&agentcomposev2.StreamSchedulerRunsResponse{Complete: true, Checkpoint: checkpoint, EmittedCount: emitted, Truncated: true})
		}
		filter := loaders.LoaderRunPageFilter{LoaderIDs: selection.loaderIDs, RequireTrigger: true, TriggerID: selection.triggerID, Status: selection.status, Limit: pageSize + 1}
		if before != nil {
			filter.BeforeStartedAt, filter.BeforeLoaderID, filter.BeforeRunID = before.StartedAt, before.LoaderID, before.ID
		}
		runs, err := selection.store.ListLoaderRunsPage(ctx, filter)
		if err != nil {
			return ConnectErrorForDomain(err)
		}
		if len(runs) == 0 {
			break
		}
		end := min(pageSize, len(runs))
		page := runs[:end]
		items, err := h.schedulerRunStreamItems(ctx, selection.schedulers, page)
		if err != nil {
			return err
		}
		last := page[len(page)-1]
		before = &last
		emitted += uint64(len(items))
		checkpoint = encodeSchedulerRunCursor(selection.project.ID, selection.project.CurrentRevision, selection.agentName, selection.triggerID, selection.status, last)
		if err := stream.Send(&agentcomposev2.StreamSchedulerRunsResponse{Runs: items, Checkpoint: checkpoint, EmittedCount: emitted}); err != nil {
			return err
		}
		if len(runs) <= end {
			break
		}
		if selection.limit > 0 && emitted >= uint64(selection.limit) {
			return stream.Send(&agentcomposev2.StreamSchedulerRunsResponse{Complete: true, Checkpoint: checkpoint, EmittedCount: emitted, Truncated: true})
		}
	}
	return stream.Send(&agentcomposev2.StreamSchedulerRunsResponse{Complete: true, Checkpoint: checkpoint, EmittedCount: emitted})
}

func (h *ProjectHandler) schedulerRunStreamItems(ctx context.Context, schedulers map[string]domain.ProjectSchedulerRecord, runs []domain.LoaderRunSummary) ([]*agentcomposev2.SchedulerRun, error) {
	keys := make([]loaders.LoaderRunKey, 0, len(runs))
	for _, run := range runs {
		keys = append(keys, loaders.LoaderRunKey{LoaderID: run.LoaderID, RunID: run.ID})
	}
	sandboxIDs := make(map[loaders.LoaderRunKey][]string)
	if sandboxStore, ok := h.store.(ProjectSchedulerRunSandboxStore); ok {
		var err error
		sandboxIDs, err = sandboxStore.ListLoaderRunSandboxIDs(ctx, keys)
		if err != nil {
			return nil, ConnectErrorForDomain(err)
		}
	}
	items := make([]*agentcomposev2.SchedulerRun, 0, len(runs))
	for _, run := range runs {
		scheduler, ok := schedulers[run.LoaderID]
		if !ok {
			continue
		}
		item := schedulerRunToProto(run, scheduler)
		item.SandboxIds = sandboxIDs[loaders.LoaderRunKey{LoaderID: run.LoaderID, RunID: run.ID}]
		items = append(items, item)
	}
	return items, nil
}

func schedulerLoaderIndex(schedulers []domain.ProjectSchedulerRecord) ([]string, map[string]domain.ProjectSchedulerRecord) {
	loaderIDs := make([]string, 0, len(schedulers))
	byLoaderID := make(map[string]domain.ProjectSchedulerRecord, len(schedulers))
	for _, scheduler := range schedulers {
		loaderID := strings.TrimSpace(scheduler.ManagedLoaderID)
		if loaderID == "" {
			continue
		}
		loaderIDs = append(loaderIDs, loaderID)
		byLoaderID[loaderID] = scheduler
	}
	return loaderIDs, byLoaderID
}

func schedulerStreamBatchSize(value uint32) (int, error) {
	if value == 0 {
		return defaultSchedulerStreamBatchSize, nil
	}
	if value > 500 {
		return 0, domain.ClassifyError(domain.ErrInvalidArgument, "scheduler stream batch size must be at most 500", nil)
	}
	return int(value), nil
}

func loaderEventPage(base loaders.LoaderEventPageFilter, limit, offset int, ascending bool) loaders.LoaderEventPageFilter {
	base.Limit = limit
	base.Offset = offset
	base.Ascending = ascending
	return base
}

func sameLoaderEventKey(left, right domain.LoaderEvent) bool {
	return left.LoaderID == right.LoaderID && left.ID == right.ID && left.CreatedAt.Equal(right.CreatedAt)
}

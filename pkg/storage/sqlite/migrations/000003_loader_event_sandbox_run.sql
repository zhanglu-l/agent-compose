CREATE INDEX IF NOT EXISTS idx_loader_event_sandbox_run
    ON loader_event(linked_sandbox_id, loader_id, run_id)
    WHERE linked_sandbox_id <> '';

-- The sandbox table is a rebuildable projection of filesystem metadata. Drop
-- the previous projection so the session store repopulates project ownership
-- from the authoritative run records before serving indexed queries.
DROP TABLE IF EXISTS sandboxes;
DROP TABLE IF EXISTS sandbox_projection_meta;

CREATE TABLE sandbox_projection_meta (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    version INTEGER NOT NULL
);

CREATE TABLE sandboxes (
    id                         TEXT PRIMARY KEY,
    short_id                   TEXT NOT NULL DEFAULT '',
    title                      TEXT NOT NULL DEFAULT '',
    trigger_source             TEXT NOT NULL DEFAULT '',
    driver                     TEXT NOT NULL DEFAULT '',
    vm_status                  TEXT NOT NULL DEFAULT '',
    project_id                 TEXT NOT NULL DEFAULT '',
    workspace_path             TEXT NOT NULL DEFAULT '',
    workspace_id               TEXT NOT NULL DEFAULT '',
    nested_workspace_id        TEXT NOT NULL DEFAULT '',
    workspace_name             TEXT NOT NULL DEFAULT '',
    workspace_type             TEXT NOT NULL DEFAULT '',
    created_at                 INTEGER NOT NULL DEFAULT 0,
    updated_at                 INTEGER NOT NULL DEFAULT 0,
    sandbox_type               TEXT NOT NULL DEFAULT '',
    title_search               TEXT NOT NULL DEFAULT '',
    trigger_source_search      TEXT NOT NULL DEFAULT '',
    driver_search              TEXT NOT NULL DEFAULT '',
    vm_status_search           TEXT NOT NULL DEFAULT '',
    project_id_search          TEXT NOT NULL DEFAULT '',
    workspace_path_search      TEXT NOT NULL DEFAULT '',
    workspace_id_search        TEXT NOT NULL DEFAULT '',
    nested_workspace_id_search TEXT NOT NULL DEFAULT '',
    workspace_name_search      TEXT NOT NULL DEFAULT '',
    workspace_type_search      TEXT NOT NULL DEFAULT ''
);

CREATE INDEX idx_sandboxes_updated
    ON sandboxes(updated_at DESC, id DESC);
CREATE INDEX idx_sandboxes_vm_status_updated
    ON sandboxes(vm_status_search, updated_at DESC, id DESC);
CREATE INDEX idx_sandboxes_project_updated
    ON sandboxes(project_id_search, updated_at DESC, id DESC);
CREATE INDEX idx_sandboxes_type_updated
    ON sandboxes(sandbox_type, updated_at DESC, id DESC);

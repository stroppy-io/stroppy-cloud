CREATE TABLE shared_runs (
    token      TEXT PRIMARY KEY,
    run_id     TEXT NOT NULL,
    tenant_id  TEXT NOT NULL,
    snapshot   TEXT NOT NULL,       -- frozen run snapshot (config, nodes, timing)
    metrics    TEXT NOT NULL,       -- frozen metrics JSON
    created_by TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- +goose Up
-- Record-scoped summaries include terminal history. Do not use a live-only
-- partial index. Equality prefix plus the stable display order avoids sorting.
CREATE INDEX ix_workflow_instances_record_order
 ON applications.workflow_instances(app_id,table_id,record_id,created_at DESC,id DESC);

-- +goose Down
-- Performance-only rollback: retain every instance, command and event.
DROP INDEX applications.ix_workflow_instances_record_order;

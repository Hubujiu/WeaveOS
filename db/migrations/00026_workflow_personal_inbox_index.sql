-- +goose Up
-- Global personal inbox is assignee-scoped rather than app-scoped. Preserve
-- the existing per-app index and use the exact unclosed candidate predicate.
CREATE INDEX ix_workflow_tasks_personal_order
 ON applications.workflow_tasks(assignee_id,created_at DESC,id DESC)
 WHERE closed_command_id IS NULL;

-- +goose Down
-- Performance-only rollback: retain all tasks, instances and event history.
DROP INDEX applications.ix_workflow_tasks_personal_order;

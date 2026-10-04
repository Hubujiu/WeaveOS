-- +goose Up
ALTER TABLE applications.operations
    DROP CONSTRAINT ck_operation_kind,
    ADD CONSTRAINT ck_operation_kind CHECK (operation_kind IN (
        'application.create',
        'group.create',
        'group.update',
        'members.replace',
        'grants.replace',
        'directory.create',
        'directory.update',
        'table.create',
        'table.update',
        'form.create',
        'form.update',
        'definition.save',
        'record.create',
        'record.edit',
        'draft.create',
        'draft.update',
        'draft.discard',
        'workflow.definition.save',
        'workflow.enable',
        'workflow.close'
    ));

-- +goose Down
LOCK TABLE applications.operations IN ACCESS EXCLUSIVE MODE;
-- +goose StatementBegin
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM applications.operations
        WHERE operation_kind IN ('workflow.definition.save', 'workflow.enable', 'workflow.close')
    ) THEN
        RAISE EXCEPTION 'cannot remove workflow management operation kinds while history exists'
            USING ERRCODE = '55000';
    END IF;
END;
$$;
-- +goose StatementEnd

ALTER TABLE applications.operations
    DROP CONSTRAINT ck_operation_kind,
    ADD CONSTRAINT ck_operation_kind CHECK (operation_kind IN (
        'application.create',
        'group.create',
        'group.update',
        'members.replace',
        'grants.replace',
        'directory.create',
        'directory.update',
        'table.create',
        'table.update',
        'form.create',
        'form.update',
        'definition.save',
        'record.create',
        'record.edit',
        'draft.create',
        'draft.update',
        'draft.discard'
    ));

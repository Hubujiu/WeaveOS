package appschema

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

var namespaceID = regexp.MustCompile(`^[a-z_][a-z0-9_]{0,62}$`)

// Save owns exactly one transaction. Dependency and metadata adapters are
// injected; no flow/application tables or permission rules are guessed here.
func (e Executor) Save(ctx context.Context, request Request) (Result, error) {
	if e.DB == nil {
		return Result{}, ErrInvalid
	}
	tx, err := e.DB.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return Result{}, err
	}
	defer tx.Rollback(context.Background())
	result, err := e.ApplyInTx(ctx, tx, request)
	if err != nil {
		return Result{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		if errors.Is(err, pgx.ErrTxCommitRollback) {
			return Result{}, err
		}
		return Result{}, &CommitError{Cause: err}
	}
	return result, nil
}

// ApplyInTx never owns the caller's transaction lifetime.
func (e Executor) ApplyInTx(ctx context.Context, tx pgx.Tx, request Request) (Result, error) {
	if tx == nil || e.Metadata == nil || e.Guard == nil || !namespaceID.MatchString(e.Namespace) || e.Limits.LockTimeout < time.Millisecond || e.Limits.StatementTimeout < time.Millisecond {
		return Result{}, ErrInvalid
	}
	input, err := BuildPlan(request.TableID, nil, request.Fields)
	if err != nil {
		return Result{}, err
	}
	request.Fields = input.Fields
	if _, err = tx.Exec(ctx, "SELECT set_config('lock_timeout',$1,true),set_config('statement_timeout',$2,true)", milliseconds(e.Limits.LockTimeout), milliseconds(e.Limits.StatementTimeout)); err != nil {
		return Result{}, err
	}
	if err = e.Guard.Check(ctx, tx, request.TableID); err != nil {
		return Result{}, err
	}
	before, err := e.Metadata.Lock(ctx, tx, request.TableID)
	if err != nil {
		return Result{}, err
	}
	if before.Revision != request.ExpectedRevision {
		return Result{}, ErrRevisionConflict
	}
	if !before.Exists && len(before.Fields) > 0 {
		return Result{}, fmt.Errorf("%w: inconsistent metadata snapshot", ErrInvalid)
	}
	plan, err := BuildPlan(request.TableID, before.Fields, request.Fields)
	if err != nil {
		return Result{}, err
	}
	backfills, err := validatedBackfills(request.Backfills, plan)
	if err != nil {
		return Result{}, err
	}
	qualified := pgx.Identifier{e.Namespace, plan.TableName}.Sanitize()
	if before.Exists && len(plan.Changes) > 0 {
		// ALTER TABLE needs this lock for the supported operations. Taking it
		// before inspecting data closes the confirmation/check-to-DDL window.
		if _, err = tx.Exec(ctx, "LOCK TABLE "+qualified+" IN ACCESS EXCLUSIVE MODE"); err != nil {
			return Result{}, err
		}
		if err = e.checkChanges(ctx, tx, qualified, plan, request.Confirmation, backfills); err != nil {
			return Result{}, err
		}
		for _, change := range plan.Changes {
			if err = executeChange(ctx, tx, qualified, change, backfills); err != nil {
				return Result{}, err
			}
		}
	} else if !before.Exists {
		defs := make([]string, 0, len(plan.Fields))
		for _, field := range plan.Fields {
			defs = append(defs, columnDefinition(field, field.Required))
		}
		if _, err = tx.Exec(ctx, "CREATE TABLE "+qualified+" ("+strings.Join(defs, ",")+")"); err != nil {
			return Result{}, err
		}
	}
	revision, err := e.Metadata.Store(ctx, tx, request.TableID, before, plan.Fields)
	if err != nil {
		return Result{}, err
	}
	return Result{Revision: revision, Plan: plan}, nil
}

func milliseconds(d time.Duration) string { return strconv.FormatInt(d.Milliseconds(), 10) + "ms" }
func validatedBackfills(input map[string]Value, p Plan) (map[string]Value, error) {
	added := make(map[string]Field)
	for _, change := range p.Changes {
		if change.Operation == AddColumn {
			added[change.After.ID] = *change.After
		}
	}
	result := make(map[string]Value, len(input))
	for id, value := range input {
		field, ok := added[id]
		if !ok || field.Default != nil {
			return nil, fmt.Errorf("%w: backfill must target a new field without a default", ErrInvalid)
		}
		if err := validateValue(value, field.Type); err != nil {
			return nil, err
		}
		result[id] = value
	}
	return result, nil
}
func (e Executor) checkChanges(ctx context.Context, tx pgx.Tx, table string, p Plan, confirmation DeletionConfirmation, backfills map[string]Value) error {
	var protected []string
	var drops []Change
	needsOldRowCheck := false
	for _, change := range p.Changes {
		switch change.Operation {
		case DropColumn:
			protected = append(protected, change.Before.ID)
			drops = append(drops, change)
		case AlterType:
			protected = append(protected, change.Before.ID)
		case AddColumn:
			_, hasBackfill := backfills[change.After.ID]
			needsOldRowCheck = needsOldRowCheck || (change.After.Required && change.After.Default == nil && !hasBackfill)
		}
	}
	if len(protected) > 0 {
		if e.Dependencies == nil {
			return ErrDependenciesUnavailable
		}
		refs, err := e.Dependencies.Protect(ctx, tx, p.TableID, protected)
		if err != nil {
			return err
		}
		if len(refs) > 0 {
			return &DependencyError{References: append([]Reference(nil), refs...)}
		}
	}
	if needsOldRowCheck {
		var hasRows bool
		if err := tx.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM "+table+")").Scan(&hasRows); err != nil {
			return err
		}
		if hasRows {
			return ErrRequiredBackfill
		}
	}
	if len(drops) > 0 {
		expressions := make([]string, len(drops))
		counts := make([]int64, len(drops))
		outputs := make([]any, len(drops))
		for i, drop := range drops {
			expressions[i] = "count(" + pgx.Identifier{drop.Column}.Sanitize() + ")"
			outputs[i] = &counts[i]
		}
		if err := tx.QueryRow(ctx, "SELECT "+strings.Join(expressions, ",")+" FROM "+table).Scan(outputs...); err != nil {
			return err
		}
		var impacts []ColumnImpact
		for i, drop := range drops {
			if counts[i] > 0 {
				impacts = append(impacts, ColumnImpact{FieldID: drop.Before.ID, NonNullRows: counts[i]})
			}
		}
		if len(impacts) > 0 {
			if confirmation == nil {
				return &ConfirmationError{Impacts: impacts}
			}
			if err := confirmation.Verify(ctx, tx, p.TableID, append([]ColumnImpact(nil), impacts...)); err != nil {
				return err
			}
		}
	}
	return nil
}
func literal(v Value) string {
	if v.Type == Boolean {
		if v.Boolean {
			return "true"
		}
		return "false"
	}
	// DDL cannot bind a default value. E-literals handle both quote and
	// backslash characters, independent of standard_conforming_strings.
	return "E'" + strings.ReplaceAll(strings.ReplaceAll(v.Text, `\`, `\\`), "'", "''") + "'"
}
func valueArgument(v Value) any {
	if v.Type == Boolean {
		return v.Boolean
	}
	return v.Text
}
func columnDefinition(field Field, required bool) string {
	result := pgx.Identifier{physicalID("f_", field.ID)}.Sanitize() + " " + string(field.Type)
	if field.Default != nil {
		result += " DEFAULT " + literal(*field.Default)
	}
	if required {
		result += " NOT NULL"
	}
	return result
}
func executeChange(ctx context.Context, tx pgx.Tx, table string, change Change, backfills map[string]Value) error {
	column := pgx.Identifier{change.Column}.Sanitize()
	prefix := "ALTER TABLE " + table + " "
	exec := func(sql string) error { _, err := tx.Exec(ctx, sql); return err }
	switch change.Operation {
	case AddColumn:
		value, backfill := backfills[change.After.ID]
		if err := exec(prefix + "ADD COLUMN " + columnDefinition(*change.After, change.After.Required && !backfill)); err != nil {
			return err
		}
		if backfill {
			if _, err := tx.Exec(ctx, "UPDATE "+table+" SET "+column+"=$1", valueArgument(value)); err != nil {
				return err
			}
			if change.After.Required {
				return exec(prefix + "ALTER COLUMN " + column + " SET NOT NULL")
			}
		}
		return nil
	case DropColumn:
		return exec(prefix + "DROP COLUMN " + column + " RESTRICT")
	case AlterType:
		if err := exec(prefix + "ALTER COLUMN " + column + " DROP DEFAULT"); err != nil {
			return err
		}
		return exec(prefix + "ALTER COLUMN " + column + " TYPE " + string(change.After.Type) + " USING " + column + "::" + string(change.After.Type))
	case AlterDefault:
		if change.After.Default == nil {
			return exec(prefix + "ALTER COLUMN " + column + " DROP DEFAULT")
		}
		return exec(prefix + "ALTER COLUMN " + column + " SET DEFAULT " + literal(*change.After.Default))
	case AlterRequired:
		if change.After.Required {
			return exec(prefix + "ALTER COLUMN " + column + " SET NOT NULL")
		}
		return exec(prefix + "ALTER COLUMN " + column + " DROP NOT NULL")
	default:
		return ErrInvalid
	}
}

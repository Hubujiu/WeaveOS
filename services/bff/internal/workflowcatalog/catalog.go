// Package workflowcatalog stores immutable workflow definitions and their
// deployment and instance state. Every operation uses the caller's
// transaction; session and authorization checks belong to the caller.
package workflowcatalog

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/Hubujiu/WeaveOS/services/bff/internal/appquery"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/flowgraph"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var (
	ErrInvalid  = errors.New("invalid workflow catalog request")
	ErrMissing  = errors.New("workflow catalog resource missing")
	ErrConflict = errors.New("workflow catalog version conflict")
	ErrNotReady = errors.New("workflow catalog not ready")
	ErrClosing  = errors.New("workflow catalog closing")
	canonicalID = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
)

const maxSafeInteger int64 = 9007199254740991

type Catalog struct{}

type VersionInput struct {
	AppID, TableID, ViewID, FlowID, ActorID, Name string
	ExpectedRevision, ExpectedSchemaVersion       int64
	Graph                                         flowgraph.Graph
	AllowWithdraw                                 bool
	Triggers                                      *[]Trigger
}

type Head struct {
	FlowID, AppID, TableID, ViewID, Name, State string
	Revision, CurrentVersion, CandidateVersion  int64
}

type ReserveInput struct {
	PreviousInstanceID, RoundKind                string
	AppID, FlowID, InstanceID, RecordID, ActorID string
	ExpectedRevision, ExpectedSchemaVersion      int64
	ExpectedRecordVersion                        int64
}

type Instance struct {
	ID, FlowID, AppID, TableID, ViewID, RecordID, InitiatorID, State string
	DefinitionVersion, Sequence                                      int64
}

type Conflict struct {
	FlowID, NodeID, FieldID, Reason string
	Version                         int64
}

// PublicationVersion is a locked catalog snapshot, never a public DTO. A zero
// requested version selects the current candidate. The caller owns the tx.
type PublicationVersion struct {
	Head                      Head
	CloseEpoch, SchemaVersion int64
	Version, VersionSchema    int64
	VersionID, BPMN           string
	Graph                     flowgraph.Graph
	Compatible                bool
}

func (Catalog) PublicationVersionInTx(ctx context.Context, tx pgx.Tx, appID, flowID string, version int64) (PublicationVersion, error) {
	h, resources, err := lockFlow(ctx, tx, appID, flowID, false)
	if err != nil {
		return PublicationVersion{}, err
	}
	if version == 0 {
		if err := rejectDeletion(ctx, tx, flowID); err != nil {
			return PublicationVersion{}, err
		}
		version = h.CandidateVersion
	}
	v := PublicationVersion{Head: h, SchemaVersion: resources.schemaVersion, Version: version}
	var raw []byte
	if err = tx.QueryRow(ctx, `SELECT close_epoch FROM applications.workflow_definitions WHERE app_id=$1 AND id=$2`, appID, flowID).Scan(&v.CloseEpoch); err != nil {
		return v, err
	}
	if err = tx.QueryRow(ctx, `SELECT version_id::text,schema_version,bpmn_xml,graph_json FROM applications.workflow_versions WHERE app_id=$1 AND flow_id=$2 AND version=$3`, appID, flowID, version).Scan(&v.VersionID, &v.VersionSchema, &v.BPMN, &raw); err != nil {
		return v, mapDBError(err)
	}
	v.Graph, err = decodeGraph(raw)
	if err != nil {
		return v, ErrNotReady
	}
	if resources.ready {
		_, validation := flowgraph.Validate(v.Graph, resources.fields)
		triggerValidation := validateStoredTriggers(ctx, tx, appID, flowID, version, resources.fields)
		if triggerValidation != nil && !errors.Is(triggerValidation, ErrConflict) {
			return v, triggerValidation
		}
		v.Compatible = validation == nil && triggerValidation == nil
	}
	return v, nil
}

type resourceSnapshot struct {
	fields        []appquery.Field
	schemaVersion int64
	ready         bool
}

func (Catalog) PutVersionInTx(ctx context.Context, tx pgx.Tx, in VersionInput) (Head, error) {
	if tx == nil || !validIDs(in.AppID, in.TableID, in.ViewID, in.FlowID, in.ActorID) ||
		strings.TrimSpace(in.Name) == "" || strings.ContainsRune(in.Name, '\x00') || len([]rune(in.Name)) > 100 ||
		in.ExpectedRevision < 0 || in.ExpectedRevision > maxSafeInteger ||
		in.ExpectedSchemaVersion < 1 || in.ExpectedSchemaVersion > maxSafeInteger {
		return Head{}, ErrInvalid
	}
	resources, err := lockResources(ctx, tx, in.AppID, in.TableID, in.ViewID, true)
	if err != nil {
		return Head{}, err
	}
	if resources.schemaVersion != in.ExpectedSchemaVersion {
		return Head{}, ErrConflict
	}

	if err := rejectDeletion(ctx, tx, in.FlowID); err != nil {
		return Head{}, err
	}

	old, err := headForUpdate(ctx, tx, in.AppID, in.FlowID)
	exists := err == nil
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return Head{}, mapDBError(err)
	}
	if exists {
		if old.TableID != in.TableID || old.ViewID != in.ViewID {
			return Head{}, ErrConflict
		}
		if old.Revision != in.ExpectedRevision {
			return Head{}, ErrConflict
		}
		if old.State == "closing" {
			return Head{}, ErrClosing
		}
	} else if in.ExpectedRevision != 0 {
		return Head{}, ErrConflict
	}

	var requested []Trigger
	if in.Triggers != nil {
		requested = *in.Triggers
	} else if exists {
		requested, err = (Catalog{}).TriggersForVersionInTx(ctx, tx, in.AppID, in.FlowID, old.CandidateVersion)
		if err != nil {
			return Head{}, err
		}
	}
	triggers, err := NormalizeTriggers(requested, resources.fields)
	if err != nil {
		return Head{}, ErrInvalid
	}
	triggerJSON, err := json.Marshal(triggers)
	if err != nil {
		return Head{}, ErrInvalid
	}

	if _, err := flowgraph.Validate(in.Graph, resources.fields); err != nil {
		return Head{}, ErrInvalid
	}
	versionID, err := randomID(ctx, tx)
	if err != nil {
		return Head{}, err
	}
	bpmn, err := flowgraph.CompileScopedBPMN(in.Graph, resources.fields, in.AppID, in.FlowID)
	if err != nil || len(bpmn) == 0 {
		return Head{}, ErrInvalid
	}
	graphJSON, err := json.Marshal(in.Graph)
	if err != nil {
		return Head{}, ErrInvalid
	}

	version, err := nextVersion(ctx, tx, in.AppID, in.FlowID)
	if err != nil {
		return Head{}, err
	}
	if version > maxSafeInteger {
		return Head{}, ErrConflict
	}
	if exists {
		if old.Revision >= maxSafeInteger {
			return Head{}, ErrConflict
		}
		_, err = tx.Exec(ctx, `UPDATE applications.workflow_definitions
			SET name=$3, revision=revision+1, candidate_version=$4, updated_at=clock_timestamp()
			WHERE app_id=$1 AND id=$2`, in.AppID, in.FlowID, in.Name, version)
		if err != nil {
			return Head{}, mapDBError(err)
		}
	} else {
		_, err = tx.Exec(ctx, `INSERT INTO applications.workflow_definitions
			(id,app_id,table_id,view_id,name,revision,state,current_version,candidate_version)
			VALUES($1,$2,$3,$4,$5,1,'disabled',0,1)`, in.FlowID, in.AppID, in.TableID, in.ViewID, in.Name)
		if err != nil {
			return Head{}, mapDBError(err)
		}
	}
	_, err = tx.Exec(ctx, `INSERT INTO applications.workflow_versions
		(app_id,flow_id,version,version_id,schema_version,graph_json,bpmn_xml,allow_withdraw,created_by,triggers_json)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`, in.AppID, in.FlowID, version, versionID,
		resources.schemaVersion, graphJSON, string(bpmn), in.AllowWithdraw, in.ActorID, triggerJSON)
	if err != nil {
		return Head{}, mapDBError(err)
	}
	return headForUpdate(ctx, tx, in.AppID, in.FlowID)
}

func (Catalog) ConfirmDeploymentInTx(ctx context.Context, tx pgx.Tx, appID, flowID string, version int64, deploymentID string) (Head, error) {
	if tx == nil || !validIDs(appID, flowID) || version < 1 || version > maxSafeInteger ||
		strings.TrimSpace(deploymentID) == "" || strings.ContainsRune(deploymentID, '\x00') || len(deploymentID) > 200 {
		return Head{}, ErrInvalid
	}
	h, resources, err := lockFlow(ctx, tx, appID, flowID, true)
	if err != nil {
		return Head{}, err
	}
	var versionID string
	var graphJSON []byte
	err = tx.QueryRow(ctx, `SELECT version_id::text,graph_json FROM applications.workflow_versions
		WHERE app_id=$1 AND flow_id=$2 AND version=$3`, appID, flowID, version).Scan(&versionID, &graphJSON)
	if errors.Is(err, pgx.ErrNoRows) {
		return Head{}, ErrMissing
	}
	if err != nil {
		return Head{}, mapDBError(err)
	}
	var prior string
	err = tx.QueryRow(ctx, `SELECT deployment_id FROM applications.workflow_deployments
		WHERE app_id=$1 AND flow_id=$2 AND version=$3`, appID, flowID, version).Scan(&prior)
	if err == nil {
		if prior != deploymentID {
			return Head{}, ErrConflict
		}
		return h, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return Head{}, mapDBError(err)
	}
	graph, err := decodeGraph(graphJSON)
	if err != nil {
		return Head{}, ErrNotReady
	}
	if version == h.CandidateVersion {
		if err := validateStoredTriggers(ctx, tx, appID, flowID, version, resources.fields); err != nil {
			return Head{}, err
		}
		if _, err := flowgraph.Validate(graph, resources.fields); err != nil {
			return Head{}, ErrConflict
		}
	}
	_, err = tx.Exec(ctx, `INSERT INTO applications.workflow_deployments(app_id,flow_id,version,deployment_id)
		VALUES($1,$2,$3,$4)`, appID, flowID, version, deploymentID)
	if err != nil {
		return Head{}, mapDBError(err)
	}
	if version != h.CandidateVersion {
		return h, nil
	}
	if h.Revision >= maxSafeInteger {
		return Head{}, ErrConflict
	}
	_, err = tx.Exec(ctx, `UPDATE applications.workflow_definitions SET current_version=$3,
		revision=revision+1,updated_at=clock_timestamp() WHERE app_id=$1 AND id=$2`, appID, flowID, version)
	if err != nil {
		return Head{}, mapDBError(err)
	}
	return headForUpdate(ctx, tx, appID, flowID)
}

func (Catalog) EnableInTx(ctx context.Context, tx pgx.Tx, appID, flowID string, expectedRevision int64) (Head, error) {
	if tx == nil || !validIDs(appID, flowID) || expectedRevision < 1 || expectedRevision > maxSafeInteger {
		return Head{}, ErrInvalid
	}
	h, resources, err := lockFlow(ctx, tx, appID, flowID, true)
	if err != nil {
		return Head{}, err
	}
	if err := rejectDeletion(ctx, tx, flowID); err != nil {
		return Head{}, err
	}

	if h.Revision != expectedRevision {
		return Head{}, ErrConflict
	}
	if h.CurrentVersion == 0 {
		return Head{}, ErrNotReady
	}
	if h.State != "enabled" && h.CurrentVersion != h.CandidateVersion {
		return Head{}, ErrNotReady
	}
	var deployed bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM applications.workflow_deployments
		WHERE app_id=$1 AND flow_id=$2 AND version=$3)`, appID, flowID, h.CurrentVersion).Scan(&deployed); err != nil {
		return Head{}, mapDBError(err)
	}
	if !deployed {
		return Head{}, ErrNotReady
	}
	if err := validateStoredVersion(ctx, tx, appID, flowID, h.CurrentVersion, resources.fields); err != nil {
		return Head{}, err
	}
	if h.State == "enabled" {
		return h, nil
	}
	if h.Revision >= maxSafeInteger {
		return Head{}, ErrConflict
	}
	_, err = tx.Exec(ctx, `UPDATE applications.workflow_definitions SET state='enabled',revision=revision+1,
		updated_at=clock_timestamp() WHERE app_id=$1 AND id=$2`, appID, flowID)
	if err != nil {
		return Head{}, mapDBError(err)
	}
	return headForUpdate(ctx, tx, appID, flowID)
}

func (Catalog) RequestCloseInTx(ctx context.Context, tx pgx.Tx, appID, flowID string, expectedRevision int64) (Head, error) {
	if tx == nil || !validIDs(appID, flowID) || expectedRevision < 1 || expectedRevision > maxSafeInteger {
		return Head{}, ErrInvalid
	}
	h, _, err := lockFlow(ctx, tx, appID, flowID, false)
	if err != nil {
		return Head{}, err
	}
	if h.Revision != expectedRevision {
		return Head{}, ErrConflict
	}
	if h.State == "closing" || h.State == "disabled" {
		return h, nil
	}
	if h.State != "enabled" {
		return Head{}, ErrConflict
	}
	active, err := hasInFlight(ctx, tx, appID, flowID)
	if err != nil {
		return Head{}, err
	}
	state := "disabled"
	if active {
		state = "closing"
	}
	if h.Revision >= maxSafeInteger {
		return Head{}, ErrConflict
	}
	_, err = tx.Exec(ctx, `UPDATE applications.workflow_definitions SET state=$3,revision=revision+1,
		close_epoch=close_epoch+1,updated_at=clock_timestamp() WHERE app_id=$1 AND id=$2`, appID, flowID, state)
	if err != nil {
		return Head{}, mapDBError(err)
	}
	return headForUpdate(ctx, tx, appID, flowID)
}

func (Catalog) FinalizeCloseInTx(ctx context.Context, tx pgx.Tx, appID, flowID string, expectedRevision int64) (Head, error) {
	if tx == nil || !validIDs(appID, flowID) || expectedRevision < 1 || expectedRevision > maxSafeInteger {
		return Head{}, ErrInvalid
	}
	h, _, err := lockFlow(ctx, tx, appID, flowID, false)
	if err != nil {
		return Head{}, err
	}
	if h.Revision != expectedRevision || h.State != "closing" {
		return Head{}, ErrConflict
	}
	active, err := hasInFlight(ctx, tx, appID, flowID)
	if err != nil {
		return Head{}, err
	}
	if active {
		return h, nil
	}
	if h.Revision >= maxSafeInteger {
		return Head{}, ErrConflict
	}
	_, err = tx.Exec(ctx, `UPDATE applications.workflow_definitions SET state='disabled',revision=revision+1,
		updated_at=clock_timestamp() WHERE app_id=$1 AND id=$2`, appID, flowID)
	if err != nil {
		return Head{}, mapDBError(err)
	}
	return headForUpdate(ctx, tx, appID, flowID)
}

func (Catalog) GetInTx(ctx context.Context, tx pgx.Tx, appID, flowID string) (Head, error) {
	if tx == nil || !validIDs(appID, flowID) {
		return Head{}, ErrInvalid
	}
	h, err := headRead(ctx, tx, appID, flowID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Head{}, ErrMissing
	}
	return h, mapDBError(err)
}

func (Catalog) ReserveInTx(ctx context.Context, tx pgx.Tx, in ReserveInput) (Instance, error) {
	if tx == nil || !validIDs(in.AppID, in.FlowID, in.InstanceID, in.RecordID, in.ActorID) ||
		in.ExpectedRevision < 1 || in.ExpectedRevision > maxSafeInteger ||
		in.ExpectedSchemaVersion < 1 || in.ExpectedSchemaVersion > maxSafeInteger ||
		in.ExpectedRecordVersion < 1 || in.ExpectedRecordVersion > maxSafeInteger {
		return Instance{}, ErrInvalid
	}
	if in.RoundKind == "" {
		in.RoundKind = "initial"
	}
	if in.RoundKind != "initial" && in.RoundKind != "resubmit" && in.RoundKind != "review" ||
		in.RoundKind == "initial" && in.PreviousInstanceID != "" ||
		in.RoundKind != "initial" && !validIDs(in.PreviousInstanceID) {
		return Instance{}, ErrInvalid
	}
	h, resources, err := lockFlow(ctx, tx, in.AppID, in.FlowID, true)
	if err != nil {
		return Instance{}, err
	}
	var existing Instance
	var previousID *string
	var roundKind string
	err = tx.QueryRow(ctx, `SELECT id::text,flow_id::text,app_id::text,table_id::text,view_id::text,
		record_id::text,initiator_id::text,state,definition_version,sequence,previous_instance_id::text,round_kind
		FROM applications.workflow_instances WHERE app_id=$1 AND flow_id=$2 AND id=$3`,
		in.AppID, in.FlowID, in.InstanceID).Scan(&existing.ID, &existing.FlowID, &existing.AppID,
		&existing.TableID, &existing.ViewID, &existing.RecordID, &existing.InitiatorID,
		&existing.State, &existing.DefinitionVersion, &existing.Sequence, &previousID, &roundKind)
	if err == nil {
		if existing.AppID != in.AppID || existing.FlowID != in.FlowID || existing.TableID != h.TableID ||
			existing.ViewID != h.ViewID || existing.RecordID != in.RecordID || existing.InitiatorID != in.ActorID || roundKind != in.RoundKind || (previousID == nil && in.PreviousInstanceID != "") || (previousID != nil && *previousID != in.PreviousInstanceID) {
			return Instance{}, ErrConflict
		}
		return existing, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return Instance{}, mapDBError(err)
	}
	if h.State == "closing" {
		return Instance{}, ErrClosing
	}
	if h.State != "enabled" {
		return Instance{}, ErrNotReady
	}
	if h.Revision != in.ExpectedRevision || resources.schemaVersion != in.ExpectedSchemaVersion {
		return Instance{}, ErrConflict
	}
	if h.CurrentVersion < 1 {
		return Instance{}, ErrNotReady
	}
	var deploymentID string
	err = tx.QueryRow(ctx, `SELECT deployment_id FROM applications.workflow_deployments
		WHERE app_id=$1 AND flow_id=$2 AND version=$3`, in.AppID, in.FlowID, h.CurrentVersion).Scan(&deploymentID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Instance{}, ErrNotReady
	}
	if err != nil {
		return Instance{}, mapDBError(err)
	}
	recordVersion, err := loadRecordVersion(ctx, tx, h.TableID, in.RecordID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Instance{}, ErrMissing
	}
	if err != nil {
		return Instance{}, mapDBError(err)
	}
	if recordVersion != in.ExpectedRecordVersion {
		return Instance{}, ErrConflict
	}
	_, err = tx.Exec(ctx, `INSERT INTO applications.workflow_instances
		(id,app_id,flow_id,table_id,view_id,record_id,initiator_id,definition_version,state,sequence,previous_instance_id,round_kind)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,'starting',0,NULLIF($9,'')::uuid,$10)`, in.InstanceID, in.AppID, in.FlowID,
		h.TableID, h.ViewID, in.RecordID, in.ActorID, h.CurrentVersion, in.PreviousInstanceID, in.RoundKind)
	if err != nil {
		return Instance{}, mapDBError(err)
	}
	return Instance{ID: in.InstanceID, FlowID: in.FlowID, AppID: in.AppID, TableID: h.TableID,
		ViewID: h.ViewID, RecordID: in.RecordID, InitiatorID: in.ActorID, State: "starting",
		DefinitionVersion: h.CurrentVersion, Sequence: 0}, nil
}

func (Catalog) CheckCompatibilityInTx(ctx context.Context, tx pgx.Tx, appID, tableID string, proposed []appquery.Field) ([]Conflict, error) {
	if tx == nil || !validIDs(appID, tableID) || !validFieldContext(proposed) {
		return nil, ErrInvalid
	}
	var exists bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM applications.logical_tables WHERE app_id=$1 AND id=$2 AND deleted_at IS NULL)`,
		appID, tableID).Scan(&exists); err != nil {
		return nil, mapDBError(err)
	}
	if !exists {
		return nil, ErrMissing
	}
	rows, err := tx.Query(ctx, `WITH live_versions AS (
			SELECT d.app_id,d.id AS flow_id,d.current_version AS version
			FROM applications.workflow_definitions d
			WHERE d.app_id=$1 AND d.table_id=$2 AND d.state IN ('enabled','closing')
			UNION
			SELECT i.app_id,i.flow_id,i.definition_version AS version
			FROM applications.workflow_instances i
			WHERE i.app_id=$1 AND i.table_id=$2 AND i.state IN ('starting','active')
		)
		SELECT live.flow_id::text,v.version,v.graph_json,v.triggers_json,
        EXISTS(SELECT 1 FROM applications.workflow_definitions d WHERE d.app_id=live.app_id
          AND d.id=live.flow_id AND d.current_version=live.version AND d.state IN ('enabled','closing'))
		FROM live_versions live
		JOIN applications.workflow_versions v
			ON v.app_id=live.app_id AND v.flow_id=live.flow_id AND v.version=live.version
		ORDER BY 1,2`, appID, tableID)
	if err != nil {
		return nil, mapDBError(err)
	}
	defer rows.Close()
	conflicts := make([]Conflict, 0)
	for rows.Next() {
		var flowID string
		var version int64
		var graphJSON, triggerJSON []byte
		var checkTriggers bool
		if err := rows.Scan(&flowID, &version, &graphJSON, &triggerJSON, &checkTriggers); err != nil {
			return nil, mapDBError(err)
		}
		graph, err := decodeGraph(graphJSON)
		if err != nil {
			return nil, ErrNotReady
		}
		conflicts = append(conflicts, graphConflicts(flowID, version, graph, proposed)...)
		if checkTriggers {
			triggers, err := decodeTriggers(triggerJSON)
			if err != nil {
				return nil, err
			}
			conflicts = append(conflicts, triggerConflicts(flowID, version, graph, triggers, proposed)...)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, mapDBError(err)
	}
	sort.Slice(conflicts, func(i, j int) bool {
		a, b := conflicts[i], conflicts[j]
		if a.FlowID != b.FlowID {
			return a.FlowID < b.FlowID
		}
		if a.NodeID != b.NodeID {
			return a.NodeID < b.NodeID
		}
		if a.FieldID != b.FieldID {
			return a.FieldID < b.FieldID
		}
		if a.Version != b.Version {
			return a.Version < b.Version
		}
		return a.Reason < b.Reason
	})
	return conflicts, nil
}

func lockResources(ctx context.Context, tx pgx.Tx, appID, tableID, viewID string, requireReady bool) (resourceSnapshot, error) {
	var id string
	err := tx.QueryRow(ctx, `SELECT id::text FROM applications.apps WHERE id=$1 FOR UPDATE`, appID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return resourceSnapshot{}, ErrMissing
	}
	if err != nil {
		return resourceSnapshot{}, mapDBError(err)
	}
	var snapshot resourceSnapshot
	var fieldsJSON []byte
	err = tx.QueryRow(ctx, `SELECT schema_version,schema_ready,fields_json FROM applications.logical_tables
		WHERE app_id=$1 AND id=$2 AND deleted_at IS NULL FOR UPDATE`, appID, tableID).Scan(&snapshot.schemaVersion, &snapshot.ready, &fieldsJSON)
	if errors.Is(err, pgx.ErrNoRows) {
		return resourceSnapshot{}, ErrMissing
	}
	if err != nil {
		return resourceSnapshot{}, mapDBError(err)
	}
	var viewExists bool
	err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM applications.form_views WHERE app_id=$1 AND table_id=$2 AND id=$3 AND deleted_at IS NULL)`,
		appID, tableID, viewID).Scan(&viewExists)
	if err != nil {
		return resourceSnapshot{}, mapDBError(err)
	}
	if !viewExists {
		return resourceSnapshot{}, ErrMissing
	}
	if !snapshot.ready {
		if requireReady {
			return resourceSnapshot{}, ErrNotReady
		}
		return snapshot, nil
	}
	snapshot.fields, err = parseFields(fieldsJSON)
	if err != nil {
		return resourceSnapshot{}, ErrNotReady
	}
	return snapshot, nil
}

func lockFlow(ctx context.Context, tx pgx.Tx, appID, flowID string, requireReady bool) (Head, resourceSnapshot, error) {
	var tableID, viewID string
	err := tx.QueryRow(ctx, `SELECT table_id::text,view_id::text FROM applications.workflow_definitions WHERE app_id=$1 AND id=$2`,
		appID, flowID).Scan(&tableID, &viewID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Head{}, resourceSnapshot{}, ErrMissing
	}
	if err != nil {
		return Head{}, resourceSnapshot{}, mapDBError(err)
	}
	resources, err := lockResources(ctx, tx, appID, tableID, viewID, requireReady)
	if err != nil {
		return Head{}, resourceSnapshot{}, err
	}
	h, err := headForUpdate(ctx, tx, appID, flowID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Head{}, resourceSnapshot{}, ErrMissing
	}
	if err != nil {
		return Head{}, resourceSnapshot{}, mapDBError(err)
	}
	if h.TableID != tableID || h.ViewID != viewID {
		return Head{}, resourceSnapshot{}, ErrConflict
	}
	return h, resources, nil
}

func headForUpdate(ctx context.Context, tx pgx.Tx, appID, flowID string) (Head, error) {
	var h Head
	err := tx.QueryRow(ctx, `SELECT id::text,app_id::text,table_id::text,view_id::text,name,state,
		revision,current_version,candidate_version FROM applications.workflow_definitions
		WHERE app_id=$1 AND id=$2 FOR UPDATE`, appID, flowID).Scan(&h.FlowID, &h.AppID,
		&h.TableID, &h.ViewID, &h.Name, &h.State, &h.Revision, &h.CurrentVersion, &h.CandidateVersion)
	return h, err
}

func headRead(ctx context.Context, tx pgx.Tx, appID, flowID string) (Head, error) {
	var h Head
	err := tx.QueryRow(ctx, `SELECT id::text,app_id::text,table_id::text,view_id::text,name,state,
		revision,current_version,candidate_version FROM applications.workflow_definitions
		WHERE app_id=$1 AND id=$2`, appID, flowID).Scan(&h.FlowID, &h.AppID,
		&h.TableID, &h.ViewID, &h.Name, &h.State, &h.Revision, &h.CurrentVersion, &h.CandidateVersion)
	return h, err
}

func randomID(ctx context.Context, tx pgx.Tx) (string, error) {
	var id string
	if err := tx.QueryRow(ctx, `SELECT gen_random_uuid()::text`).Scan(&id); err != nil {
		return "", mapDBError(err)
	}
	return id, nil
}

func nextVersion(ctx context.Context, tx pgx.Tx, appID, flowID string) (int64, error) {
	var version int64
	if err := tx.QueryRow(ctx, `SELECT COALESCE(max(version),0)+1 FROM applications.workflow_versions WHERE app_id=$1 AND flow_id=$2`,
		appID, flowID).Scan(&version); err != nil {
		return 0, mapDBError(err)
	}
	return version, nil
}

func validateStoredVersion(ctx context.Context, tx pgx.Tx, appID, flowID string, version int64, fields []appquery.Field) error {
	var graphJSON []byte
	err := tx.QueryRow(ctx, `SELECT graph_json FROM applications.workflow_versions WHERE app_id=$1 AND flow_id=$2 AND version=$3`,
		appID, flowID, version).Scan(&graphJSON)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotReady
	}
	if err != nil {
		return mapDBError(err)
	}
	graph, err := decodeGraph(graphJSON)
	if err != nil {
		return ErrNotReady
	}
	if _, err := flowgraph.Validate(graph, fields); err != nil {
		return ErrConflict
	}
	return validateStoredTriggers(ctx, tx, appID, flowID, version, fields)
}

func hasInFlight(ctx context.Context, tx pgx.Tx, appID, flowID string) (bool, error) {
	var active bool
	err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM applications.workflow_instances
		WHERE app_id=$1 AND flow_id=$2 AND state IN ('starting','active'))`, appID, flowID).Scan(&active)
	return active, mapDBError(err)
}

func loadRecordVersion(ctx context.Context, tx pgx.Tx, tableID, recordID string) (int64, error) {
	table := pgx.Identifier{"appdata", "t_" + strings.ReplaceAll(tableID, "-", "")}.Sanitize()
	var version int64
	err := tx.QueryRow(ctx, `SELECT record_version FROM `+table+` WHERE id=$1`, recordID).Scan(&version)
	return version, err
}

func parseFields(raw []byte) ([]appquery.Field, error) {
	var encoded []struct {
		ID   string `json:"id"`
		Kind string `json:"kind"`
	}
	if err := json.Unmarshal(raw, &encoded); err != nil {
		return nil, err
	}
	fields := make([]appquery.Field, 0, len(encoded))
	for _, field := range encoded {
		fields = append(fields, appquery.Field{ID: field.ID, Kind: appquery.FieldKind(field.Kind)})
	}
	if !validFieldContext(fields) {
		return nil, ErrInvalid
	}
	return fields, nil
}

func validFieldContext(fields []appquery.Field) bool {
	if len(fields) > 200 {
		return false
	}
	seen := make(map[string]bool, len(fields))
	for _, field := range fields {
		if !validIDs(field.ID) || seen[field.ID] || !validFieldKind(field.Kind) {
			return false
		}
		seen[field.ID] = true
	}
	return true
}

func validFieldKind(kind appquery.FieldKind) bool {
	switch kind {
	case appquery.Text, appquery.Multiline, appquery.Number, appquery.Money, appquery.Date,
		appquery.Datetime, appquery.Boolean, appquery.SingleSelect, appquery.MultiSelect,
		appquery.Member, appquery.Department:
		return true
	default:
		return false
	}
}

func graphConflicts(flowID string, version int64, graph flowgraph.Graph, proposed []appquery.Field) []Conflict {
	byID := make(map[string]appquery.FieldKind, len(proposed))
	for _, field := range proposed {
		byID[field.ID] = field.Kind
	}
	conflicts := make([]Conflict, 0)
	for _, node := range graph.Nodes {
		if node.Kind == "approval" && node.Approval != nil {
			for _, fieldID := range node.Approval.EditableFieldIDs {
				if _, ok := byID[fieldID]; !ok {
					conflicts = append(conflicts, Conflict{FlowID: flowID, NodeID: node.ID, FieldID: fieldID,
						Reason: "editable_field_removed", Version: version})
				}
			}
		}
		if node.Kind != "condition" || len(node.Condition) == 0 {
			continue
		}
		for _, leaf := range conditionLeaves(node.Condition) {
			var value struct {
				FieldID string `json:"fieldId"`
			}
			if json.Unmarshal(leaf, &value) != nil || value.FieldID == "" {
				continue
			}
			wrapped, _ := json.Marshal(map[string]any{"operator": "and", "children": []json.RawMessage{leaf}})
			if _, err := appquery.Compile(wrapped, nil, proposed, 1); err == nil {
				continue
			}
			reason := "condition_type_incompatible"
			if _, exists := byID[value.FieldID]; !exists {
				reason = "condition_field_removed"
			}
			conflicts = append(conflicts, Conflict{FlowID: flowID, NodeID: node.ID, FieldID: value.FieldID,
				Reason: reason, Version: version})
		}
	}
	return conflicts
}

func conditionLeaves(raw json.RawMessage) []json.RawMessage {
	var object map[string]json.RawMessage
	if json.Unmarshal(raw, &object) != nil {
		return nil
	}
	childrenRaw, ok := object["children"]
	if !ok {
		return []json.RawMessage{append(json.RawMessage(nil), raw...)}
	}
	var children []json.RawMessage
	if json.Unmarshal(childrenRaw, &children) != nil {
		return nil
	}
	leaves := make([]json.RawMessage, 0, len(children))
	for _, child := range children {
		leaves = append(leaves, conditionLeaves(child)...)
	}
	return leaves
}

func decodeGraph(raw []byte) (flowgraph.Graph, error) {
	var graph flowgraph.Graph
	if err := json.Unmarshal(raw, &graph); err != nil {
		return flowgraph.Graph{}, err
	}
	for i := range graph.Nodes {
		if bytes.Equal(bytes.TrimSpace(graph.Nodes[i].Condition), []byte("null")) {
			graph.Nodes[i].Condition = nil
		}
	}
	return graph, nil
}

func validIDs(ids ...string) bool {
	for _, id := range ids {
		if !canonicalID.MatchString(id) {
			return false
		}
	}
	return true
}

func mapDBError(err error) error {
	if err == nil {
		return nil
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505", "40001":
			return fmt.Errorf("%w: %s", ErrConflict, pgErr.Message)
		case "23503":
			return fmt.Errorf("%w: %s", ErrMissing, pgErr.Message)
		case "23514", "22001", "22P02":
			return fmt.Errorf("%w: %s", ErrInvalid, pgErr.Message)
		}
	}
	return err
}

// conditionLeaves and the compiler together keep compatibility decisions tied
// to the same supported condition language used when the graph was authored.

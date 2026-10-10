package appworkflows

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/Hubujiu/WeaveOS/services/bff/internal/applications"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/apppolicy"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appschema"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/flowgraph"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/platform/httpserver"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/workflowcatalog"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Application struct {
	RuntimeReady     func(context.Context) error
	Pool             *pgxpool.Pool
	Limits           appschema.Limits
	DeploymentClient DeploymentClient
	DeletionClient   DeletionClient
}

type Service struct {
	Application       *Application
	Authenticator     session.Authenticator
	TrustedProxyHosts []string
}

const maxSafeInteger int64 = 9007199254740991

var canonicalID = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

var (
	errInvalid      = errors.New("invalid workflow request")
	errApprover     = errors.New("workflow approver is not currently eligible")
	errWorkflowMiss = errors.New("workflow not found")
)

type workflowGraph struct {
	Version int            `json:"version"`
	Nodes   []workflowNode `json:"nodes"`
	Edges   []workflowEdge `json:"edges"`
}

type workflowNode struct {
	ID        string            `json:"id"`
	Kind      string            `json:"kind"`
	Approval  *workflowApproval `json:"approval,omitempty"`
	Condition json.RawMessage   `json:"condition,omitempty"`
}

type workflowApproval struct {
	Mode             string   `json:"mode"`
	AssigneeIDs      []string `json:"assigneeIds"`
	EditableFieldIDs []string `json:"editableFieldIds"`
}

type workflowEdge struct {
	From   string `json:"from"`
	To     string `json:"to"`
	Branch string `json:"branch,omitempty"`
}

type definitionRequest struct {
	OperationID           string                     `json:"operationId"`
	Name                  string                     `json:"name"`
	ExpectedRevision      int64                      `json:"expectedRevision"`
	ExpectedSchemaVersion int64                      `json:"expectedSchemaVersion"`
	Graph                 json.RawMessage            `json:"graph"`
	AllowWithdraw         *bool                      `json:"allowWithdraw"`
	Triggers              *[]workflowcatalog.Trigger `json:"triggers,omitempty"`
}

type lifecycleRequest struct {
	OperationID      string `json:"operationId"`
	ExpectedRevision int64  `json:"expectedRevision"`
}

type definitionView struct {
	ID               string                    `json:"id"`
	Name             string                    `json:"name"`
	Revision         int64                     `json:"revision"`
	CurrentVersion   int64                     `json:"currentVersion"`
	CandidateVersion int64                     `json:"candidateVersion"`
	State            string                    `json:"state"`
	SchemaVersion    int64                     `json:"schemaVersion"`
	AllowWithdraw    bool                      `json:"allowWithdraw"`
	Graph            workflowGraph             `json:"graph"`
	Triggers         []workflowcatalog.Trigger `json:"triggers"`
}

type mutationData struct {
	OperationID      string `json:"operationId"`
	ID               string `json:"id"`
	Revision         int64  `json:"revision"`
	CurrentVersion   int64  `json:"currentVersion"`
	CandidateVersion int64  `json:"candidateVersion"`
	State            string `json:"state"`
}

func validID(id string) bool {
	return canonicalID.MatchString(id) && id != "00000000-0000-0000-0000-000000000000"
}

func writeEnvelope(w http.ResponseWriter, r *http.Request, status int, code string, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	if r.Method == http.MethodHead {
		return
	}
	message := "请求未完成"
	if code == "OK" {
		message = "success"
	}
	_ = json.NewEncoder(w).Encode(struct {
		Code    string         `json:"code"`
		Message string         `json:"message"`
		Data    any            `json:"data"`
		Meta    map[string]any `json:"meta"`
	}{code, message, value, map[string]any{"requestId": httpserver.Metadata(r.Context()).RequestID}})
}

func fail(w http.ResponseWriter, r *http.Request, err error, operationID string) {
	status, code, data := http.StatusServiceUnavailable, "COMMON_SERVICE_UNAVAILABLE", any(nil)
	switch {
	case errors.Is(err, errInvalid), errors.Is(err, applications.ErrInvalid):
		status, code = http.StatusBadRequest, "COMMON_INVALID_ARGUMENT"
	case errors.Is(err, applications.ErrExpectedActorInvalid):
		status, code, data = http.StatusBadRequest, "COMMON_VALIDATION_FAILED", applications.ActorHeaderViolation()
	case errors.Is(err, applications.ErrSessionChanged):
		status, code = http.StatusConflict, "AUTH_SESSION_CHANGED"
	case errors.Is(err, session.ErrUnauthorized):
		status, code = http.StatusUnauthorized, "AUTH_UNAUTHENTICATED"
		w.Header().Set("WWW-Authenticate", `Session realm="enterprise-management-system"`)
	case errors.Is(err, session.ErrForbidden):
		status, code = http.StatusForbidden, "COMMON_CSRF_REJECTED"
	case errors.Is(err, applications.ErrDenied):
		status, code = http.StatusForbidden, "APPLICATION_FORBIDDEN"
	case errors.Is(err, errApprover):
		status, code = http.StatusForbidden, "WORKFLOW_APPROVER_FORBIDDEN"
	case errors.Is(err, applications.ErrMissing), errors.Is(err, errWorkflowMiss), errors.Is(err, workflowcatalog.ErrMissing):
		status, code = http.StatusNotFound, "APPLICATION_NOT_FOUND"
	case errors.Is(err, applications.ErrOperationConflict):
		status, code = http.StatusConflict, "APPLICATION_OPERATION_CONFLICT"
	case errors.Is(err, applications.ErrUnconfirmed):
		code, data = "APPLICATION_OPERATION_UNCONFIRMED", map[string]string{"operationId": operationID}
	case errors.Is(err, workflowcatalog.ErrConflict):
		status, code = http.StatusConflict, "WORKFLOW_CONFLICT"
	case errors.Is(err, workflowcatalog.ErrNotReady):
		status, code = http.StatusConflict, "WORKFLOW_NOT_READY"
	case errors.Is(err, workflowcatalog.ErrClosing):
		status, code = http.StatusConflict, "WORKFLOW_CLOSING"
	case errors.Is(err, workflowcatalog.ErrInvalid):
		status, code = http.StatusBadRequest, "COMMON_INVALID_ARGUMENT"
	}
	writeEnvelope(w, r, status, code, data)
}

// jsonValue walks every nested object so encoding/json cannot silently accept
// duplicate keys before the strict DTO decoder sees them.
func jsonValue(d *json.Decoder) error {
	token, err := d.Token()
	if err != nil {
		return err
	}
	delim, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delim {
	case '{':
		seen := map[string]bool{}
		for d.More() {
			key, err := d.Token()
			name, ok := key.(string)
			if err != nil || !ok || seen[name] {
				return errInvalid
			}
			seen[name] = true
			if err = jsonValue(d); err != nil {
				return err
			}
		}
	case '[':
		for d.More() {
			if err := jsonValue(d); err != nil {
				return err
			}
		}
	default:
		return errInvalid
	}
	end, err := d.Token()
	if err != nil || delim == '{' && end != json.Delim('}') || delim == '[' && end != json.Delim(']') {
		return errInvalid
	}
	return nil
}

func exactObject(raw []byte, required, optional []string) (map[string]json.RawMessage, error) {
	var object map[string]json.RawMessage
	if json.Unmarshal(raw, &object) != nil || object == nil {
		return nil, errInvalid
	}
	allowed := make(map[string]bool, len(required)+len(optional))
	for _, key := range required {
		allowed[key] = true
		value, exists := object[key]
		if !exists || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return nil, errInvalid
		}
	}
	for _, key := range optional {
		allowed[key] = true
	}
	for key, value := range object {
		if !allowed[key] || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return nil, errInvalid
		}
	}
	return object, nil
}

func strictJSON(w http.ResponseWriter, r *http.Request) ([]byte, error) {
	media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || media != "application/json" {
		return nil, errUnsupportedMediaType
	}
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err != nil {
		return nil, errInvalid
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	if err = jsonValue(decoder); err != nil {
		return nil, errInvalid
	}
	if _, err = decoder.Token(); err != io.EOF {
		return nil, errInvalid
	}
	return raw, nil
}

var errUnsupportedMediaType = errors.New("unsupported media type")

func decodeGraph(raw []byte) (flowgraph.Graph, error) {
	outer, err := exactObject(raw, []string{"version", "nodes", "edges"}, nil)
	if err != nil {
		return flowgraph.Graph{}, err
	}
	var graph workflowGraph
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&graph) != nil || graph.Nodes == nil || graph.Edges == nil {
		return flowgraph.Graph{}, errInvalid
	}
	var version int64
	if json.Unmarshal(outer["version"], &version) != nil || version != 1 {
		return flowgraph.Graph{}, errInvalid
	}
	graph.Version = int(version)
	var rawNodes []json.RawMessage
	if json.Unmarshal(outer["nodes"], &rawNodes) != nil || rawNodes == nil || len(rawNodes) != len(graph.Nodes) {
		return flowgraph.Graph{}, errInvalid
	}
	nodes := make([]flowgraph.Node, len(graph.Nodes))
	for i, node := range graph.Nodes {
		kindFields := map[string][]string{
			"start":     {"id", "kind"},
			"end":       {"id", "kind"},
			"approval":  {"id", "kind", "approval"},
			"condition": {"id", "kind", "condition"},
		}
		fields, ok := kindFields[node.Kind]
		if !ok {
			return flowgraph.Graph{}, errInvalid
		}
		nodeObject, e := exactObject(rawNodes[i], fields, nil)
		if e != nil {
			return flowgraph.Graph{}, e
		}
		switch node.Kind {
		case "start", "end":
			nodes[i] = flowgraph.Node{ID: node.ID, Kind: node.Kind}
		case "approval":
			_, e := exactObject(nodeObject["approval"], []string{"mode", "assigneeIds", "editableFieldIds"}, nil)
			if e != nil {
				return flowgraph.Graph{}, e
			}
			var a workflowApproval
			if json.Unmarshal(nodeObject["approval"], &a) != nil || a.AssigneeIDs == nil || a.EditableFieldIDs == nil {
				return flowgraph.Graph{}, errInvalid
			}
			for _, id := range a.AssigneeIDs {
				if !validID(id) {
					return flowgraph.Graph{}, errInvalid
				}
			}
			nodes[i] = flowgraph.Node{ID: node.ID, Kind: node.Kind, Approval: &flowgraph.Approval{Mode: a.Mode, AssigneeIDs: a.AssigneeIDs, EditableFieldIDs: a.EditableFieldIDs}}
		case "condition":
			if len(bytes.TrimSpace(nodeObject["condition"])) == 0 {
				return flowgraph.Graph{}, errInvalid
			}
			nodes[i] = flowgraph.Node{ID: node.ID, Kind: node.Kind, Condition: nodeObject["condition"]}
		}
	}
	edges := make([]flowgraph.Edge, len(graph.Edges))
	var rawEdges []json.RawMessage
	if json.Unmarshal(outer["edges"], &rawEdges) != nil || len(rawEdges) != len(graph.Edges) {
		return flowgraph.Graph{}, errInvalid
	}
	for i, edge := range graph.Edges {
		if _, e := exactObject(rawEdges[i], []string{"from", "to"}, []string{"branch"}); e != nil {
			return flowgraph.Graph{}, e
		}
		edges[i] = flowgraph.Edge{From: edge.From, To: edge.To, Branch: edge.Branch}
	}
	return flowgraph.Graph{Version: graph.Version, Nodes: nodes, Edges: edges}, nil
}

func decodeDefinition(w http.ResponseWriter, r *http.Request) (definitionRequest, flowgraph.Graph, error) {
	raw, err := strictJSON(w, r)
	if err != nil {
		return definitionRequest{}, flowgraph.Graph{}, err
	}
	fields, err := exactObject(raw, []string{"operationId", "name", "expectedRevision", "expectedSchemaVersion", "graph", "allowWithdraw"}, []string{"triggers"})
	if err != nil {
		return definitionRequest{}, flowgraph.Graph{}, err
	}
	var in definitionRequest
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	decodeErr := decoder.Decode(&in)
	if decodeErr != nil || in.AllowWithdraw == nil || !validID(in.OperationID) || strings.TrimSpace(in.Name) == "" || in.ExpectedRevision < 0 || in.ExpectedRevision > maxSafeInteger || in.ExpectedSchemaVersion < 1 || in.ExpectedSchemaVersion > maxSafeInteger {
		return definitionRequest{}, flowgraph.Graph{}, errInvalid
	}
	if rawTriggers, exists := fields["triggers"]; exists {
		var entries []map[string]json.RawMessage
		if json.Unmarshal(rawTriggers, &entries) != nil || entries == nil || len(entries) > 3 {
			return definitionRequest{}, flowgraph.Graph{}, errInvalid
		}
		for _, entry := range entries {
			_, hasEvent := entry["event"]
			_, hasCondition := entry["condition"]
			if len(entry) != 2 || !hasEvent || !hasCondition {
				return definitionRequest{}, flowgraph.Graph{}, errInvalid
			}
		}
	}
	graph, err := decodeGraph(fields["graph"])
	if err != nil {
		return definitionRequest{}, flowgraph.Graph{}, err
	}
	return in, graph, nil
}

func decodeLifecycle(w http.ResponseWriter, r *http.Request) (lifecycleRequest, error) {
	raw, err := strictJSON(w, r)
	if err != nil {
		return lifecycleRequest{}, err
	}
	if _, err = exactObject(raw, []string{"operationId", "expectedRevision"}, nil); err != nil {
		return lifecycleRequest{}, err
	}
	var in lifecycleRequest
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&in) != nil || !validID(in.OperationID) || in.ExpectedRevision < 1 || in.ExpectedRevision > maxSafeInteger {
		return lifecycleRequest{}, errInvalid
	}
	return in, nil
}

func toGraphDTO(graph flowgraph.Graph) workflowGraph {
	out := workflowGraph{Version: graph.Version, Nodes: make([]workflowNode, len(graph.Nodes)), Edges: make([]workflowEdge, len(graph.Edges))}
	for i, node := range graph.Nodes {
		out.Nodes[i] = workflowNode{ID: node.ID, Kind: node.Kind}
		if node.Approval != nil {
			out.Nodes[i].Approval = &workflowApproval{Mode: node.Approval.Mode, AssigneeIDs: node.Approval.AssigneeIDs, EditableFieldIDs: node.Approval.EditableFieldIDs}
		}
		if node.Kind == "condition" {
			out.Nodes[i].Condition = node.Condition
		}
	}
	for i, edge := range graph.Edges {
		out.Edges[i] = workflowEdge{From: edge.From, To: edge.To, Branch: edge.Branch}
	}
	return out
}

func (s *Service) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	r, err := httpserver.Prepare(w, r, s.TrustedProxyHosts)
	if err != nil {
		fail(w, r, err, "")
		return
	}
	p, err := s.Authenticator.Authenticate(r, r.Method != http.MethodGet)
	if err != nil {
		fail(w, r, err, "")
		return
	}
	if err = applications.ExpectedActor(r, p); err != nil {
		fail(w, r, err, "")
		return
	}
	if s.Application == nil || s.Application.Pool == nil {
		fail(w, r, session.ErrUnavailable, "")
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/applications/")
	parts := strings.Split(path, "/")
	if path == r.URL.Path || (len(parts) != 6 && len(parts) != 7) || parts[1] != "forms" || parts[3] != "workflows" {
		writeEnvelope(w, r, http.StatusNotFound, "API_NOT_FOUND", nil)
		return
	}
	appID, viewID, flowID, action := parts[0], parts[2], parts[4], parts[5]
	if !validID(appID) || !validID(viewID) || !validID(flowID) {
		fail(w, r, errInvalid, "")
		return
	}
	if r.URL.RawQuery != "" || r.URL.ForceQuery {
		fail(w, r, errInvalid, "")
		return
	}
	switch {
	case r.Method == http.MethodGet && len(parts) == 7 && action == "deletions":
		s.deletionStatus(w, r, p, appID, viewID, flowID, parts[6])
	case r.Method == http.MethodGet && len(parts) == 7 && action == "publications":
		s.publicationStatus(w, r, p, appID, viewID, flowID, parts[6])
	case len(parts) != 6:
		writeEnvelope(w, r, http.StatusNotFound, "API_NOT_FOUND", nil)
	case r.Method == http.MethodPost && action == "delete":
		s.deleteWorkflow(w, r, p, appID, viewID, flowID)
	case r.Method == http.MethodPost && action == "publish":
		s.publish(w, r, p, appID, viewID, flowID)
	case r.Method == http.MethodGet && action == "definition":
		s.getDefinition(w, r, p, appID, viewID, flowID)
	case r.Method == http.MethodPut && action == "definition":
		in, graph, e := decodeDefinition(w, r)
		if e != nil {
			if errors.Is(e, errUnsupportedMediaType) {
				writeEnvelope(w, r, http.StatusUnsupportedMediaType, "COMMON_UNSUPPORTED_MEDIA_TYPE", nil)
			} else {
				fail(w, r, e, "")
			}
			return
		}
		s.mutate(w, r, p, appID, viewID, flowID, "save", in, graph)
	case (r.Method == http.MethodPost && action == "enable") || (r.Method == http.MethodPost && action == "close"):
		in, e := decodeLifecycle(w, r)
		if e != nil {
			if errors.Is(e, errUnsupportedMediaType) {
				writeEnvelope(w, r, http.StatusUnsupportedMediaType, "COMMON_UNSUPPORTED_MEDIA_TYPE", nil)
			} else {
				fail(w, r, e, "")
			}
			return
		}
		operation := "enable"
		if action == "close" {
			operation = "close"
		}
		s.mutate(w, r, p, appID, viewID, flowID, operation, in, flowgraph.Graph{})
	default:
		writeEnvelope(w, r, http.StatusNotFound, "API_NOT_FOUND", nil)
	}
}

func (s *Service) getDefinition(w http.ResponseWriter, r *http.Request, p session.Principal, appID, viewID, flowID string) {
	ctx := r.Context()
	tx, err := s.Application.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		fail(w, r, err, "")
		return
	}
	defer tx.Rollback(context.Background())
	var status, authVersion string
	var bootstrap bool
	err = tx.QueryRow(ctx, "SELECT status,auth_version::text,is_bootstrap_admin FROM auth.users WHERE id=$1", p.UserID).Scan(&status, &authVersion, &bootstrap)
	if errors.Is(err, pgx.ErrNoRows) || err == nil && (status != "active" || authVersion != p.Record.AuthVersion) {
		fail(w, r, session.ErrUnauthorized, "")
		return
	}
	if err != nil {
		fail(w, r, err, "")
		return
	}
	var owner string
	err = tx.QueryRow(ctx, "SELECT owner_user_id::text FROM applications.apps WHERE id=$1", appID).Scan(&owner)
	if errors.Is(err, pgx.ErrNoRows) {
		fail(w, r, applications.ErrMissing, "")
		return
	}
	if err != nil {
		fail(w, r, err, "")
		return
	}
	var registered bool
	err = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM personnel.permission_catalog WHERE code='app.'||$1::text||'.access' AND app_id=$1::text AND category='application' AND enabled)", appID).Scan(&registered)
	if err != nil || !registered {
		if err == nil {
			err = applications.ErrDenied
		}
		fail(w, r, err, "")
		return
	}
	if !bootstrap && p.UserID != owner {
		fail(w, r, applications.ErrDenied, "")
		return
	}
	var exists bool
	err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM applications.form_views v
		JOIN applications.logical_tables t ON t.app_id=v.app_id AND t.id=v.table_id
		JOIN applications.menu_resources m ON m.app_id=v.app_id AND m.resource_kind='form' AND m.resource_id=v.id
		WHERE v.app_id=$1 AND v.id=$2 AND v.deleted_at IS NULL AND t.deleted_at IS NULL)`, appID, viewID).Scan(&exists)
	if err != nil || !exists {
		if err == nil {
			err = applications.ErrMissing
		}
		fail(w, r, err, "")
		return
	}
	head, err := (workflowcatalog.Catalog{}).GetInTx(ctx, tx, appID, flowID)
	if err != nil {
		fail(w, r, err, "")
		return
	}
	if head.ViewID != viewID {
		fail(w, r, errWorkflowMiss, "")
		return
	}
	var schemaVersion int64
	var graphJSON []byte
	var allowWithdraw bool
	err = tx.QueryRow(ctx, `SELECT schema_version,graph_json,allow_withdraw FROM applications.workflow_versions
		WHERE app_id=$1 AND flow_id=$2 AND version=$3`, appID, flowID, head.CandidateVersion).Scan(&schemaVersion, &graphJSON, &allowWithdraw)
	if errors.Is(err, pgx.ErrNoRows) {
		fail(w, r, workflowcatalog.ErrNotReady, "")
		return
	}
	if err != nil {
		fail(w, r, err, "")
		return
	}
	triggers, err := (workflowcatalog.Catalog{}).TriggersForVersionInTx(ctx, tx, appID, flowID, head.CandidateVersion)
	if err != nil {
		fail(w, r, err, "")
		return
	}
	graph, err := workflowcatalogGraph(graphJSON)
	if err != nil {
		fail(w, r, err, "")
		return
	}
	if err = tx.Commit(ctx); err != nil {
		fail(w, r, err, "")
		return
	}
	if err = s.Authenticator.Renew(ctx, w, r, p); err != nil {
		fail(w, r, err, "")
		return
	}
	writeEnvelope(w, r, http.StatusOK, "OK", definitionView{ID: flowID, Name: head.Name, Revision: head.Revision,
		CurrentVersion: head.CurrentVersion, CandidateVersion: head.CandidateVersion, State: head.State,
		SchemaVersion: schemaVersion, AllowWithdraw: allowWithdraw, Graph: toGraphDTO(graph), Triggers: triggers})
}

func workflowcatalogGraph(raw []byte) (flowgraph.Graph, error) {
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

func (s *Service) mutate(w http.ResponseWriter, r *http.Request, p session.Principal, appID, viewID, flowID, action string, input any, graph flowgraph.Graph) {
	operationID, operationKind, expectedRevision, expectedSchema := "", "", int64(0), int64(0)
	var save definitionRequest
	var lifecycle lifecycleRequest
	if action == "save" {
		save = input.(definitionRequest)
		operationID, operationKind = save.OperationID, "workflow.definition.save"
		expectedRevision, expectedSchema = save.ExpectedRevision, save.ExpectedSchemaVersion
	} else {
		lifecycle = input.(lifecycleRequest)
		operationID, expectedRevision = lifecycle.OperationID, lifecycle.ExpectedRevision
		if action == "enable" {
			operationKind = "workflow.enable"
		} else {
			operationKind = "workflow.close"
		}
	}
	application := &applications.Application{Pool: s.Application.Pool}
	options := []applications.WriteOptions{}
	if s.Application.Limits.LockTimeout > 0 && s.Application.Limits.StatementTimeout > 0 {
		options = append(options, applications.WriteOptions{LockTimeout: s.Application.Limits.LockTimeout, StatementTimeout: s.Application.Limits.StatementTimeout})
	}
	managerWrite, err := application.BeginManagerWrite(r.Context(), p, appID, options...)
	if err != nil {
		fail(w, r, err, operationID)
		return
	}
	defer managerWrite.Rollback(context.Background())
	fingerprintPayload, _ := json.Marshal(input)
	fingerprint := sha256.Sum256(append([]byte(appID+"\x00"+viewID+"\x00"+flowID+"\x00"+action+"\x00"), fingerprintPayload...))
	if old, e := managerWrite.Replay(r.Context(), operationID, operationKind, fingerprint); e != nil {
		fail(w, r, e, operationID)
		return
	} else if old != nil {
		if e = managerWrite.Commit(r.Context()); e != nil {
			fail(w, r, e, operationID)
			return
		}
		s.finish(w, r, p, *old, action != "")
		return
	}
	if err = managerWrite.Claim(r.Context(), operationID, operationKind, fingerprint); err != nil {
		fail(w, r, err, operationID)
		return
	}

	tx := managerWrite.Tx()
	var head workflowcatalog.Head
	catalog := workflowcatalog.Catalog{}
	switch action {
	case "save":
		var tableID string
		err = tx.QueryRow(r.Context(), `SELECT table_id::text FROM applications.form_views WHERE app_id=$1 AND id=$2 AND deleted_at IS NULL`, appID, viewID).Scan(&tableID)
		if errors.Is(err, pgx.ErrNoRows) {
			err = applications.ErrMissing
			break
		}
		if err != nil {
			break
		}
		if err = authorizeAssignees(r.Context(), tx, appID, viewID, graph); err != nil {
			fail(w, r, err, operationID)
			return
		}
		head, err = catalog.PutVersionInTx(r.Context(), tx, workflowcatalog.VersionInput{AppID: appID, TableID: tableID, ViewID: viewID, FlowID: flowID,
			ActorID: p.UserID, Name: save.Name, ExpectedRevision: save.ExpectedRevision, ExpectedSchemaVersion: save.ExpectedSchemaVersion,
			Graph: graph, AllowWithdraw: *save.AllowWithdraw, Triggers: save.Triggers})
	case "enable":
		head, err = catalog.GetInTx(r.Context(), tx, appID, flowID)
		if err == nil && head.ViewID != viewID {
			err = errWorkflowMiss
		}
		if err == nil && head.Revision != expectedRevision {
			err = workflowcatalog.ErrConflict
		}
		if err == nil {
			var graphJSON []byte
			err = tx.QueryRow(r.Context(), `SELECT graph_json FROM applications.workflow_versions WHERE app_id=$1 AND flow_id=$2 AND version=$3`, appID, flowID, head.CandidateVersion).Scan(&graphJSON)
			if errors.Is(err, pgx.ErrNoRows) {
				err = workflowcatalog.ErrNotReady
			} else if err == nil {
				var stored flowgraph.Graph
				stored, err = workflowcatalogGraph(graphJSON)
				if err == nil {
					err = authorizeAssignees(r.Context(), tx, appID, viewID, stored)
				}
			}
		}
		if err == nil {
			head, err = catalog.EnableInTx(r.Context(), tx, appID, flowID, expectedRevision)
		}
	case "close":
		head, err = catalog.RequestCloseInTx(r.Context(), tx, appID, flowID, expectedRevision)
		if err == nil && head.ViewID != viewID {
			err = errWorkflowMiss
		}
	}
	if err != nil {
		fail(w, r, err, operationID)
		return
	}
	if expectedSchema > 0 && head.CandidateVersion < 1 {
		fail(w, r, workflowcatalog.ErrConflict, operationID)
		return
	}
	resultData := mutationData{OperationID: operationID, ID: flowID, Revision: head.Revision, CurrentVersion: head.CurrentVersion,
		CandidateVersion: head.CandidateVersion, State: head.State}
	status := http.StatusOK
	if action == "save" && expectedRevision == 0 {
		status = http.StatusCreated
	}
	rawResult, _ := json.Marshal(resultData)
	result := applications.Result{Status: status, Data: rawResult}
	if err = insertAudit(r.Context(), tx, p, appID, viewID, flowID, operationID, action, head); err != nil {
		fail(w, r, err, operationID)
		return
	}
	if err = managerWrite.Complete(r.Context(), operationID, result); err != nil {
		fail(w, r, err, operationID)
		return
	}
	if err = managerWrite.Commit(r.Context()); err != nil {
		fail(w, r, err, operationID)
		return
	}
	s.finish(w, r, p, result, true)
}

func insertAudit(ctx context.Context, tx pgx.Tx, p session.Principal, appID, viewID, flowID, operationID, action string, head workflowcatalog.Head) error {
	changeAction, reasonCode := "", ""
	switch action {
	case "save":
		changeAction, reasonCode = "workflow.definition.save", "WORKFLOW_DEFINITION_SAVE"
	case "enable":
		changeAction, reasonCode = "workflow.enable", "WORKFLOW_ENABLE"
	case "close":
		changeAction, reasonCode = "workflow.close", "WORKFLOW_CLOSE"
	default:
		return errInvalid
	}
	summary, err := json.Marshal(map[string]any{"appId": appID, "flowId": flowID, "operationId": operationID,
		"revision": head.Revision, "state": head.State, "action": changeAction})
	if err != nil {
		return err
	}
	metadata := httpserver.Metadata(ctx)
	_, err = tx.Exec(ctx, `INSERT INTO auth.authentication_events(event_type,outcome,actor_user_id,session_ref,reason_code,request_id,
		object_type,object_id,change_summary) VALUES('application_structure_changed','success',$1,NULLIF($2,'')::uuid,$3,$4,'form',$5,$6)`,
		p.UserID, p.SessionRef, reasonCode, metadata.RequestID, viewID, summary)
	return err
}

func (s *Service) finish(w http.ResponseWriter, r *http.Request, p session.Principal, result applications.Result, write bool) {
	ctx := r.Context()
	if write {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
	}
	if err := s.Authenticator.Renew(ctx, w, r, p); err != nil {
		if !write {
			fail(w, r, err, "")
			return
		}
		session.ClearCookies(w)
	}
	writeEnvelope(w, r, result.Status, "OK", json.RawMessage(result.Data))
}

func authorizeAssignees(ctx context.Context, tx pgx.Tx, appID, viewID string, graph flowgraph.Graph) error {
	assignees := map[string]bool{}
	for _, node := range graph.Nodes {
		if node.Kind == "approval" && node.Approval != nil {
			for _, id := range node.Approval.AssigneeIDs {
				assignees[id] = true
			}
		}
	}
	if len(assignees) == 0 {
		return nil
	}
	ids := make([]string, 0, len(assignees))
	for id := range assignees {
		ids = append(ids, id)
	}
	rows, err := tx.Query(ctx, "SELECT id::text,status,is_bootstrap_admin FROM auth.users WHERE id=ANY($1::uuid[]) ORDER BY id FOR SHARE", ids)
	if err != nil {
		return err
	}
	type userState struct {
		status    string
		bootstrap bool
	}
	users := make(map[string]userState, len(ids))
	for rows.Next() {
		var id, status string
		var root bool
		if err = rows.Scan(&id, &status, &root); err != nil {
			rows.Close()
			return err
		}
		users[id] = userState{status, root}
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	var owner string
	var fieldsJSON []byte
	err = tx.QueryRow(ctx, `SELECT a.owner_user_id::text,t.fields_json FROM applications.apps a
		JOIN applications.form_views v ON v.app_id=a.id AND v.id=$2
		JOIN applications.logical_tables t ON t.app_id=v.app_id AND t.id=v.table_id WHERE a.id=$1 AND a.deleted_at IS NULL AND v.deleted_at IS NULL AND t.deleted_at IS NULL`, appID, viewID).Scan(&owner, &fieldsJSON)
	if errors.Is(err, pgx.ErrNoRows) {
		return applications.ErrMissing
	}
	if err != nil {
		return err
	}
	var fieldRows []struct {
		ID string `json:"id"`
	}
	if json.Unmarshal(fieldsJSON, &fieldRows) != nil {
		return errInvalid
	}
	fieldIDs := make([]string, 0, len(fieldRows))
	for _, field := range fieldRows {
		if validID(field.ID) {
			fieldIDs = append(fieldIDs, field.ID)
		}
	}
	resource := apppolicy.Resource{Ref: apppolicy.ResourceRef{ApplicationID: appID, Kind: "form", ID: viewID}, Exists: true}
	for _, id := range ids {
		state, exists := users[id]
		if !exists || state.status != "active" {
			return errApprover
		}
		if id == owner || state.bootstrap {
			continue
		}
		policy := apppolicy.TrustedContext{Actor: apppolicy.TrustedActor{ID: id}, Application: apppolicy.Application{ID: appID, OwnerID: owner, Exists: true}}
		groupRows, e := tx.Query(ctx, `SELECT p.id::text,p.enabled FROM applications.permission_groups p
			JOIN applications.group_members m ON m.app_id=p.app_id AND m.group_id=p.id
			WHERE p.app_id=$1 AND m.user_id=$2 ORDER BY p.id`, appID, id)
		if e != nil {
			return e
		}
		groupIndex := map[string]int{}
		for groupRows.Next() {
			var groupID string
			var enabled bool
			if e = groupRows.Scan(&groupID, &enabled); e != nil {
				groupRows.Close()
				return e
			}
			groupIndex[groupID] = len(policy.Groups)
			policy.Groups = append(policy.Groups, apppolicy.PermissionGroup{ID: groupID, ApplicationID: appID, Enabled: enabled, MemberIDs: []string{id}})
		}
		if e = groupRows.Err(); e != nil {
			groupRows.Close()
			return e
		}
		groupRows.Close()
		grantRows, e := tx.Query(ctx, `SELECT g.group_id::text,g.resource_kind,g.resource_id::text,g.action,g.row_scope,
			ARRAY(SELECT gf.field_id::text FROM applications.grant_fields gf WHERE gf.app_id=g.app_id AND gf.grant_id=g.id ORDER BY gf.field_id)
			FROM applications.grants g JOIN applications.group_members m ON m.app_id=g.app_id AND m.group_id=g.group_id AND m.user_id=$2
			JOIN applications.permission_groups p ON p.app_id=g.app_id AND p.id=g.group_id
			WHERE g.app_id=$1 AND p.enabled AND g.resource_kind='form' AND g.resource_id=$3 ORDER BY g.id`, appID, id, viewID)
		if e != nil {
			return e
		}
		for grantRows.Next() {
			var groupID, kind, resourceID, action, scope string
			var fields []string
			if e = grantRows.Scan(&groupID, &kind, &resourceID, &action, &scope, &fields); e != nil {
				grantRows.Close()
				return e
			}
			index, present := groupIndex[groupID]
			if !present {
				grantRows.Close()
				return errInvalid
			}
			policy.Groups[index].Grants = append(policy.Groups[index].Grants, apppolicy.Grant{Resource: apppolicy.ResourceRef{ApplicationID: appID, Kind: kind, ID: resourceID}, Action: apppolicy.Action(action), Rows: apppolicy.RowScope(scope), Fields: fields})
		}
		if e = grantRows.Err(); e != nil {
			grantRows.Close()
			return e
		}
		grantRows.Close()
		if !apppolicy.Allows(policy, resource, apppolicy.MenuEnter, apppolicy.RowFact{}, "") {
			return errApprover
		}
		canRead := false
		for _, fieldID := range fieldIDs {
			row := apppolicy.RowFact{ApplicationID: appID, Exists: true, CreatedBy: id}
			if apppolicy.Allows(policy, resource, apppolicy.DataRead, row, fieldID) {
				canRead = true
				break
			}
		}
		if !canRead {
			return errApprover
		}
	}
	return nil
}

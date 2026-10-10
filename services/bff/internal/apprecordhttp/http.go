// Package apprecordhttp composes the frozen record/draft consumer with the
// application's shared Session and HTTP contract. It contains no business SQL.
package apprecordhttp

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Hubujiu/WeaveOS/services/bff/internal/appdrafts"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appfields"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/applications"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appquery"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/apprecords"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/apprecordservice"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appstructure"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/platform/httpserver"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/querycontext"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/workflowcatalog"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/workflowevidence"
	"github.com/jackc/pgx/v5"
)

type Service struct {
	Records           *apprecordservice.Service
	Authenticator     session.Authenticator
	TrustedProxyHosts []string
}

func respond(w http.ResponseWriter, r *http.Request, status int, code string, data any, pagination any) {
	w.Header().Set("Cache-Control", "no-store")
	if status == 204 {
		w.WriteHeader(status)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	meta := map[string]any{"requestId": httpserver.Metadata(r.Context()).RequestID}
	if pagination != nil {
		meta["pagination"] = pagination
	}
	message := "请求未完成"
	if code == "OK" {
		message = "success"
	}
	if code == "OK" {
		if workflow, ok := data.(apprecordservice.WorkflowOperationResult); ok {
			switch workflow.Status {
			case "pending":
				message = "操作处理中"
			case "no_effect":
				message = "操作未执行"
			}
		}
	}

	_ = json.NewEncoder(w).Encode(struct {
		Code    string         `json:"code"`
		Message string         `json:"message"`
		Data    any            `json:"data"`
		Meta    map[string]any `json:"meta"`
	}{code, message, data, meta})
}

func failure(w http.ResponseWriter, r *http.Request, e error, operation string) {
	status, code, data := 503, "COMMON_SERVICE_UNAVAILABLE", any(nil)
	var domain *appstructure.Error
	switch {
	case errors.Is(e, workflowcatalog.ErrClosing):
		status, code = 409, "WORKFLOW_CLOSING"
	case errors.Is(e, workflowcatalog.ErrNotReady):
		status, code = 409, "WORKFLOW_NOT_READY"
	case errors.Is(e, workflowcatalog.ErrConflict):
		status, code = 409, "WORKFLOW_CONFLICT"
	case errors.Is(e, workflowcatalog.ErrMissing):
		status, code = 404, "APPLICATION_NOT_FOUND"
	case errors.Is(e, workflowcatalog.ErrInvalid):
		status, code = 400, "COMMON_VALIDATION_FAILED"
	case errors.Is(e, apprecordservice.ErrWorkflowRecordReadOnly):
		status, code = 409, "WORKFLOW_RECORD_READ_ONLY"
	case errors.Is(e, apprecordservice.ErrWorkflowTaskChanged):
		status, code = 409, "WORKFLOW_TASK_CHANGED"
	case errors.Is(e, apprecordservice.ErrWorkflowBasisChanged):
		status, code = 409, "WORKFLOW_BASIS_CHANGED"
	case errors.Is(e, apprecordservice.ErrWorkflowBasisExpired):
		status, code = 409, "WORKFLOW_BASIS_EXPIRED"
	case errors.Is(e, workflowevidence.ErrTooLarge):
		status, code = 409, "WORKFLOW_EVIDENCE_TOO_LARGE"
	case errors.Is(e, applications.ErrExpectedActorInvalid):
		status, code, data = 400, "COMMON_VALIDATION_FAILED", applications.ActorHeaderViolation()
	case errors.Is(e, applications.ErrSessionChanged):
		status, code = 409, "AUTH_SESSION_CHANGED"
	case errors.Is(e, session.ErrUnauthorized):
		status, code = 401, "AUTH_UNAUTHENTICATED"
		w.Header().Set("WWW-Authenticate", `Session realm="enterprise-management-system"`)
	case errors.Is(e, session.ErrForbidden):
		status, code = 403, "COMMON_CSRF_REJECTED"
	case errors.As(e, &domain):
		code, data = domain.Code, domain.Data
		status = 409
		switch code {
		case "COMMON_VALIDATION_FAILED", "APPLICATION_RESOURCE_INVALID":
			status = 400
		case "COMMON_UNSUPPORTED_MEDIA_TYPE":
			status = 415
		case "COMMON_SERVICE_UNAVAILABLE":
			status = 503
		}
	case errors.Is(e, applications.ErrDenied), errors.Is(e, appdrafts.ErrForbidden):
		status, code = 403, "APPLICATION_FORBIDDEN"
	case errors.Is(e, applications.ErrMissing), errors.Is(e, apprecords.ErrMissing), errors.Is(e, appdrafts.ErrMissing), errors.Is(e, pgx.ErrNoRows):
		status, code = 404, "APPLICATION_NOT_FOUND"
	case errors.Is(e, applications.ErrResourceInvalid):
		status, code = 400, "APPLICATION_RESOURCE_INVALID"
	case errors.Is(e, applications.ErrOperationConflict):
		status, code = 409, "APPLICATION_OPERATION_CONFLICT"
	case errors.Is(e, applications.ErrUnconfirmed):
		code, data = "APPLICATION_OPERATION_UNCONFIRMED", map[string]string{"operationId": operation}
	case errors.Is(e, apprecords.ErrNotReady):
		status, code = 409, "APPLICATION_SCHEMA_NOT_READY"
	case errors.Is(e, apprecords.ErrConflict):
		status, code = 409, "APPLICATION_RECORD_CONFLICT"
	case errors.Is(e, appdrafts.ErrConflict):
		status, code = 409, "APPLICATION_DRAFT_CONFLICT"
	case errors.Is(e, appdrafts.ErrBaseConflict):
		status, code = 409, "APPLICATION_DRAFT_BASE_CONFLICT"
	case errors.Is(e, querycontext.ErrChanged):
		status, code = 409, "APPLICATION_QUERY_CHANGED"
	case errors.Is(e, querycontext.ErrExpired), errors.Is(e, querycontext.ErrInvalid):
		status, code = 409, "APPLICATION_QUERY_CONTEXT_EXPIRED"
	case errors.Is(e, applications.ErrInvalid), errors.Is(e, apprecords.ErrInvalid), errors.Is(e, appdrafts.ErrInvalid), errors.Is(e, appquery.ErrInvalid), errors.Is(e, appfields.ErrInvalid):
		status, code = 400, "COMMON_VALIDATION_FAILED"
		data = map[string]any{"violations": []any{map[string]string{"location": "body", "field": "record", "code": "VALIDATION_INVALID", "message": "记录或草稿输入不合法"}}}
	}
	respond(w, r, status, code, data, nil)
}

func (s *Service) finish(w http.ResponseWriter, r *http.Request, p session.Principal, status int, data any, location string, mutation bool, pagination any) {
	ctx := r.Context()
	cancel := func() {}
	if mutation {
		ctx, cancel = context.WithTimeout(context.Background(), 2*time.Second)
	}
	e := s.Authenticator.Renew(ctx, w, r, p)
	cancel()
	if e != nil {
		if !mutation {
			failure(w, r, e, "")
			return
		}
		session.ClearCookies(w)
	}
	if location != "" {
		w.Header().Set("Location", location)
	}
	respond(w, r, status, "OK", data, pagination)
}

func closedQuery(r *http.Request, allowed ...string) (url.Values, error) {
	q, e := url.ParseQuery(r.URL.RawQuery)
	if e != nil {
		return nil, applications.ErrInvalid
	}
	for key, values := range q {
		found := false
		for _, k := range allowed {
			found = found || key == k
		}
		if !found || len(values) != 1 || values[0] == "" {
			return nil, applications.ErrInvalid
		}
	}
	return q, nil
}
func number(raw string, min, max int64) (int64, error) {
	n, e := strconv.ParseInt(raw, 10, 64)
	if e != nil || n < min || n > max || strconv.FormatInt(n, 10) != raw {
		return 0, applications.ErrInvalid
	}
	return n, nil
}
func value[T any](m map[string]json.RawMessage, k string) T {
	var v T
	_ = json.Unmarshal(m[k], &v)
	return v
}

func (s *Service) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var e error
	r, e = httpserver.Prepare(w, r, s.TrustedProxyHosts)
	if e != nil {
		failure(w, r, e, "")
		return
	}
	mutation := r.Method != "GET" && r.Method != "HEAD"
	p, e := s.Authenticator.Authenticate(r, mutation)
	if e != nil {
		failure(w, r, e, "")
		return
	}
	if e = applications.ExpectedActor(r, p); e != nil {
		failure(w, r, e, "")
		return
	}
	if s.Records == nil || s.Records.Pool == nil {
		failure(w, r, apprecordservice.ErrUnavailable, "")
		return
	}
	if s.workflowRoundHTTP(w, r, p) || s.workflowInboxHTTP(w, r, p) || s.workflowEventHistoryHTTP(w, r, p) || s.workflowEventHTTP(w, r, p) || s.workflowManualOptionsHTTP(w, r, p) || s.workflowManualHTTP(w, r, p) || s.workflowReadHTTP(w, r, p) || s.workflowLifecycleHTTP(w, r, p) || s.workflowHTTP(w, r, p) {
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/applications/")
	parts := strings.Split(path, "/")
	if path == r.URL.Path || len(parts) < 4 || len(parts) > 5 || parts[1] != "forms" || parts[3] != "records" && parts[3] != "drafts" {
		respond(w, r, 404, "API_NOT_FOUND", nil, nil)
		return
	}
	app, view, kind := parts[0], parts[2], parts[3]
	if !appfields.ValidID(app) || !appfields.ValidID(view) {
		failure(w, r, applications.ErrInvalid, "")
		return
	}
	id := ""
	if len(parts) == 5 {
		id = parts[4]
		if id != "search" || kind != "records" {
			if !appfields.ValidID(id) {
				failure(w, r, applications.ErrInvalid, "")
				return
			}
		}
	}
	operation := ""
	status := 200
	location := ""
	var data, pagination any
	m := httpserver.Metadata(r.Context())
	metadata := applications.Metadata{RequestID: m.RequestID, ClientIP: m.ClientIP, UserAgent: r.UserAgent()}
	if r.Method == "GET" || r.Method == "DELETE" {
		switch {
		case r.Method == "GET" && kind == "records" && id != "" && id != "search":
			if _, e = closedQuery(r); e == nil {
				data, e = s.Records.GetRecord(r.Context(), p, app, view, id)
			}
		case r.Method == "GET" && kind == "drafts" && id != "":
			if _, e = closedQuery(r); e == nil {
				data, e = s.Records.GetDraft(r.Context(), p, app, view, id)
			}
		case r.Method == "GET" && kind == "drafts" && id == "":
			var q url.Values
			q, e = closedQuery(r, "pageSize", "pageToken")
			if e == nil {
				size := int64(20)
				if q.Get("pageSize") != "" {
					size, e = number(q.Get("pageSize"), 1, 100)
				}
				if e == nil {
					var page appdrafts.Page
					page, e = s.Records.ListDrafts(r.Context(), p, apprecordservice.DraftListRequest{AppID: app, ViewID: view, PageSize: int(size), PageToken: q.Get("pageToken")})
					data = map[string]any{"items": page.Items}
					pagination = map[string]any{"pageSize": size, "hasMore": page.NextToken != "", "nextPageToken": nil}
					if page.NextToken != "" {
						pagination.(map[string]any)["nextPageToken"] = page.NextToken
					}
				}
			}
		case r.Method == "DELETE" && kind == "drafts" && id != "":
			var q url.Values
			q, e = closedQuery(r, "operationId", "expectedDraftVersion")
			if e == nil {
				operation = q.Get("operationId")
				if !appfields.ValidID(operation) {
					e = applications.ErrInvalid
				}
				version, ve := number(q.Get("expectedDraftVersion"), 1, 9007199254740991)
				if e == nil {
					e = ve
				}
				if e == nil {
					e = s.Records.DiscardDraft(r.Context(), p, apprecordservice.DraftDiscardRequest{AppID: app, ViewID: view, DraftID: id, OperationID: operation, ExpectedDraftVersion: version}, metadata)
					status = 204
				}
			}
		default:
			respond(w, r, 404, "API_NOT_FOUND", nil, nil)
			return
		}
	} else {
		bodyKind := ""
		switch {
		case r.Method == "POST" && kind == "records" && id == "":
			bodyKind = "record.create"
		case r.Method == "POST" && kind == "records" && id == "search":
			bodyKind = "record.search"
		case r.Method == "PATCH" && kind == "records" && id != "" && id != "search":
			bodyKind = "record.edit"
		case r.Method == "POST" && kind == "drafts" && id == "":
			bodyKind = "draft.create"
		case r.Method == "PATCH" && kind == "drafts" && id != "":
			bodyKind = "draft.update"
		}
		if bodyKind == "" {
			respond(w, r, 404, "API_NOT_FOUND", nil, nil)
			return
		}
		if _, e = closedQuery(r); e == nil {
			var b map[string]json.RawMessage
			b, e = appstructure.DecodeRecordBody(w, r, bodyKind)
			if e == nil {
				operation = value[string](b, "operationId")
				switch bodyKind {
				case "record.create":
					var result apprecordservice.MutationResult
					result, e = s.Records.Create(r.Context(), p, apprecordservice.CreateRequest{AppID: app, ViewID: view, OperationID: operation, ExpectedSchemaVersion: value[int64](b, "expectedSchemaVersion"), Values: value[map[string]any](b, "values"), DraftRef: value[*apprecordservice.DraftRef](b, "draftRef"), QueryVersion: value[string](b, "queryVersion")}, metadata)
					data = result
					status = 201
					location = r.URL.Path + "/" + result.ID
				case "record.edit":
					data, e = s.Records.Edit(r.Context(), p, apprecordservice.EditRequest{AppID: app, ViewID: view, RecordID: id, OperationID: operation, ExpectedSchemaVersion: value[int64](b, "expectedSchemaVersion"), ExpectedRecordVersion: value[int64](b, "expectedRecordVersion"), Changes: value[map[string]any](b, "changes"), DraftRef: value[*apprecordservice.DraftRef](b, "draftRef"), QueryVersion: value[string](b, "queryVersion")}, metadata)
				case "record.search":
					data, e = s.Records.Search(r.Context(), p, apprecordservice.SearchRequest{AppID: app, ViewID: view, Page: value[int64](b, "page"), PageSize: value[int64](b, "pageSize"), Filter: b["filter"], Sort: b["sort"], QuickSearch: b["quickSearch"], QueryVersion: value[string](b, "queryVersion")})
					mutation = false
				case "draft.create":
					var result appdrafts.Draft
					result, e = s.Records.CreateDraft(r.Context(), p, apprecordservice.DraftCreateRequest{AppID: app, ViewID: view, OperationID: operation, SchemaVersion: value[int64](b, "schemaVersion"), TargetRecordID: value[*string](b, "targetRecordId"), BaseRecordVersion: value[*int64](b, "baseRecordVersion"), Values: value[appdrafts.Values](b, "values")}, metadata)
					data = map[string]any{"operationId": operation, "id": result.ID, "draftVersion": result.DraftVersion}
					status = 201
					location = r.URL.Path + "/" + result.ID
				case "draft.update":
					var result appdrafts.Draft
					result, e = s.Records.UpdateDraft(r.Context(), p, apprecordservice.DraftUpdateRequest{AppID: app, ViewID: view, DraftID: id, OperationID: operation, ExpectedDraftVersion: value[int64](b, "expectedDraftVersion"), Changes: value[appdrafts.Values](b, "changes"), RemoveFieldIDs: value[[]string](b, "removeFieldIds")}, metadata)
					data = map[string]any{"operationId": operation, "id": result.ID, "draftVersion": result.DraftVersion}
				}
			}
		}
	}
	if e != nil {
		failure(w, r, e, operation)
		return
	}
	s.finish(w, r, p, status, data, location, mutation, pagination)
}

package appworkflows

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/Hubujiu/WeaveOS/services/bff/internal/applications"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appquery"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/querycontext"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"github.com/jackc/pgx/v5"
	"github.com/redis/go-redis/v9"
)

type managerBinding struct {
	ActorID string `json:"actorId"`
	AppID   string `json:"appId"`
	ViewID  string `json:"viewId"`
}
type managerRevision struct {
	Fingerprint string `json:"fingerprint"`
}
type managerItem struct {
	ID               string   `json:"id"`
	Name             string   `json:"name"`
	Revision         int64    `json:"revision"`
	CurrentVersion   int64    `json:"currentVersion"`
	CandidateVersion int64    `json:"candidateVersion"`
	State            string   `json:"state"`
	TriggerEvents    []string `json:"triggerEvents"`
	InFlightCount    int64    `json:"inFlightCount"`
	UpdatedAt        string   `json:"updatedAt"`
}
type managerResult struct {
	Items        []managerItem `json:"items"`
	Total        int64         `json:"total"`
	Page         int           `json:"page"`
	PageSize     int           `json:"pageSize"`
	QueryVersion string        `json:"queryVersion"`
}

func managerResource(b managerBinding) string { return "workflow-manager:" + b.AppID + ":" + b.ViewID }
func managerValidBinding(raw json.RawMessage) (managerBinding, bool) {
	var b managerBinding
	_, e := exactObject(raw, []string{"actorId", "appId", "viewId"}, nil)
	if e != nil || json.Unmarshal(raw, &b) != nil {
		return b, false
	}
	return b, validID(b.ActorID) && validID(b.AppID) && validID(b.ViewID)
}

func managerValidRevision(raw json.RawMessage) (managerRevision, bool) {
	var r managerRevision
	_, e := exactObject(raw, []string{"fingerprint"}, nil)
	if e != nil || json.Unmarshal(raw, &r) != nil {
		return r, false
	}
	v, e := hex.DecodeString(r.Fingerprint)
	return r, e == nil && len(v) == 32 && hex.EncodeToString(v) == r.Fingerprint
}
func NewManagerQueryStore(client redis.UniversalClient, generation string) *querycontext.Store {
	return querycontext.NewStore(client, "workflow-manager", generation, querycontext.Policy{
		Validate: func(m querycontext.Metadata) bool {
			b, ok := managerValidBinding(m.Criteria)
			r, valid := managerValidRevision(m.Revision)
			return ok && valid && m.View == managerResource(b) && m.Fingerprint == r.Fingerprint
		},
		Forward: func(resource string, old, next json.RawMessage) bool {
			_, a := managerValidRevision(old)
			_, b := managerValidRevision(next)
			parts := strings.Split(resource, ":")
			return a && b && len(parts) == 3 && parts[0] == "workflow-manager" && validID(parts[1]) && validID(parts[2])
		},
	})
}

type managerStrategy struct {
	app         *Application
	principal   session.Principal
	binding     managerBinding
	page        querycontext.Page
	observation querycontext.Observation[[]managerItem]
}

func (s *managerStrategy) Resource() string { return managerResource(s.binding) }
func (s *managerStrategy) OpenRead(ctx context.Context) (pgx.Tx, error) {
	tx, _, e := (&applications.Application{Pool: s.app.Pool}).BeginManagerRead(ctx, s.principal, s.binding.AppID)
	if e != nil {
		return nil, e
	}
	var exists bool
	e = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM applications.form_views v
 JOIN applications.logical_tables t ON t.app_id=v.app_id AND t.id=v.table_id
 JOIN applications.menu_resources m ON m.app_id=v.app_id AND m.resource_kind='form' AND m.resource_id=v.id
 WHERE v.app_id=$1 AND v.id=$2 AND v.deleted_at IS NULL AND t.deleted_at IS NULL)`, s.binding.AppID, s.binding.ViewID).Scan(&exists)
	if e != nil || !exists {
		_ = tx.Rollback(context.Background())
		if e == nil {
			e = applications.ErrMissing
		}
		return nil, e
	}
	return tx, nil
}
func (s *managerStrategy) validCriteria(raw json.RawMessage) bool {
	b, ok := managerValidBinding(raw)
	return ok && b == s.binding
}
func (s *managerStrategy) Prepare(_ context.Context, _ pgx.Tx, saved, incoming json.RawMessage) (json.RawMessage, json.RawMessage, error) {
	if len(saved) > 0 && !s.validCriteria(saved) {
		return nil, nil, querycontext.ErrExpired
	}
	if !s.validCriteria(incoming) {
		return nil, nil, querycontext.ErrInvalid
	}
	return saved, incoming, nil
}
func (s *managerStrategy) Revisions(ctx context.Context, tx pgx.Tx) (json.RawMessage, error) {
	_, offset, e := appquery.PageWindow(int64(s.page.Number), int64(s.page.Size))
	if e != nil {
		return nil, e
	}
	rows, e := tx.Query(ctx, `SELECT d.id::text,d.name,d.revision,d.current_version,d.candidate_version,d.state,d.updated_at,
 COALESCE((SELECT jsonb_agg(t->>'event' ORDER BY t->>'event') FROM applications.workflow_versions v
 CROSS JOIN LATERAL jsonb_array_elements(v.triggers_json) t WHERE v.app_id=d.app_id AND v.flow_id=d.id AND v.version=d.current_version),'[]'::jsonb),
 (SELECT count(*) FROM applications.workflow_instances i WHERE i.app_id=d.app_id AND i.flow_id=d.id AND i.state IN ('starting','active'))
 FROM applications.workflow_definitions d WHERE d.app_id=$1 AND d.view_id=$2 ORDER BY d.updated_at DESC,d.id DESC`, s.binding.AppID, s.binding.ViewID)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	h := sha256.New()
	_, _ = h.Write([]byte("weaveos/workflow-manager/v1\x00"))
	s.observation = querycontext.Observation[[]managerItem]{Items: make([]managerItem, 0, s.page.Size)}
	for rows.Next() {
		var item managerItem
		var updated time.Time
		var events []byte
		if e = rows.Scan(&item.ID, &item.Name, &item.Revision, &item.CurrentVersion, &item.CandidateVersion, &item.State, &updated, &events, &item.InFlightCount); e != nil {
			return nil, e
		}
		if json.Unmarshal(events, &item.TriggerEvents) != nil || item.TriggerEvents == nil || item.InFlightCount < 0 || item.InFlightCount > maxSafeInteger || s.observation.Total >= maxSafeInteger {
			return nil, session.ErrUnavailable
		}
		item.UpdatedAt = updated.UTC().Format(time.RFC3339Nano)
		raw, e := json.Marshal(item)
		if e != nil {
			return nil, e
		}
		_, _ = h.Write(raw)
		_, _ = h.Write([]byte{0})
		n := s.observation.Total
		s.observation.Total++
		if n >= offset && len(s.observation.Items) < s.page.Size {
			s.observation.Items = append(s.observation.Items, item)
		}
	}
	if e = rows.Err(); e != nil {
		return nil, e
	}
	s.observation.Fingerprint = hex.EncodeToString(h.Sum(nil))
	return json.Marshal(managerRevision{Fingerprint: s.observation.Fingerprint})
}
func (s *managerStrategy) Page(_ context.Context, _ pgx.Tx, raw json.RawMessage, page querycontext.Page) ([]managerItem, error) {
	if !s.validCriteria(raw) || page != s.page {
		return nil, querycontext.ErrInvalid
	}
	return s.observation.Items, nil
}
func (s *managerStrategy) Observe(ctx context.Context, tx pgx.Tx, raw json.RawMessage, page querycontext.Page) (querycontext.Observation[[]managerItem], error) {
	_, e := s.Page(ctx, tx, raw, page)
	return s.observation, e
}
func (s *Service) managerSearch(w http.ResponseWriter, r *http.Request, p session.Principal, appID, viewID string) {
	if r.Method != http.MethodPost {
		writeEnvelope(w, r, 404, "API_NOT_FOUND", nil)
		return
	}
	if !validID(appID) || !validID(viewID) || r.URL.RawQuery != "" || r.URL.ForceQuery {
		fail(w, r, errInvalid, "")
		return
	}
	raw, e := strictJSON(w, r)
	if e != nil {
		if errors.Is(e, errUnsupportedMediaType) {
			writeEnvelope(w, r, 415, "COMMON_UNSUPPORTED_MEDIA_TYPE", nil)
		} else {
			fail(w, r, e, "")
		}
		return
	}
	fields, e := exactObject(raw, []string{"page", "pageSize"}, []string{"queryVersion"})
	var q struct {
		Page         int    `json:"page"`
		PageSize     int    `json:"pageSize"`
		QueryVersion string `json:"queryVersion"`
	}
	if e != nil || json.Unmarshal(raw, &q) != nil {
		fail(w, r, errInvalid, "")
		return
	}
	if _, _, e = appquery.PageWindow(int64(q.Page), int64(q.PageSize)); e != nil {
		fail(w, r, errInvalid, "")
		return
	}
	if _, exists := fields["queryVersion"]; exists && (q.QueryVersion == "" || len(q.QueryVersion) > 256) {
		fail(w, r, errInvalid, "")
		return
	}
	if s.Application.ManagerQueries == nil {
		fail(w, r, session.ErrUnavailable, "")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	b := managerBinding{ActorID: p.UserID, AppID: appID, ViewID: viewID}
	criteria, _ := json.Marshal(b)
	strategy := &managerStrategy{app: s.Application, principal: p, binding: b, page: querycontext.Page{Number: q.Page, Size: q.PageSize}}
	result, e := querycontext.Execute(ctx, s.Application.ManagerQueries, p.SessionRef, q.QueryVersion, criteria, strategy.page, strategy)
	if e != nil {
		switch {
		case errors.Is(e, querycontext.ErrChanged):
			writeEnvelope(w, r, 409, "COMMON_QUERY_CHANGED", nil)
		case errors.Is(e, querycontext.ErrExpired), errors.Is(e, querycontext.ErrCAS):
			writeEnvelope(w, r, 409, "COMMON_QUERY_CONTEXT_EXPIRED", nil)
		case errors.Is(e, querycontext.ErrInvalid):
			fail(w, r, errInvalid, "")
		default:
			fail(w, r, e, "")
		}
		return
	}
	if e = s.Authenticator.Renew(ctx, w, r, p); e != nil {
		fail(w, r, e, "")
		return
	}
	writeEnvelope(w, r, 200, "OK", managerResult{Items: result.Items, Total: result.Total, Page: q.Page, PageSize: q.PageSize, QueryVersion: result.Version})
}

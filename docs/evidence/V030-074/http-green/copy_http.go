package apptemplates

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/applications"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appstructure"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/platform/httpserver"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"io"
	"mime"
	"net/http"
	"reflect"
	"strings"
	"time"
	"unicode/utf8"
)

type copyExecuteWire struct {
	Source                         CopySource         `json:"source"`
	Target                         CopyTarget         `json:"target"`
	ExpectedSourceFingerprint      string             `json:"expectedSourceFingerprint"`
	ExpectedTargetStructureVersion int64              `json:"expectedTargetStructureVersion"`
	ExpectedTargetPolicyRevision   int64              `json:"expectedTargetPolicyRevision"`
	Bindings                       []Binding          `json:"bindings"`
	GroupMappings                  []CopyGroupMapping `json:"groupMappings"`
	OperationID                    string             `json:"operationId"`
}

func decodeCopy(raw []byte, output any) error {
	if len(raw) > 4*1024*1024 {
		return errTooLarge
	}
	if len(raw) == 0 || !utf8.Valid(raw) {
		return ErrInvalid
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	if tokens(d, 0) != nil {
		return ErrInvalid
	}
	if _, e := d.Token(); e != io.EOF {
		return ErrInvalid
	}
	if !shape(raw, reflect.TypeOf(output).Elem(), 0) {
		return ErrInvalid
	}
	var top map[string]json.RawMessage
	if json.Unmarshal(raw, &top) != nil {
		return ErrInvalid
	}
	for _, value := range top {
		if bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return ErrInvalid
		}
	}
	if json.Unmarshal(raw, output) != nil {
		return ErrInvalid
	}
	return nil
}
func copyFail(w http.ResponseWriter, r *http.Request, e error) {
	if errors.Is(e, ErrCopySourceChanged) {
		templateRespond(w, r, 409, "APPLICATION_COPY_SOURCE_CHANGED", nil)
		return
	}
	var domain *appstructure.Error
	if errors.As(e, &domain) {
		status := 409
		if domain.Code == "COMMON_VALIDATION_FAILED" {
			status = 400
		}
		templateRespond(w, r, status, domain.Code, domain.Data)
		return
	}
	templateFail(w, r, e)
}
func (s *HTTP) serveCopy(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	var e error
	r, e = httpserver.Prepare(w, r, s.TrustedProxyHosts)
	if e != nil {
		copyFail(w, r, e)
		return
	}
	p, e := s.Authenticator.Authenticate(r, r.Method != "GET" && r.Method != "HEAD")
	if e != nil {
		copyFail(w, r, e)
		return
	}
	if e = applications.ExpectedActor(r, p); e != nil {
		copyFail(w, r, e)
		return
	}
	if s.Application == nil || s.Application.Pool == nil {
		copyFail(w, r, session.ErrUnavailable)
		return
	}
	if r.URL.RawQuery != "" || r.URL.ForceQuery {
		copyFail(w, r, ErrInvalid)
		return
	}
	action := strings.TrimPrefix(r.URL.Path, "/api/v1/structure-copies/")
	if action != "plan" && action != "preflight" && action != "execute" {
		templateRespond(w, r, 404, "API_NOT_FOUND", nil)
		return
	}
	if r.Method != "POST" {
		w.Header().Set("Allow", "POST")
		templateRespond(w, r, 405, "COMMON_METHOD_NOT_ALLOWED", nil)
		return
	}
	media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	encoding := r.Header.Get("Content-Encoding")
	if err != nil || media != "application/json" || encoding != "" && encoding != "identity" {
		templateRespond(w, r, 415, "COMMON_UNSUPPORTED_MEDIA_TYPE", nil)
		return
	}
	raw, err := io.ReadAll(io.LimitReader(r.Body, 4*1024*1024+1))
	if err != nil {
		copyFail(w, r, ErrInvalid)
		return
	}
	var result any
	switch action {
	case "plan":
		var in CopyPlanInput
		e = decodeCopy(raw, &in)
		if e == nil {
			result, e = s.Application.PlanCopy(r.Context(), p, in)
		}
	case "preflight":
		var in CopyPreflightInput
		e = decodeCopy(raw, &in)
		if e == nil {
			result, e = s.Application.PreflightCopy(r.Context(), p, in)
		}
	case "execute":
		var in copyExecuteWire
		if e = decodeCopy(raw, &in); e != nil {
			copyFail(w, r, e)
			return
		}
		req := CopyPreflightInput{Source: in.Source, Target: in.Target, ExpectedSourceFingerprint: in.ExpectedSourceFingerprint, ExpectedTargetStructureVersion: in.ExpectedTargetStructureVersion, ExpectedTargetPolicyRevision: in.ExpectedTargetPolicyRevision, Bindings: in.Bindings, GroupMappings: in.GroupMappings}
		meta := httpserver.Metadata(r.Context())
		committed, err := s.Application.ExecuteCopy(r.Context(), p, in.OperationID, req, applications.Metadata{RequestID: meta.RequestID, ClientIP: meta.ClientIP, UserAgent: meta.UserAgent})
		if errors.Is(err, applications.ErrUnconfirmed) {
			templateRespond(w, r, 503, "APPLICATION_OPERATION_UNCONFIRMED", map[string]string{"operationId": in.OperationID})
			return
		}
		if err != nil {
			copyFail(w, r, err)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		err = s.Authenticator.Renew(ctx, w, r, p)
		cancel()
		if err != nil {
			session.ClearCookies(w)
		}
		w.Header().Set("Location", committed.Location)
		templateRespond(w, r, committed.Status, "OK", committed.Data)
		return
	}
	if e != nil {
		copyFail(w, r, e)
		return
	}
	if e = s.Authenticator.Renew(r.Context(), w, r, p); e != nil {
		copyFail(w, r, e)
		return
	}
	templateRespond(w, r, 200, "OK", result)
}

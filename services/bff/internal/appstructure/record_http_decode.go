package appstructure

import (
	"bytes"
	"encoding/json"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appfields"
	"io"
	"mime"
	"net/http"
	"unicode/utf8"
)

// Decode only the closed public envelope; V015 owns field normalization,
// live tuple policy and the query compiler.
func decodeRecord(w http.ResponseWriter, r *http.Request, kind string) (map[string]json.RawMessage, error) {
	required, optional, nullable := []string{}, []string{}, []string{}
	switch kind {
	case "workflow.manual.start":
		required = []string{"operationId", "flowId", "expectedWorkflowRevision", "expectedSchemaVersion", "expectedRecordVersion"}
	case "workflow.instances.search", "workflow.manual.options.search", "workflow.inbox.search":
		required = []string{"page"}
		optional = []string{"pageSize", "queryVersion"}
	case "workflow.lifecycle.action":
		required = []string{"operationId", "action", "basisToken"}
		optional = []string{"targetNodeId"}
	case "workflow.task.action":
		required = []string{"operationId", "action", "basisToken"}
	case "workflow.task.save":
		required = []string{"operationId", "basisToken", "changes"}
	case "record.create":
		required = []string{"operationId", "expectedSchemaVersion", "values"}
		optional = []string{"draftRef", "queryVersion"}
	case "record.edit":
		required = []string{"operationId", "expectedSchemaVersion", "expectedRecordVersion", "changes"}
		optional = []string{"draftRef", "queryVersion"}
	case "record.search":
		required = []string{"page", "pageSize", "filter", "sort"}
		optional = []string{"queryVersion", "quickSearch"}
		nullable = []string{"filter", "sort"}
	case "draft.create":
		required = []string{"operationId", "targetRecordId", "schemaVersion", "baseRecordVersion", "values"}
		nullable = []string{"targetRecordId", "baseRecordVersion"}
	case "draft.update":
		required = []string{"operationId", "expectedDraftVersion", "changes", "removeFieldIds"}
	default:
		return nil, recordBodyInvalid()
	}
	media, _, e := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if e != nil || media != "application/json" {
		return nil, &Error{Code: "COMMON_UNSUPPORTED_MEDIA_TYPE"}
	}
	limit := int64(1 << 20)
	if kind == "record.search" {
		limit = 64 << 10
	}
	if kind == "workflow.manual.start" || kind == "workflow.task.action" || kind == "workflow.lifecycle.action" || kind == "workflow.instances.search" || kind == "workflow.manual.options.search" || kind == "workflow.inbox.search" {
		limit = 4096
	}
	raw, e := io.ReadAll(http.MaxBytesReader(w, r.Body, limit))
	if e != nil || !utf8.Valid(raw) {
		return nil, recordBodyInvalid()
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	if jsonValue(d) != nil {
		return nil, recordBodyInvalid()
	}
	if _, e = d.Token(); e != io.EOF {
		return nil, recordBodyInvalid()
	}
	m, e := object(raw, required, optional, nullable)
	if e != nil {
		return nil, recordBodyInvalid()
	}

	if kind == "workflow.manual.start" {
		for _, key := range []string{"operationId", "flowId"} {
			var id string
			if json.Unmarshal(m[key], &id) != nil || !appfields.ValidID(id) || id == "00000000-0000-0000-0000-000000000000" {
				return nil, recordBodyInvalid()
			}
		}
		for _, key := range []string{"expectedWorkflowRevision", "expectedSchemaVersion", "expectedRecordVersion"} {
			var n int64
			if json.Unmarshal(m[key], &n) != nil || n < 1 || n > maxVersion {
				return nil, recordBodyInvalid()
			}
		}
	}
	if kind == "workflow.task.action" {
		var operation, action, token string
		if json.Unmarshal(m["operationId"], &operation) != nil || operation == "00000000-0000-0000-0000-000000000000" ||
			json.Unmarshal(m["action"], &action) != nil || (action != "agree" && action != "reject") ||
			json.Unmarshal(m["basisToken"], &token) != nil || token == "" || utf8.RuneCountInString(token) > 256 {
			return nil, recordBodyInvalid()
		}
	}
	if kind == "workflow.task.save" {
		var operation, token string
		if json.Unmarshal(m["operationId"], &operation) != nil || operation == "00000000-0000-0000-0000-000000000000" ||
			json.Unmarshal(m["basisToken"], &token) != nil || token == "" || utf8.RuneCountInString(token) > 256 {
			return nil, recordBodyInvalid()
		}
	}
	if kind == "workflow.lifecycle.action" {
		var operation, action, token string
		if json.Unmarshal(m["operationId"], &operation) != nil || operation == "00000000-0000-0000-0000-000000000000" || json.Unmarshal(m["action"], &action) != nil || (action != "withdraw" && action != "return") || json.Unmarshal(m["basisToken"], &token) != nil || token == "" || utf8.RuneCountInString(token) > 256 {
			return nil, recordBodyInvalid()
		}
		target, present := m["targetNodeId"]
		if action == "withdraw" && present {
			return nil, recordBodyInvalid()
		}
		if action == "return" {
			var id string
			if !present || json.Unmarshal(target, &id) != nil || !appfields.ValidID(id) || id == "00000000-0000-0000-0000-000000000000" {
				return nil, recordBodyInvalid()
			}
		}
	}
	for _, key := range []string{"operationId", "targetRecordId"} {
		if b, ok := m[key]; ok && string(b) != "null" {
			var id string
			if json.Unmarshal(b, &id) != nil || !appfields.ValidID(id) {
				return nil, recordBodyInvalid()
			}
		}
	}
	for _, key := range []string{"expectedSchemaVersion", "schemaVersion", "expectedRecordVersion", "expectedDraftVersion", "baseRecordVersion", "page", "pageSize"} {
		if b, ok := m[key]; ok && string(b) != "null" {
			var n int64
			minimum := int64(0)
			if key != "expectedSchemaVersion" && key != "schemaVersion" {
				minimum = 1
			}
			if json.Unmarshal(b, &n) != nil || n < minimum || n > maxVersion || key == "pageSize" && n > 100 {
				return nil, recordBodyInvalid()
			}
		}
	}
	for _, key := range []string{"values", "changes"} {
		if b, ok := m[key]; ok {
			var values map[string]json.RawMessage
			if json.Unmarshal(b, &values) != nil || values == nil {
				return nil, recordBodyInvalid()
			}
			if kind == "workflow.task.save" && (len(values) == 0 || len(values) > 200) {
				return nil, recordBodyInvalid()
			}
			for id, v := range values {
				if !appfields.ValidID(id) {
					return nil, recordBodyInvalid()
				}
				var value any
				if json.Unmarshal(v, &value) != nil {
					return nil, recordBodyInvalid()
				}
				switch typed := value.(type) {
				case nil, string, bool:
				case []any:
					for _, item := range typed {
						if _, ok := item.(string); !ok {
							return nil, recordBodyInvalid()
						}
					}
				default:
					return nil, recordBodyInvalid()
				}
			}
		}
	}
	if b, ok := m["draftRef"]; ok {
		v, e := object(b, []string{"id", "draftVersion"}, nil, nil)
		if e != nil {
			return nil, recordBodyInvalid()
		}
		var id string
		var version int64
		if json.Unmarshal(v["id"], &id) != nil || !appfields.ValidID(id) || json.Unmarshal(v["draftVersion"], &version) != nil || version < 1 || version > maxVersion {
			return nil, recordBodyInvalid()
		}
	}
	if b, ok := m["queryVersion"]; ok {
		var token string
		if json.Unmarshal(b, &token) != nil || token == "" || utf8.RuneCountInString(token) > 256 {
			return nil, recordBodyInvalid()
		}
	}
	if b, ok := m["removeFieldIds"]; ok {
		var ids []string
		if json.Unmarshal(b, &ids) != nil || ids == nil {
			return nil, recordBodyInvalid()
		}
		for _, id := range ids {
			if !appfields.ValidID(id) {
				return nil, recordBodyInvalid()
			}
		}
	}
	if b, ok := m["quickSearch"]; ok {
		v, e := object(b, []string{"term", "fieldIds"}, nil, nil)
		if e != nil {
			return nil, recordBodyInvalid()
		}
		var term string
		var ids []string
		if json.Unmarshal(v["term"], &term) != nil || json.Unmarshal(v["fieldIds"], &ids) != nil || ids == nil {
			return nil, recordBodyInvalid()
		}
	}
	return m, nil
}

// DecodeRecordBody is the shared closed HTTP envelope boundary. Field values,
// criteria compilation and live authorization remain owned by the consumer.
func DecodeRecordBody(w http.ResponseWriter, r *http.Request, kind string) (map[string]json.RawMessage, error) {
	return decodeRecord(w, r, kind)
}

func recordBodyInvalid() error {
	return &Error{Code: "COMMON_VALIDATION_FAILED", Data: map[string]any{"violations": []any{map[string]string{"location": "body", "field": "record", "code": "VALIDATION_INVALID", "message": "记录或草稿输入不合法"}}}}
}

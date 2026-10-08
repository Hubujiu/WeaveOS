package appstructure

import (
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
)

const recordDecodeID = "00000000-0000-0000-0000-000000000001"

func decodeRecordBody(t *testing.T, kind, raw string) error {
	t.Helper()
	r := httptest.NewRequest("POST", "https://weaveos.test/api/v1/applications/"+recordDecodeID+"/forms/"+recordDecodeID+"/records", strings.NewReader(raw))
	r.Header.Set("Content-Type", "application/json")
	_, e := decodeRecord(httptest.NewRecorder(), r, kind)
	return e
}
func TestFrozenRecordHTTPDecoderAcceptsSparseAndIncompleteBodies(t *testing.T) {
	for _, sample := range []struct{ kind, raw string }{
		{"record.create", `{"operationId":"` + recordDecodeID + `","expectedSchemaVersion":0,"values":{}}`},
		{"record.edit", `{"operationId":"` + recordDecodeID + `","expectedSchemaVersion":1,"expectedRecordVersion":1,"changes":{}}`},
		{"record.search", `{"page":999,"pageSize":100,"filter":null,"sort":null,"quickSearch":{"term":"A%_\\B","fieldIds":["` + recordDecodeID + `"]}}`},
		{"draft.create", `{"operationId":"` + recordDecodeID + `","targetRecordId":null,"schemaVersion":1,"baseRecordVersion":null,"values":{"` + recordDecodeID + `":"-12."}}`},
		{"draft.update", `{"operationId":"` + recordDecodeID + `","expectedDraftVersion":1,"changes":{"` + recordDecodeID + `":null},"removeFieldIds":[]}`},
	} {
		t.Run(sample.kind, func(t *testing.T) {
			if e := decodeRecordBody(t, sample.kind, sample.raw); e != nil {
				t.Fatal("frozen valid body rejected", e)
			}
		})
	}
}
func TestFrozenRecordHTTPDecoderRejectsDuplicateUnknownNullAndNumericValues(t *testing.T) {
	valid := `{"operationId":"` + recordDecodeID + `","expectedSchemaVersion":1,"values":{}}`
	for _, raw := range []string{
		strings.Replace(valid, `"values":{}`, `"values":null`, 1),
		strings.Replace(valid, `"values":{}`, `"values":[],"extra":1`, 1),
		strings.Replace(valid, `"values":{}`, `"values":{"`+recordDecodeID+`":1.23}`, 1),
		strings.Replace(valid, `"values":{}`, `"values":{"`+recordDecodeID+`":"a","`+recordDecodeID+`":"b"}`, 1),
		strings.Replace(valid, `"values":{}`, `"values":{"createdBy":"`+recordDecodeID+`"}`, 1),
		strings.Replace(valid, `"expectedSchemaVersion":1`, `"expectedSchemaVersion":1,"expectedSchemaVersion":1`, 1),
		strings.Replace(valid, `"expectedSchemaVersion":1`, `"expectedSchemaVersion":9007199254740992`, 1),
		strings.Replace(valid, `"values":{}`, `"values":{},"draftRef":null`, 1),
		strings.Replace(valid, `"values":{}`, `"values":{},"queryVersion":null`, 1),
		valid + ` {}`,
	} {
		if e := decodeRecordBody(t, "record.create", raw); e == nil {
			t.Fatal("closed body accepted", raw)
		}
	}
	if e := decodeRecordBody(t, "record.search", `{"page":1,"pageSize":20,"filter":null,"sort":null,"quickSearch":null}`); e == nil {
		t.Fatal("explicit null quickSearch accepted")
	}
}
func TestFrozenRecordHTTPDecoderBoundsRawBodiesAndMedia(t *testing.T) {
	for _, sample := range []struct {
		kind string
		size int
		raw  string
	}{
		{"record.search", 64 * 1024, `{"page":1,"pageSize":20,"filter":null,"sort":null}`},
		{"record.create", 1024 * 1024, `{"operationId":"` + recordDecodeID + `","expectedSchemaVersion":1,"values":{}}`},
		{"draft.create", 1024 * 1024, `{"operationId":"` + recordDecodeID + `","targetRecordId":null,"schemaVersion":1,"baseRecordVersion":null,"values":{}}`},
	} {
		t.Run(sample.kind, func(t *testing.T) {
			boundary := sample.raw + strings.Repeat(" ", sample.size-len(sample.raw))
			if e := decodeRecordBody(t, sample.kind, boundary); e != nil {
				t.Fatalf("valid JSON at raw byte boundary rejected: %v", e)
			}
			e := decodeRecordBody(t, sample.kind, boundary+" ")
			var domain *Error
			if !errors.As(e, &domain) || domain.Code != "COMMON_VALIDATION_FAILED" {
				t.Fatalf("otherwise valid JSON one byte over limit must reject: %v", e)
			}
		})
	}
	r := httptest.NewRequest("POST", "https://weaveos.test", strings.NewReader(`{}`))
	r.Header.Set("Content-Type", "text/plain")
	_, e := decodeRecord(httptest.NewRecorder(), r, "record.create")
	var domain *Error
	if !errors.As(e, &domain) || domain.Code != "COMMON_UNSUPPORTED_MEDIA_TYPE" {
		t.Fatal("wrong media must retain415 code", e)
	}
}

func TestRecordHTTPQueryTokenMaximumMatchesFrozenDTO(t *testing.T) {
	for _, sample := range []struct{ kind, raw string }{{"record.create", `{"operationId":"` + recordDecodeID + `","expectedSchemaVersion":1,"values":{},"queryVersion":"` + strings.Repeat("a", 257) + `"}`}, {"record.search", `{"page":1,"pageSize":20,"filter":null,"sort":null,"queryVersion":"` + strings.Repeat("a", 257) + `"}`}} {
		if e := decodeRecordBody(t, sample.kind, sample.raw); e == nil {
			t.Fatalf("frozen queryVersion256 character wire limit ignored: %s", sample.kind)
		}
	}
}

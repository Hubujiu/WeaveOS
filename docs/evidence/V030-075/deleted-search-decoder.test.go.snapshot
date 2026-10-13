package appstructure

import (
	"strings"
	"testing"
)

func TestRootDeletedSearchDecoderAcceptsOnlyExplicitPageRead(t *testing.T) {
	for _, raw := range []string{`{"page":1,"pageSize":20}`, `{"page":4,"pageSize":100,"queryVersion":"opaque-read-context"}`} {
		if e := decodeRecordBody(t, "record.deleted.search", raw); e != nil {
			t.Fatal("valid closed deleted discovery rejected", e)
		}
	}
}
func TestRootDeletedSearchDecoderRawBoundaryAndClosedInput(t *testing.T) {
	body := `{"page":1,"pageSize":20}`
	boundary := body + strings.Repeat(" ", 4096-len(body))
	if e := decodeRecordBody(t, "record.deleted.search", boundary); e != nil {
		t.Fatal("valid 4096-byte read rejected", e)
	}
	for _, raw := range []string{boundary + " ", `{}`, `{"page":1}`, `{"page":0,"pageSize":20}`, `{"page":1,"pageSize":101}`, `{"page":1,"page":2,"pageSize":20}`, `{"page":1,"pageSize":20,"queryVersion":null}`, `{"page":1,"pageSize":20,"queryVersion":""}`, `{"page":1,"pageSize":20,"queryVersion":"` + strings.Repeat("x", 257) + `"}`, `{"page":1,"pageSize":20,"operationId":"` + recordDecodeID + `"}`, `{"page":1,"pageSize":20,"actorId":"` + recordDecodeID + `"}`} {
		if e := decodeRecordBody(t, "record.deleted.search", raw); e == nil {
			t.Fatal("invalid deleted discovery accepted", raw)
		}
	}
}

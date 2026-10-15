package appstructure

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRootWorkflowActionReasonText(t *testing.T) {
	for _, c := range []struct {
		name, text string
		want       bool
	}{
		{"one scalar", "a", true}, {"Chinese", "请补充票据", true}, {"preserve whitespace", "  说明\n第二行\t备注  ", true},
		{"500 Unicode scalars", strings.Repeat("🦊", 500), true}, {"501 scalars", strings.Repeat("🦊", 501), false},
		{"empty", "", false}, {"Unicode whitespace", "\u3000\u2003\t\r\n", false}, {"NUL", "a\x00b", false},
		{"invalid UTF8", string([]byte{0xff}), false}, {"HTML is inert text", "<script>text</script>", true},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := ValidWorkflowActionReason(c.text); got != c.want {
				t.Fatalf("valid=%v; want %v", got, c.want)
			}
		})
	}
}

func TestRootWorkflowActionReasonDecoder(t *testing.T) {
	const op = "10000000-0000-4000-8000-000000000001"
	const target = "10000000-0000-4000-8000-000000000002"
	for _, action := range []string{"reject", "return", "agree", "withdraw"} {
		kind := "workflow.task.action"
		if action == "return" || action == "withdraw" {
			kind = "workflow.lifecycle.action"
		}
		for _, c := range []struct {
			name          string
			reason        any
			present, want bool
		}{
			{"new valid", "  请补票据 🦊  ", true, action == "reject" || action == "return"},
			{"legacy omitted", nil, false, true}, {"empty", "", true, false}, {"null", nil, true, false}, {"whitespace", "\u3000\n", true, false}, {"wrong type", 42, true, false}, {"too long", strings.Repeat("x", 501), true, false}, {"NUL", "x\x00y", true, false},
		} {
			t.Run(action+"/"+c.name, func(t *testing.T) {
				body := map[string]any{"operationId": op, "action": action, "basisToken": "basis"}
				if action == "return" {
					body["targetNodeId"] = target
				}
				if c.present {
					body["reason"] = c.reason
				}
				raw, _ := json.Marshal(body)
				req := httptest.NewRequest("POST", "/", strings.NewReader(string(raw)))
				req.Header.Set("Content-Type", "application/json")
				got, err := DecodeRecordBody(httptest.NewRecorder(), req, kind)
				if (err == nil) != c.want {
					t.Fatalf("accepted=%v; want %v", err == nil, c.want)
				}
				if err == nil && c.present {
					var original string
					if json.Unmarshal(got["reason"], &original) != nil || original != c.reason {
						t.Fatal("original reason was changed")
					}
				}
			})
		}
	}
}

func TestRootWorkflowActionReasonJSONUnicodeScalars(t *testing.T) {
	for _, c := range []struct {
		name, raw, want string
		valid           bool
	}{
		{"valid pair", `"\ud83e\udd8a"`, "🦊", true},
		{"literal escape text", `"\\ud800"`, `\ud800`, true},
		{"actual replacement scalar", `"�"`, "�", true},
		{"unpaired high", `"\ud800"`, "", false},
		{"unpaired low", `"\udfff"`, "", false},
		{"high then ordinary", `"\ud800a"`, "", false},
	} {
		t.Run(c.name, func(t *testing.T) {
			body := `{"operationId":"10000000-0000-4000-8000-000000000001","action":"reject","basisToken":"basis","reason":` + c.raw + `}`
			req := httptest.NewRequest("POST", "/", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			got, e := DecodeRecordBody(httptest.NewRecorder(), req, "workflow.task.action")
			if (e == nil) != c.valid {
				t.Fatalf("accepted=%v want%v", e == nil, c.valid)
			}
			if e == nil {
				var text string
				if json.Unmarshal(got["reason"], &text) != nil || text != c.want {
					t.Fatal("reason altered")
				}
			}
		})
	}
}

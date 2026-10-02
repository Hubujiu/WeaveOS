package personnel

import (
	"encoding/json"
	"testing"
)

func TestQ36FilterRejectsLoneSurrogatesWithoutChangingValidStrings(t *testing.T) {
	wrap := func(value string) json.RawMessage {
		return json.RawMessage(`{"operator":"and","children":[{"field":"account","operator":"eq","value":` + value + `}]}`)
	}
	for _, raw := range []string{`"\ud800"`, `"\udfff"`, `"\ud800x"`, `"\ud800\u0041"`, `"\ud800\ud800"`, `"\udc00\ud800"`, `"\ud83d\ude00\ud800"`} {
		t.Run(raw, func(t *testing.T) {
			if _, err := CompileFilter("members", wrap(raw), 1); err == nil {
				t.Fatal("ill-formed UTF16 must be rejected before replacement")
			}
		})
	}
	for _, tc := range []struct{ raw, want string }{
		{`"\ud83d\ude00"`, "😀"}, {`"\uD83D\uDE00"`, "😀"}, {`"😀中文"`, "😀中文"}, {`"\ufffd"`, "�"}, {`"�"`, "�"}, {`"\\ud800"`, `\ud800`}, {`"a\"b\\c"`, "a\"b\\c"},
	} {
		t.Run(tc.raw, func(t *testing.T) {
			p, err := CompileFilter("members", wrap(tc.raw), 1)
			if err != nil || len(p.Arguments) != 1 || p.Arguments[0] != tc.want {
				t.Fatalf("valid string must remain exact: %#v %v", p.Arguments, err)
			}
		})
	}
}

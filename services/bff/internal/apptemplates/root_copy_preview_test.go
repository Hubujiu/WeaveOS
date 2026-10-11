package apptemplates

import (
	"encoding/json"
	"testing"
)

func TestRootCopyPreviewRejectsUnresolvableOptionWithoutMutation(t *testing.T) {
	valid := rootRemapManifest()
	before := string(mbytes(t, valid))
	if e := validateCopyPreview(valid); e != nil {
		t.Fatal("valid complete preview rejected", e)
	}
	if string(mbytes(t, valid)) != before {
		t.Fatal("preview allocated final identities into input")
	}
	invalid := rootRemapManifest()
	invalid.Workflows[0].Triggers[0].Condition = json.RawMessage(`{"operator":"and","children":[{"fieldId":"` + mid(60) + `","operator":"eq","value":"` + mid(888) + `"}]}`)
	before = string(mbytes(t, invalid))
	if e := validateCopyPreview(invalid); e == nil {
		t.Fatal("preview accepted a missing option that cannot be remapped")
	}
	if string(mbytes(t, invalid)) != before {
		t.Fatal("invalid preview mutated source")
	}
}

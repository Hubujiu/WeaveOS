package flowcommands

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"testing"
)

func executionID(n int) string { return fmt.Sprintf("%08x-0000-4000-8000-%012x", n, n) }

type executionVector struct {
	Hex    string
	SHA256 string
	Size   int
}

func executionVectors(t testing.TB) map[string]executionVector {
	t.Helper()
	raw, err := os.ReadFile("testdata/root-execution-vectors.json")
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]executionVector
	if err = json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	return out
}
func vectorBytes(t testing.TB, v executionVector) []byte {
	t.Helper()
	out, err := hex.DecodeString(v.Hex)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != v.Size || fmt.Sprintf("%x", sha256.Sum256(out)) != v.SHA256 {
		t.Fatal("invalid fixed Root vector")
	}
	return out
}
func smallExecutionPayload() ExecutionPayload {
	return ExecutionPayload{
		EvidenceHash: sha256.Sum256([]byte("root-audit-evidence")),
		Start: &ExecutionStart{AllowWithdraw: true, Approvers: map[string][]string{
			executionID(3): {executionID(12), executionID(10), executionID(11)},
			executionID(2): {executionID(9), executionID(8)},
		}},
		Routes: map[string]bool{},
	}
}
func executionCommand(schema, record int64) Command {
	return Command{
		ProtocolVersion: 2, CommandID: executionID(108), AppID: executionID(101), TableID: executionID(103),
		ViewID: executionID(104), RecordID: executionID(105), FlowID: executionID(102), VersionID: executionID(100),
		InstanceID: executionID(107), ActorID: executionID(106), Action: "start", DefinitionVersion: 1,
		SchemaVersion: schema, RecordVersion: record, FenceEpoch: 1, PayloadHash: [32]byte{1},
	}
}
func executionReceipt(t testing.TB, c Command, body []byte, outcome string) Receipt {
	t.Helper()
	fingerprint, err := Fingerprint(c)
	if err != nil {
		t.Fatal(err)
	}
	seq := c.ExpectedSequence
	if outcome == "success" {
		seq++
	}
	return Receipt{CommandID: c.CommandID, CommandHash: fingerprint, Outcome: outcome, Sequence: seq,
		ProofID: executionID(109), ResultHash: sha256.Sum256(body)}
}
func TestRootExecutionPayloadMatchesIndependentGoldenBytes(t *testing.T) {
	vectors := executionVectors(t)
	cases := []struct {
		name, action string
		p            ExecutionPayload
	}{
		{"payload_start", "start", smallExecutionPayload()},
		{"payload_agree", "agree", ExecutionPayload{EvidenceHash: sha256.Sum256([]byte("root-audit-evidence")), Routes: map[string]bool{executionID(3): false}}},
		{"payload_withdraw", "withdraw", ExecutionPayload{EvidenceHash: sha256.Sum256([]byte("root-audit-evidence"))}},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			actual, err := EncodeExecutionPayload(tt.action, tt.p)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(actual, vectorBytes(t, vectors[tt.name])) {
				t.Fatalf("canonical payload mismatch: %x", actual)
			}
		})
	}
}
func TestRootExecutionPayloadCanonicalizesWithoutMutatingCaller(t *testing.T) {
	p := smallExecutionPayload()
	before, _ := json.Marshal(p)
	first, err := EncodeExecutionPayload("start", p)
	if err != nil {
		t.Fatal(err)
	}
	after, _ := json.Marshal(p)
	if !bytes.Equal(before, after) {
		t.Fatal("caller input mutated")
	}
	p.Start.Approvers[executionID(2)] = []string{executionID(8), executionID(9)}
	p.Start.Approvers[executionID(3)] = []string{executionID(10), executionID(11), executionID(12)}
	second, err := EncodeExecutionPayload("start", p)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Fatal("equivalent input order changed canonical bytes")
	}
	first[0] ^= 1
	third, err := EncodeExecutionPayload("start", p)
	if err != nil || !bytes.Equal(second, third) {
		t.Fatal("returned buffer is shared", err)
	}
}
func TestRootExecutionPayloadRejectsInvalidActionConfigurationAndIdentities(t *testing.T) {
	base := func() ExecutionPayload { return smallExecutionPayload() }
	cases := []struct {
		action string
		edit   func(*ExecutionPayload)
	}{
		{"unknown", func(p *ExecutionPayload) {}},
		{"start", func(p *ExecutionPayload) { p.Start = nil }},
		{"agree", func(p *ExecutionPayload) {}},
		{"start", func(p *ExecutionPayload) { p.EvidenceHash = [32]byte{} }},
		{"start", func(p *ExecutionPayload) { p.Start.Approvers["bad"] = []string{executionID(8)} }},
		{"start", func(p *ExecutionPayload) { p.Start.Approvers[executionID(2)] = nil }},
		{"start", func(p *ExecutionPayload) {
			p.Start.Approvers[executionID(2)] = []string{executionID(8), executionID(8)}
		}},
		{"start", func(p *ExecutionPayload) {
			p.Start.Approvers[executionID(2)] = []string{"00000000-0000-0000-0000-000000000000"}
		}},
		{"start", func(p *ExecutionPayload) {
			p.Start.Approvers[executionID(2)] = []string{"AAAAAAAA-AAAA-4AAA-8AAA-AAAAAAAAAAAA"}
		}},
		{"start", func(p *ExecutionPayload) { p.Routes = map[string]bool{"bad": true} }},
		{"start", func(p *ExecutionPayload) {
			p.Start.Approvers[executionID(2)] = make([]string, 51)
			for i := range p.Start.Approvers[executionID(2)] {
				p.Start.Approvers[executionID(2)][i] = executionID(i + 1)
			}
		}},
		{"start", func(p *ExecutionPayload) {
			for i := 0; i < 101; i++ {
				p.Start.Approvers[executionID(i+1000)] = []string{executionID(8)}
			}
		}},
		{"start", func(p *ExecutionPayload) {
			for i := 0; i < 101; i++ {
				p.Routes[executionID(i+1000)] = true
			}
		}},
	}
	for i, tt := range cases {
		p := base()
		tt.edit(&p)
		raw, err := EncodeExecutionPayload(tt.action, p)
		if !errors.Is(err, ErrInvalid) || len(raw) != 0 {
			t.Fatalf("case %d accepted or leaked output: %v", i, err)
		}
	}
}
func TestRootExecutionPayloadAllowsAllKnownNonstartActionsAndEmptyGraph(t *testing.T) {
	v := executionVectors(t)
	for _, action := range []string{"agree", "reject", "withdraw", "return"} {
		p := ExecutionPayload{EvidenceHash: sha256.Sum256([]byte("root-audit-evidence"))}
		raw, err := EncodeExecutionPayload(action, p)
		if err != nil || !bytes.Equal(raw, vectorBytes(t, v["payload_withdraw"])) {
			t.Fatal(action, err)
		}
	}
	p := ExecutionPayload{EvidenceHash: [32]byte{1}, Start: &ExecutionStart{}}
	if _, err := EncodeExecutionPayload("start", p); err != nil {
		t.Fatal("empty graph is legal", err)
	}
}
func largestExecutionPayload() ExecutionPayload {
	p := ExecutionPayload{EvidenceHash: [32]byte{1}, Start: &ExecutionStart{Approvers: map[string][]string{}}, Routes: map[string]bool{}}
	for n := 0; n < 100; n++ {
		actors := make([]string, 50)
		for i := range actors {
			actors[i] = executionID(10000 + i)
		}
		p.Start.Approvers[executionID(1000+n)] = actors
		p.Routes[executionID(2000+n)] = n%2 == 0
	}
	return p
}
func TestRootExecutionPayloadUpperBoundsRemainWithinProtocolLimit(t *testing.T) {
	raw, err := EncodeExecutionPayload("start", largestExecutionPayload())
	if err != nil {
		t.Fatal(err)
	}
	// Prefix/evidence/flags/counts + 100*(node length+node+actor count+50 actors) + 100 routes.
	if len(raw) != 208550 || len(raw) > 262144 {
		t.Fatalf("unexpected bounded size %d", len(raw))
	}
}
func TestRootExecutionResultDecodesEveryFixedGoldenState(t *testing.T) {
	vectors := executionVectors(t)
	for _, state := range []string{"active", "completed", "rejected", "withdrawn", "no_effect"} {
		t.Run(state, func(t *testing.T) {
			schema, record := int64(2), int64(7)
			outcome := "success"
			if state == "active" || state == "no_effect" {
				schema = 1
				record = 1
			}
			if state == "no_effect" {
				outcome = "no_effect"
			}
			c := executionCommand(schema, record)
			raw := vectorBytes(t, vectors["result_"+state])
			receipt := executionReceipt(t, c, raw, outcome)
			result, err := DecodeExecutionResult(c, receipt, raw)
			if err != nil {
				t.Fatal(err)
			}
			wantState := state
			if state == "no_effect" {
				wantState = "unchanged"
			}
			if result.InstanceID != c.InstanceID || result.State != wantState || result.SchemaVersion != schema || result.RecordVersion != record {
				t.Fatalf("wrong bound result %+v", result)
			}
			if state == "active" {
				want := []ExecutionTask{{executionID(201), executionID(2), executionID(8), "engine-task-8", 1}, {executionID(202), executionID(2), executionID(9), "engine-task-9", 1}}
				if !reflect.DeepEqual(result.Tasks, want) {
					t.Fatalf("wrong full task mapping %+v", result.Tasks)
				}
			} else if len(result.Tasks) != 0 {
				t.Fatal("terminal result has tasks")
			}
			if state == "no_effect" {
				if result.EngineProcessID != "" || result.Reason != "cancelled" {
					t.Fatal("wrong no-effect shape")
				}
			} else if result.EngineProcessID != "engine-process-7" || result.Reason != "" {
				t.Fatal("wrong successful shape")
			}
		})
	}
}
func TestRootExecutionResultValidatesExistingReceiptBindingsAndSequence(t *testing.T) {
	raw := vectorBytes(t, executionVectors(t)["result_active"])
	c := executionCommand(1, 1)
	r := executionReceipt(t, c, raw, "success")
	cases := []struct {
		edit func(*Command, *Receipt)
		want error
	}{
		{func(c *Command, r *Receipt) { r.CommandID = executionID(999) }, ErrConflict},
		{func(c *Command, r *Receipt) { r.CommandHash = [32]byte{2} }, ErrConflict},
		{func(c *Command, r *Receipt) { r.ResultHash = [32]byte{3} }, ErrConflict},
		{func(c *Command, r *Receipt) { r.Sequence = 0 }, ErrOutOfOrder},
		{func(c *Command, r *Receipt) { r.Outcome = "unknown" }, ErrInvalid},
		{func(c *Command, r *Receipt) { r.ProofID = "" }, ErrInvalid},
		{func(c *Command, r *Receipt) { r.ResultHash = [32]byte{} }, ErrInvalid},
		{func(c *Command, r *Receipt) { c.SchemaVersion = 2; r.CommandHash, _ = Fingerprint(*c) }, ErrConflict},
		{func(c *Command, r *Receipt) { c.RecordVersion = 2; r.CommandHash, _ = Fingerprint(*c) }, ErrConflict},
		{func(c *Command, r *Receipt) { c.InstanceID = executionID(999); r.CommandHash, _ = Fingerprint(*c) }, ErrConflict},
	}
	for i, tt := range cases {
		cc, rr := c, r
		tt.edit(&cc, &rr)
		_, err := DecodeExecutionResult(cc, rr, raw)
		if !errors.Is(err, tt.want) {
			t.Fatalf("case %d got %v want %v", i, err, tt.want)
		}
	}
}
func TestRootExecutionResultRejectsEvenAValidLegacyV1Command(t *testing.T) {
	raw := vectorBytes(t, executionVectors(t)["result_active"])
	c := executionCommand(1, 1)
	c.ProtocolVersion = 1
	c.ViewID = ""
	c.FlowID = ""
	c.VersionID = ""
	c.DefinitionVersion = 0
	c.SchemaVersion = 0
	if _, err := Fingerprint(c); err != nil {
		t.Fatal("legacy command fixture must remain valid", err)
	}
	r := executionReceipt(t, c, raw, "success")
	if _, err := DecodeExecutionResult(c, r, raw); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
}
func TestRootExecutionResultRejectsEveryTruncationTailPrefixAndOversize(t *testing.T) {
	good := vectorBytes(t, executionVectors(t)["result_active"])
	c := executionCommand(1, 1)
	cases := [][]byte{nil, append(append([]byte{}, good...), 0), make([]byte, 65537)}
	prefix := append([]byte{}, good...)
	prefix[0] ^= 1
	cases = append(cases, prefix)
	for i := 0; i < len(good); i++ {
		cases = append(cases, good[:i])
	}
	for i, body := range cases {
		r := executionReceipt(t, c, body, "success")
		_, err := DecodeExecutionResult(c, r, body)
		if !errors.Is(err, ErrInvalid) {
			t.Fatalf("malformed %d returned %v", i, err)
		}
	}
}
func writeExecutionText(buf *bytes.Buffer, s string) {
	binary.Write(buf, binary.BigEndian, uint32(len(s)))
	buf.WriteString(s)
}
func rawExecutionResult(result ExecutionResult) []byte {
	var out bytes.Buffer
	out.Write([]byte{'W', 'V', 'F', 'R', 'S', 'L', 0, 1})
	for _, s := range []string{result.InstanceID, result.EngineProcessID, result.State, result.Reason} {
		writeExecutionText(&out, s)
	}
	binary.Write(&out, binary.BigEndian, uint64(result.SchemaVersion))
	binary.Write(&out, binary.BigEndian, uint64(result.RecordVersion))
	binary.Write(&out, binary.BigEndian, uint32(len(result.Tasks)))
	for _, task := range result.Tasks {
		for _, s := range []string{task.ID, task.NodeID, task.AssigneeID, task.EngineTaskID} {
			writeExecutionText(&out, s)
		}
		binary.Write(&out, binary.BigEndian, uint64(task.ActivationEpoch))
	}
	return out.Bytes()
}
func validActiveResult() ExecutionResult {
	return ExecutionResult{InstanceID: executionID(107), EngineProcessID: "engine-process-7", State: "active", SchemaVersion: 1, RecordVersion: 1,
		Tasks: []ExecutionTask{{executionID(201), executionID(2), executionID(8), "engine-task-8", 1}, {executionID(202), executionID(2), executionID(9), "engine-task-9", 1}}}
}
func TestRootExecutionResultRejectsInvalidTaskSetsAndStateShape(t *testing.T) {
	edits := []func(*ExecutionResult){
		func(r *ExecutionResult) { r.State = "pending" },
		func(r *ExecutionResult) { r.EngineProcessID = "" },
		func(r *ExecutionResult) { r.EngineProcessID = string(bytes.Repeat([]byte{'a'}, 201)) },
		func(r *ExecutionResult) { r.EngineProcessID = "id\n" },
		func(r *ExecutionResult) { r.Reason = "unexpected" },
		func(r *ExecutionResult) { r.Tasks = nil },
		func(r *ExecutionResult) { r.Tasks[0], r.Tasks[1] = r.Tasks[1], r.Tasks[0] },
		func(r *ExecutionResult) { r.Tasks[1].ID = r.Tasks[0].ID },
		func(r *ExecutionResult) { r.Tasks[1].EngineTaskID = r.Tasks[0].EngineTaskID },
		func(r *ExecutionResult) { r.Tasks[1].AssigneeID = r.Tasks[0].AssigneeID },
		func(r *ExecutionResult) { r.Tasks[1].NodeID = executionID(3) },
		func(r *ExecutionResult) { r.Tasks[1].ActivationEpoch = 2 },
		func(r *ExecutionResult) { r.Tasks[0].ID = "bad" },
		func(r *ExecutionResult) { r.Tasks[0].AssigneeID = "00000000-0000-0000-0000-000000000000" },
		func(r *ExecutionResult) { r.Tasks[0].ActivationEpoch = 0 },
		func(r *ExecutionResult) { r.Tasks[0].ActivationEpoch = maxSafeInteger + 1 },
		func(r *ExecutionResult) { r.Tasks[0].EngineTaskID = string(bytes.Repeat([]byte{'x'}, 201)) },
		func(r *ExecutionResult) { r.Tasks[0].EngineTaskID = "bad\x00id" },
		func(r *ExecutionResult) { r.Tasks[0].EngineTaskID = string([]byte{0xff}) },
		func(r *ExecutionResult) { r.State = "completed" },
		func(r *ExecutionResult) { r.SchemaVersion = 0 },
		func(r *ExecutionResult) { r.RecordVersion = maxSafeInteger + 1 },
	}
	c := executionCommand(1, 1)
	for i, edit := range edits {
		r := validActiveResult()
		edit(&r)
		raw := rawExecutionResult(r)
		receipt := executionReceipt(t, c, raw, "success")
		_, err := DecodeExecutionResult(c, receipt, raw)
		if !errors.Is(err, ErrInvalid) {
			t.Fatalf("case %d got %v", i, err)
		}
	}
}
func TestRootExecutionResultRejectsLengthBombsBeforeAllocation(t *testing.T) {
	good := vectorBytes(t, executionVectors(t)["result_active"])
	c := executionCommand(1, 1)
	bad := append([]byte{}, good...)
	binary.BigEndian.PutUint32(bad[8:12], 0xffffffff)
	r := executionReceipt(t, c, bad, "success")
	if _, err := DecodeExecutionResult(c, r, bad); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
	offset := 8
	for i := 0; i < 4; i++ {
		n := binary.BigEndian.Uint32(good[offset : offset+4])
		offset += 4 + int(n)
	}
	offset += 16
	bad = append([]byte{}, good...)
	binary.BigEndian.PutUint32(bad[offset:offset+4], 0xffffffff)
	r = executionReceipt(t, c, bad, "success")
	if _, err := DecodeExecutionResult(c, r, bad); !errors.Is(err, ErrInvalid) {
		t.Fatal("task count bomb", err)
	}
}
func TestRootExecutionResultRequiresOutcomeAndStateAgreement(t *testing.T) {
	vectors := executionVectors(t)
	c := executionCommand(1, 1)
	for _, tt := range []struct{ name, outcome string }{{"result_active", "no_effect"}, {"result_no_effect", "success"}} {
		raw := vectorBytes(t, vectors[tt.name])
		r := executionReceipt(t, c, raw, tt.outcome)
		if _, err := DecodeExecutionResult(c, r, raw); !errors.Is(err, ErrConflict) {
			t.Fatal(tt.name, err)
		}
	}
}
func TestRootExecutionResultAllowsOnlyFrozenNoEffectReasons(t *testing.T) {
	c := executionCommand(1, 1)
	reasons := []string{"instance_missing", "instance_exists", "deployment_missing", "deployment_mismatch", "scope_mismatch", "terminal_instance", "stale_sequence", "stale_fence", "stale_schema_version", "stale_record_version", "task_missing", "task_inactive", "task_epoch_mismatch", "actor_mismatch", "withdrawal_forbidden", "return_target_unvisited", "return_target_forbidden", "cancelled"}
	for _, reason := range reasons {
		result := ExecutionResult{InstanceID: c.InstanceID, State: "unchanged", Reason: reason, SchemaVersion: 1, RecordVersion: 1}
		raw := rawExecutionResult(result)
		receipt := executionReceipt(t, c, raw, "no_effect")
		decoded, err := DecodeExecutionResult(c, receipt, raw)
		if err != nil || decoded.Reason != reason {
			t.Fatal(reason, err)
		}
	}
	for _, reason := range []string{"", "network_timeout", "unknown"} {
		raw := rawExecutionResult(ExecutionResult{InstanceID: c.InstanceID, State: "unchanged", Reason: reason, SchemaVersion: 1, RecordVersion: 1})
		if _, err := DecodeExecutionResult(c, executionReceipt(t, c, raw, "no_effect"), raw); !errors.Is(err, ErrInvalid) {
			t.Fatal(reason, err)
		}
	}
}
func TestRootExecutionResultDoesNotAliasInputOrOtherDecodeResults(t *testing.T) {
	body := vectorBytes(t, executionVectors(t)["result_active"])
	saved := append([]byte{}, body...)
	c := executionCommand(1, 1)
	receipt := executionReceipt(t, c, body, "success")
	first, err := DecodeExecutionResult(c, receipt, body)
	if err != nil {
		t.Fatal(err)
	}
	for i := range body {
		body[i] = 0
	}
	if !reflect.DeepEqual(first, validActiveResult()) {
		t.Fatal("input alias changed result")
	}
	first.Tasks[0].AssigneeID = executionID(999)
	next, err := DecodeExecutionResult(c, receipt, saved)
	if err != nil || !reflect.DeepEqual(next, validActiveResult()) {
		t.Fatal("decode shared mutable task storage", err)
	}
}
func maximalExecutionResult() ExecutionResult {
	r := validActiveResult()
	r.Tasks = nil
	r.EngineProcessID = string(bytes.Repeat([]byte{'p'}, 200))
	for i := 0; i < 50; i++ {
		r.Tasks = append(r.Tasks, ExecutionTask{executionID(200 + i), executionID(2), executionID(1000 + i), fmt.Sprintf("%0200d", i), 1})
	}
	return r
}
func TestRootExecutionResultBoundsTaskCount(t *testing.T) {
	c := executionCommand(1, 1)
	r := maximalExecutionResult()
	body := rawExecutionResult(r)
	result, err := DecodeExecutionResult(c, executionReceipt(t, c, body, "success"), body)
	if err != nil || len(result.Tasks) != 50 {
		t.Fatal("50 tasks rejected", err)
	}
	r.Tasks = append(r.Tasks, ExecutionTask{executionID(9999), executionID(2), executionID(9999), "engine-extra", 1})
	body = rawExecutionResult(r)
	if _, err = DecodeExecutionResult(c, executionReceipt(t, c, body, "success"), body); !errors.Is(err, ErrInvalid) {
		t.Fatal("51 tasks accepted", err)
	}
}
func BenchmarkRootExecutionPayloadSmall(b *testing.B) {
	p := smallExecutionPayload()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := EncodeExecutionPayload("start", p); err != nil {
			b.Fatal(err)
		}
	}
}
func BenchmarkRootExecutionPayloadMax(b *testing.B) {
	p := largestExecutionPayload()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := EncodeExecutionPayload("start", p); err != nil {
			b.Fatal(err)
		}
	}
}
func BenchmarkRootExecutionResultSmall(b *testing.B) {
	body := vectorBytes(b, executionVectors(b)["result_active"])
	c := executionCommand(1, 1)
	r := executionReceipt(b, c, body, "success")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := DecodeExecutionResult(c, r, body); err != nil {
			b.Fatal(err)
		}
	}
}
func BenchmarkRootExecutionResultMax(b *testing.B) {
	body := rawExecutionResult(maximalExecutionResult())
	c := executionCommand(1, 1)
	r := executionReceipt(b, c, body, "success")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := DecodeExecutionResult(c, r, body); err != nil {
			b.Fatal(err)
		}
	}
}

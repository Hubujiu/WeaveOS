//go:build weaveos_cost

package apprecordservice

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"runtime"
	"sort"
	"testing"
	"time"

	"github.com/Hubujiu/WeaveOS/services/bff/internal/appfields"
	ev "github.com/Hubujiu/WeaveOS/services/bff/internal/workflowevidence"
	"github.com/jackc/pgx/v5"
)

// These full-copy and delta representations are independent test comparators,
// not production persistence paths or user authorization implementations.
type rootCostMetadata struct {
	Header          ev.Header `json:"header"`
	VisibleFieldIDs []string  `json:"visibleFieldIds"`
}

var rootEvidenceCostSink []ev.Field

func rootCostText(seed int64, n int) string {
	r := rand.New(rand.NewSource(seed))
	alphabet := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	s := make([]byte, n)
	for i := range s {
		s[i] = alphabet[r.Intn(len(alphabet))]
	}
	return string(s)
}
func rootCostFrame(manifest []byte, blobs []ev.Blob) []byte {
	out := []byte("ROOTCOST1")
	appendPart := func(b []byte) { out = binary.BigEndian.AppendUint32(out, uint32(len(b))); out = append(out, b...) }
	appendPart(manifest)
	out = binary.BigEndian.AppendUint32(out, uint32(len(blobs)))
	for _, blob := range blobs {
		out = append(out, []byte(blob.FieldID)...)
		appendPart(blob.Body)
	}
	return out
}
func rootCostParse(raw []byte, decodeHeader bool) (ev.Manifest, []byte, []ev.Blob, error) {
	if len(raw) < 9 || string(raw[:9]) != "ROOTCOST1" || len(raw) > ev.MaxBundleBytes+40*ev.MaxFields+17 {
		return ev.Manifest{}, nil, nil, fmt.Errorf("invalid comparison frame")
	}
	rest := raw[9:]
	part := func(limit int) ([]byte, error) {
		if len(rest) < 4 {
			return nil, fmt.Errorf("truncated comparison length")
		}
		n := int(binary.BigEndian.Uint32(rest))
		rest = rest[4:]
		if n > limit || n > len(rest) {
			return nil, fmt.Errorf("invalid comparison part")
		}
		v := rest[:n]
		rest = rest[n:]
		return v, nil
	}
	manifest, e := part(ev.MaxManifestBytes)
	if e != nil {
		return ev.Manifest{}, nil, nil, e
	}
	var m ev.Manifest
	if decodeHeader {
		m, e = ev.DecodeManifest(manifest)
		if e != nil {
			return ev.Manifest{}, nil, nil, e
		}
	}
	if len(rest) < 4 {
		return ev.Manifest{}, nil, nil, fmt.Errorf("truncated comparison count")
	}
	n := int(binary.BigEndian.Uint32(rest))
	rest = rest[4:]
	if n > ev.MaxFields {
		return ev.Manifest{}, nil, nil, fmt.Errorf("comparison field count")
	}
	fields := make([]ev.Blob, 0, n)
	for i := 0; i < n; i++ {
		if len(rest) < 36 {
			return ev.Manifest{}, nil, nil, fmt.Errorf("truncated comparison ID")
		}
		id := string(rest[:36])
		rest = rest[36:]
		if !appfields.ValidID(id) {
			return ev.Manifest{}, nil, nil, fmt.Errorf("invalid comparison ID")
		}
		b, e := part(ev.MaxFieldBytes)
		if e != nil {
			return ev.Manifest{}, nil, nil, e
		}
		fields = append(fields, ev.Blob{FieldID: id, Body: b})
	}
	if len(rest) != 0 {
		return ev.Manifest{}, nil, nil, fmt.Errorf("comparison trailing bytes")
	}
	return m, manifest, fields, nil
}
func rootCostLoad(ctx context.Context, conn *pgx.Conn, mode string, version int, expected ev.Bundle) ([]ev.Field, error) {
	tx, e := conn.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if e != nil {
		return nil, e
	}
	defer tx.Rollback(context.Background())
	target, e := ev.DecodeManifest(expected.Manifest)
	if e != nil {
		return nil, e
	}
	ids := make([]string, len(target.Fields))
	for i, ref := range target.Fields {
		ids[i] = ref.FieldID
	}
	if mode == "content-reuse" {
		return (ev.Store{}).FieldsInTx(ctx, tx, target.Header.AppID, expected.Hash, ids)
	}
	var frames [][]byte
	if mode == "full-copy" {
		var body, digest []byte
		if e = tx.QueryRow(ctx, "SELECT body,digest FROM root_evidence_full_cost WHERE version=$1", version).Scan(&body, &digest); e != nil {
			return nil, e
		}
		h := sha256.Sum256(body)
		if !bytes.Equal(h[:], digest) {
			return nil, fmt.Errorf("full-copy hash mismatch")
		}
		frames = append(frames, body)
	} else if mode == "delta-chain" {
		rows, e := tx.Query(ctx, "SELECT version,parent,body,digest FROM root_evidence_delta_cost WHERE version<=$1 ORDER BY version DESC", version)
		if e != nil {
			return nil, e
		}
		defer rows.Close()
		want := version
		for rows.Next() {
			var v, parent int
			var body, digest []byte
			if e = rows.Scan(&v, &parent, &body, &digest); e != nil {
				return nil, e
			}
			h := sha256.Sum256(body)
			if v != want || parent != v-1 || !bytes.Equal(h[:], digest) {
				return nil, fmt.Errorf("delta chain discontinuity")
			}
			frames = append(frames, bytes.Clone(body))
			want--
		}
		if e = rows.Err(); e != nil {
			return nil, e
		}
		if want != 0 {
			return nil, fmt.Errorf("delta base missing")
		}
	} else {
		return nil, fmt.Errorf("unknown comparator")
	}
	resolved := make(map[string]ev.Field, len(target.Fields))
	observedHashes := make(map[string][32]byte, len(target.Fields))
	hashes := make(map[string][32]byte, len(target.Fields))
	for _, ref := range target.Fields {
		hashes[ref.FieldID] = ref.Hash
	}
	for index, body := range frames {
		m, manifest, parts, e := rootCostParse(body, index == 0 && mode == "full-copy")
		if e != nil {
			return nil, e
		}
		if index == 0 && mode == "delta-chain" {
			var metadata rootCostMetadata
			if e = json.Unmarshal(manifest, &metadata); e != nil {
				return nil, e
			}
			m.Header = metadata.Header
			m.VisibleFieldIDs = metadata.VisibleFieldIDs
			if m.Header != target.Header || fmt.Sprint(m.VisibleFieldIDs) != fmt.Sprint(target.VisibleFieldIDs) {
				return nil, fmt.Errorf("delta metadata differs")
			}
		}
		if index == 0 && (m.Header.AppID != target.Header.AppID || m.Header.RecordID != target.Header.RecordID || m.Header.RecordVersion != int64(version)) {
			return nil, fmt.Errorf("comparison scope/version differs")
		}
		if index == 0 && mode == "full-copy" && !bytes.Equal(manifest, expected.Manifest) {
			return nil, fmt.Errorf("comparison current manifest differs")
		}
		for _, part := range parts {
			id, raw := part.FieldID, part.Body
			if _, ok := resolved[id]; ok {
				continue
			}
			expectedHash, needed := hashes[id]
			if !needed {
				continue
			}
			actualHash := sha256.Sum256(raw)
			if actualHash != expectedHash {
				return nil, fmt.Errorf("comparison field identity mismatch")
			}
			field, e := ev.DecodeField(raw)
			if e != nil || field.Definition.ID != id {
				return nil, fmt.Errorf("comparison invalid selected field: %v", e)
			}
			resolved[id] = field
			observedHashes[id] = actualHash
		}
		if len(resolved) == len(ids) {
			break
		}
	}
	if len(resolved) != len(ids) {
		return nil, fmt.Errorf("comparison incomplete history")
	}
	out := make([]ev.Field, len(ids))
	for i, id := range ids {
		out[i] = resolved[id]
	}
	if mode == "delta-chain" {
		refs := make([]ev.FieldRef, len(ids))
		for i, id := range ids {
			refs[i] = ev.FieldRef{FieldID: id, Hash: observedHashes[id]}
		}
		rebuilt, e := ev.EncodeManifest(ev.Manifest{Header: target.Header, Fields: refs, VisibleFieldIDs: target.VisibleFieldIDs})
		if e != nil || sha256.Sum256(rebuilt) != expected.Hash {
			return nil, fmt.Errorf("delta reconstructed evidence hash differs: %v", e)
		}
	}
	return out, nil
}
func rootCostAssert(t *testing.T, got []ev.Field, want ev.Bundle) {
	t.Helper()
	if len(got) != len(want.Fields) {
		t.Fatal("comparison omitted fields")
	}
	byID := map[string]ev.Blob{}
	for _, blob := range want.Fields {
		byID[blob.FieldID] = blob
	}
	for _, field := range got {
		raw, e := ev.EncodeField(field)
		if e != nil || !bytes.Equal(raw, byID[field.Definition.ID].Body) {
			t.Fatal("comparison changed original field bytes")
		}
	}
}

type rootCostSize struct{ Table, Index, Total int64 }

func rootCostSizes(t *testing.T, ctx context.Context, conn *pgx.Conn, relations []string) rootCostSize {
	t.Helper()
	var out rootCostSize
	for _, name := range relations {
		var s rootCostSize
		if e := conn.QueryRow(ctx, "SELECT pg_table_size($1::regclass),pg_indexes_size($1::regclass),pg_total_relation_size($1::regclass)", name).Scan(&s.Table, &s.Index, &s.Total); e != nil {
			t.Fatal(e)
		}
		out.Table += s.Table
		out.Index += s.Index
		out.Total += s.Total
	}
	return out
}
func TestRootEvidenceCostComparison(t *testing.T) {
	f := rootEvidenceStoreSetup(t)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	f.ctx = ctx
	connection, e := f.owner.Acquire(ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer connection.Release()
	conn := connection.Conn()
	if _, e = conn.Exec(ctx, "CREATE TEMP TABLE root_evidence_full_cost(version integer PRIMARY KEY,body bytea NOT NULL,digest bytea NOT NULL); CREATE TEMP TABLE root_evidence_delta_cost(version integer PRIMARY KEY,parent integer NOT NULL CHECK(parent=version-1),body bytea NOT NULL,digest bytea NOT NULL)"); e != nil {
		t.Fatal(e)
	}
	const count = 32
	valueBytes := 4096
	switch os.Getenv("WEAVEOS_EVIDENCE_COST_VALUE_BYTES") {
	case "", "4096":
	case "64":
		valueBytes = 64
	default:
		t.Fatal("unsupported controlled cost workload")
	}
	const versions = 128
	fields := make([]ev.Field, count)
	ids := make([]string, count)
	for i := range fields {
		ids[i] = recordOperationID(t, f.recordFixture)
		fields[i] = ev.Field{Definition: appfields.Field{ID: ids[i], Name: fmt.Sprintf("Cost %02d", i), Kind: "text", Default: json.RawMessage("null"), Config: json.RawMessage(`{"maxLength":null}`)}, Value: rootEvidenceString(rootCostText(int64(100+i), valueBytes))}
	}
	relations := []string{"applications.workflow_evidence_blobs", "applications.workflow_evidence_documents", "applications.workflow_evidence_members"}
	baseline := rootCostSizes(t, ctx, conn, relations)
	var server string
	if e = conn.QueryRow(ctx, "SHOW server_version").Scan(&server); e != nil {
		t.Fatal(e)
	}
	history := make([]ev.Bundle, 0, versions)
	var fullLogical, deltaLogical int64
	for v := 1; v <= versions; v++ {
		if v > 1 {
			fields[0].Value = rootEvidenceString(rootCostText(int64(10000+v), valueBytes))
		}
		h := f.header
		h.RecordVersion = int64(v)
		bundle, e := ev.Build(h, fields, ids)
		if e != nil {
			t.Fatal(e)
		}
		history = append(history, bundle)
		rootEvidenceStorePut(t, f, bundle)
		full := rootCostFrame(bundle.Manifest, bundle.Fields)
		changed := bundle.Fields
		if v > 1 {
			changed = []ev.Blob{rootEvidenceBlob(t, bundle, ids[0])}
		}
		decoded, e := ev.DecodeManifest(bundle.Manifest)
		if e != nil {
			t.Fatal(e)
		}
		metadata, e := json.Marshal(rootCostMetadata{Header: decoded.Header, VisibleFieldIDs: decoded.VisibleFieldIDs})
		if e != nil {
			t.Fatal(e)
		}
		delta := rootCostFrame(metadata, changed)
		fullDigest, deltaDigest := sha256.Sum256(full), sha256.Sum256(delta)
		if _, e = conn.Exec(ctx, "INSERT INTO root_evidence_full_cost VALUES($1,$2,$3)", v, full, fullDigest[:]); e != nil {
			t.Fatal(e)
		}
		if _, e = conn.Exec(ctx, "INSERT INTO root_evidence_delta_cost VALUES($1,$2,$3,$4)", v, v-1, delta, deltaDigest[:]); e != nil {
			t.Fatal(e)
		}
		fullLogical += int64(len(full))
		deltaLogical += int64(len(delta))
		if v != 32 && v != versions {
			continue
		}
		rootEvidenceStoreCounts(t, f, count+v-1, v, count*v)
		modes := []string{"full-copy", "delta-chain", "content-reuse"}
		for _, mode := range modes {
			for _, target := range []int{1, v / 2, v} {
				got, e := rootCostLoad(ctx, conn, mode, target, history[target-1])
				if e != nil {
					t.Fatal(e)
				}
				rootCostAssert(t, got, history[target-1])
			}
			samples := make([]int64, 20)
			for i := range samples {
				start := time.Now()
				got, e := rootCostLoad(ctx, conn, mode, v, bundle)
				samples[i] = time.Since(start).Nanoseconds()
				if e != nil {
					t.Fatal(e)
				}
				rootEvidenceCostSink = got
			}
			sort.Slice(samples, func(i, j int) bool { return samples[i] < samples[j] })
			var benchmarkErr error
			result := testing.Benchmark(func(b *testing.B) {
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					got, e := rootCostLoad(ctx, conn, mode, v, bundle)
					if e != nil {
						benchmarkErr = e
						b.Fatal(e)
					}
					rootEvidenceCostSink = got
				}
			})
			if benchmarkErr != nil {
				t.Fatal(benchmarkErr)
			}
			var size rootCostSize
			var logical int64
			switch mode {
			case "full-copy":
				size = rootCostSizes(t, ctx, conn, []string{"root_evidence_full_cost"})
				logical = fullLogical
			case "delta-chain":
				size = rootCostSizes(t, ctx, conn, []string{"root_evidence_delta_cost"})
				logical = deltaLogical
			default:
				size = rootCostSizes(t, ctx, conn, relations)
				size.Table -= baseline.Table
				size.Index -= baseline.Index
				size.Total -= baseline.Total
				if e = conn.QueryRow(ctx, "SELECT (SELECT COALESCE(sum(octet_length(body)),0) FROM applications.workflow_evidence_blobs WHERE app_id=$1)+(SELECT COALESCE(sum(octet_length(body)),0) FROM applications.workflow_evidence_documents WHERE app_id=$1)", f.app).Scan(&logical); e != nil {
					t.Fatal(e)
				}
			}
			report := map[string]any{"mode": mode, "fields": count, "versions": v, "valueBytes": valueBytes, "changedFieldsPerVersion": 1, "warmSamples": 20, "p50ns": samples[9], "p95ns": samples[18], "benchmarkN": result.N, "nsPerOp": result.NsPerOp(), "clientAllocatedBytesPerOp": result.AllocedBytesPerOp(), "clientAllocsPerOp": result.AllocsPerOp(), "pgTableBytes": size.Table, "pgIndexBytes": size.Index, "pgTotalBytes": size.Total, "logicalBodyBytes": logical, "go": runtime.Version(), "postgres": server, "architecture": runtime.GOARCH, "scope": "synthetic warm reads; same owner benchmark connection; production write role validated separately; allocation is not RSS; default TOAST; Store size is three-table growth"}
			raw, e := json.Marshal(report)
			if e != nil {
				t.Fatal(e)
			}
			t.Logf("ROOT_EVIDENCE_COST %s", raw)
		}
	}
}

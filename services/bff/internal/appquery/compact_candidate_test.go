package appquery

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

const compactActor = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"

type compactObservation struct {
	full, compact string
	count         int64
}

// The candidate must use the complete oracle after a schema transition. A
// schemaVersion mismatch alone does not force a user-visible refresh when P
// remains equal.
func candidateChanged(before, after compactObservation, beforeSchema, afterSchema int64) bool {
	if beforeSchema != afterSchema {
		return before.full != after.full || before.count != after.count
	}
	return before.compact != after.compact
}

// Independent complete-result oracle: value, order, mask, actual reference
// display, and observable system metadata are present in every matched row.
func observeCompactFixture(ctx context.Context, db *pgx.Conn, secretAll, refOwn bool) (compactObservation, error) {
	var got compactObservation
	tx, err := db.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
	if err != nil {
		return got, err
	}
	defer tx.Rollback(ctx)
	secret := `CASE WHEN r.created_by=$1::uuid THEN jsonb_build_object('secret',r.secret) ELSE '{}'::jsonb END`
	if secretAll {
		secret = `jsonb_build_object('secret',r.secret)`
	}
	ref := `jsonb_build_object('id',r.ref_id,'display',s.display,'deleted',s.deleted)`
	if refOwn {
		ref = `CASE WHEN r.created_by=$1::uuid THEN ` + ref + ` ELSE '{}'::jsonb END`
	}
	q := `SELECT jsonb_build_array(r.id,r.created_by,r.created_at,r.updated_at,r.record_version,r.number,r.public_text,` + secret + `,` + ref + `) FROM v015_compact_rows r JOIN v015_compact_sources s ON s.id=r.ref_id WHERE r.number<50 AND $1::uuid IS NOT NULL ORDER BY r.number DESC,r.id DESC`
	rows, err := tx.Query(ctx, q, compactActor)
	if err != nil {
		return got, err
	}
	full, err := FingerprintRows(ctx, rows)
	if err != nil {
		return got, err
	}
	got.full, got.count = full.Fingerprint, full.Total
	got.compact, err = candidateCompactSignature(ctx, tx, secretAll, refOwn)
	if err != nil {
		return got, err
	}
	return got, tx.Commit(ctx)
}

// Test-only compact A candidate. This is not a production query strategy.
// It streams 16-byte ID, 8-byte version and 1-byte effective own-mask marker,
// then canonical distinct labels of only references actually visible in the
// matching set. All queries must share the observation's RR transaction.
func candidateCompactSignature(ctx context.Context, tx pgx.Tx, secretAll, refOwn bool) (string, error) {
	h := sha256.New()
	_, _ = h.Write([]byte("v015/compact-experiment/v1\x00"))
	refScope := "ref:all\x00"
	if refOwn {
		refScope = "ref:own\x00"
	}
	if secretAll {
		_, _ = h.Write([]byte("number:all;text:all;secret:all;" + refScope))
	} else {
		_, _ = h.Write([]byte("number:all;text:all;secret:own;" + refScope))
	}
	rows, err := tx.Query(ctx, `SELECT r.id,r.record_version,r.created_by=$1::uuid
        FROM v015_compact_rows r WHERE r.number<50 ORDER BY r.number DESC,r.id DESC`, compactActor)
	if err != nil {
		return "", err
	}
	var count uint64
	var versionBytes [8]byte
	for rows.Next() {
		var id pgtype.UUID
		var version int64
		var own bool
		if err = rows.Scan(&id, &version, &own); err != nil {
			break
		}
		if !id.Valid || version < 1 || version > MaxJSONVersion || count >= uint64(MaxJSONVersion) {
			err = errors.New("invalid compact row")
			break
		}
		_, _ = h.Write(id.Bytes[:])
		binary.BigEndian.PutUint64(versionBytes[:], uint64(version))
		_, _ = h.Write(versionBytes[:])
		if own {
			_, _ = h.Write([]byte{1})
		} else {
			_, _ = h.Write([]byte{0})
		}
		count++
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		return "", err
	}
	binary.BigEndian.PutUint64(versionBytes[:], count)
	_, _ = h.Write(versionBytes[:])
	_, _ = h.Write([]byte("visible-references\x00"))
	refQuery := `SELECT DISTINCT s.id,s.display,s.deleted
        FROM v015_compact_rows r JOIN v015_compact_sources s ON s.id=r.ref_id
		WHERE r.number<50`
	var refArgs []any
	if refOwn {
		refQuery += ` AND r.created_by=$1::uuid`
		refArgs = []any{compactActor}
	}
	refQuery += ` ORDER BY s.id,s.display,s.deleted`
	refs, err := tx.Query(ctx, refQuery, refArgs...)
	if err != nil {
		return "", err
	}
	for refs.Next() {
		var id pgtype.UUID
		var label string
		var deleted bool
		if err = refs.Scan(&id, &label, &deleted); err != nil {
			break
		}
		if !id.Valid || len(label) > 65536 {
			err = errors.New("invalid visible reference")
			break
		}
		_, _ = h.Write(id.Bytes[:])
		binary.BigEndian.PutUint64(versionBytes[:], uint64(len(label)))
		_, _ = h.Write(versionBytes[:])
		_, _ = h.Write([]byte(label))
		if deleted {
			_, _ = h.Write([]byte{1})
		} else {
			_, _ = h.Write([]byte{0})
		}
	}
	if err == nil {
		err = refs.Err()
	}
	refs.Close()
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func TestCompactCandidateAgainstCompletePGOracle(t *testing.T) {
	dsn := os.Getenv("WEAVEOS_TEST_DATABASE_URL")
	if dsn == "" {
		t.Fatal("isolated PostgreSQL required")
	}
	ctx := context.Background()
	db, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close(ctx)
	if _, err = db.Exec(ctx, `CREATE TEMP TABLE v015_compact_sources(id uuid PRIMARY KEY,display text NOT NULL,deleted boolean NOT NULL DEFAULT false);
 CREATE TEMP TABLE v015_compact_rows(id uuid PRIMARY KEY,created_by uuid NOT NULL,created_at timestamptz NOT NULL,updated_at timestamptz NOT NULL,record_version bigint NOT NULL,number numeric NOT NULL,public_text text NOT NULL,secret text NOT NULL,ref_id uuid NOT NULL REFERENCES v015_compact_sources(id));
 INSERT INTO v015_compact_sources(id,display) VALUES
 ('44444444-4444-4444-8444-444444444444','North'),
 ('55555555-5555-4555-8555-555555555555','South'),
 ('66666666-6666-4666-8666-666666666666','Outside');
 INSERT INTO v015_compact_rows VALUES
 ('11111111-1111-4111-8111-111111111111','aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa','2026-01-01 UTC','2026-01-01 UTC',1,10,'visible-own','secret-own','44444444-4444-4444-8444-444444444444'),
 ('22222222-2222-4222-8222-222222222222','bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb','2026-01-01 UTC','2026-01-01 UTC',1,20,'visible-other','secret-other','55555555-5555-4555-8555-555555555555'),
 ('33333333-3333-4333-8333-333333333333','bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb','2026-01-01 UTC','2026-01-01 UTC',1,70,'outside','outside-secret','66666666-6666-4666-8666-666666666666');`); err != nil {
		t.Fatal(err)
	}
	previous, err := observeCompactFixture(ctx, db, false, false)
	if err != nil {
		t.Fatal(err)
	}
	if previous.count != 2 {
		t.Fatalf("initial member count=%d", previous.count)
	}
	stages := []struct {
		name, sql              string
		secretAll, wantChanged bool
	}{
		{"no-op", `UPDATE v015_compact_rows SET public_text=public_text WHERE id='11111111-1111-4111-8111-111111111111'`, false, false},
		{"unmatching-row-version", `UPDATE v015_compact_rows SET secret='outside-new',record_version=record_version+1,updated_at=updated_at+interval '1 second' WHERE id='33333333-3333-4333-8333-333333333333'`, false, false},
		{"unmatching-source-label", `UPDATE v015_compact_sources SET display='Outside renamed' WHERE id='66666666-6666-4666-8666-666666666666'`, false, false},
		{"visible-text", `UPDATE v015_compact_rows SET public_text='visible-new',record_version=record_version+1,updated_at=updated_at+interval '1 second' WHERE id='11111111-1111-4111-8111-111111111111'`, false, true},
		{"hidden-field-but-visible-version", `UPDATE v015_compact_rows SET secret='other-new',record_version=record_version+1,updated_at=updated_at+interval '1 second' WHERE id='22222222-2222-4222-8222-222222222222'`, false, true},
		{"numeric-sort-reorder", `UPDATE v015_compact_rows SET number=30,record_version=record_version+1,updated_at=updated_at+interval '1 second' WHERE id='11111111-1111-4111-8111-111111111111'`, false, true},
		{"actual-visible-reference-label", `UPDATE v015_compact_sources SET display='North renamed' WHERE id='44444444-4444-4444-8444-444444444444'`, false, true},
		{"actual-visible-reference-deletion", `UPDATE v015_compact_sources SET deleted=true WHERE id='44444444-4444-4444-8444-444444444444'`, false, true},
		{"effective-field-mask", `SELECT 1`, true, true},
		{"membership-entry", `UPDATE v015_compact_rows SET number=40,record_version=record_version+1,updated_at=updated_at+interval '1 second' WHERE id='33333333-3333-4333-8333-333333333333'`, true, true},
		{"reference-assignment", `UPDATE v015_compact_rows SET ref_id='44444444-4444-4444-8444-444444444444',record_version=record_version+1,updated_at=updated_at+interval '1 second' WHERE id='22222222-2222-4222-8222-222222222222'`, true, true},
	}
	for _, stage := range stages {
		if _, err = db.Exec(ctx, stage.sql); err != nil {
			t.Fatalf("%s mutation: %v", stage.name, err)
		}
		current, e := observeCompactFixture(ctx, db, stage.secretAll, false)
		if e != nil {
			t.Fatalf("%s observation: %v", stage.name, e)
		}
		fullChanged := current.full != previous.full || current.count != previous.count
		compactChanged := current.compact != previous.compact
		if fullChanged != stage.wantChanged || compactChanged != fullChanged {
			t.Fatalf("%s: oracle changed=%v compact changed=%v want=%v count=%d", stage.name, fullChanged, compactChanged, stage.wantChanged, current.count)
		}
		previous = current
	}
	// The source digest is over actual visible refs, not every matching row's
	// source or a global sourceRevision. Changing only an other-row label is
	// irrelevant after the reference field becomes own-only.
	refOwn, err := observeCompactFixture(ctx, db, true, true)
	if err != nil || refOwn.full == previous.full || refOwn.compact == previous.compact {
		t.Fatalf("own reference mask not reflected: %+v %v", refOwn, err)
	}
	previous = refOwn
	if _, err = db.Exec(ctx, `UPDATE v015_compact_sources SET display='South renamed' WHERE id='55555555-5555-4555-8555-555555555555'`); err != nil {
		t.Fatal(err)
	}
	maskedSource, err := observeCompactFixture(ctx, db, true, true)
	if err != nil || maskedSource.full != previous.full || maskedSource.compact != previous.compact {
		t.Fatalf("masked reference label changed result: %+v %v", maskedSource, err)
	}
	previous = maskedSource
	// Explicit counterexample: a writer that skips the version increment makes
	// compact A unsound. This must never be adopted until every writer is covered.
	if _, err = db.Exec(ctx, `UPDATE v015_compact_rows SET public_text='bad-writer' WHERE id='11111111-1111-4111-8111-111111111111'`); err != nil {
		t.Fatal(err)
	}
	broken, err := observeCompactFixture(ctx, db, true, true)
	if err != nil {
		t.Fatal(err)
	}
	if broken.full == previous.full || broken.compact != previous.compact {
		t.Fatal("missing recordVersion invariant did not expose candidate false negative")
	}
	if !candidateChanged(previous, broken, 1, 2) || candidateChanged(previous, previous, 1, 2) {
		t.Fatal("schemaVersion transition must invoke full oracle without coarse refresh")
	}
}

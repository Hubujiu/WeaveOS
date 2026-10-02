package appschema

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type guardFunc func(context.Context, pgx.Tx, string) error

func (f guardFunc) Check(c context.Context, t pgx.Tx, id string) error { return f(c, t, id) }

type dependencyFunc func(context.Context, pgx.Tx, string, []string) ([]Reference, error)

func (f dependencyFunc) Protect(c context.Context, t pgx.Tx, id string, fields []string) ([]Reference, error) {
	return f(c, t, id, fields)
}

type confirmationFunc func(context.Context, pgx.Tx, string, []ColumnImpact) error

func (f confirmationFunc) Verify(c context.Context, t pgx.Tx, id string, impact []ColumnImpact) error {
	return f(c, t, id, impact)
}

// This is a PRIVATE integration fixture, not B1's metadata or B3's flow schema.
type fixtureMetadata struct {
	namespace   string
	storeFault  error
	beforeStore func()
}

func (m *fixtureMetadata) Lock(c context.Context, tx pgx.Tx, id string) (Snapshot, error) {
	var s Snapshot
	var fields []byte
	err := tx.QueryRow(c, "SELECT present,revision::text,fields FROM "+pgx.Identifier{m.namespace, "fixture_metadata"}.Sanitize()+" WHERE id=$1 FOR UPDATE", id).Scan(&s.Exists, &s.Revision, &fields)
	if err == nil {
		err = json.Unmarshal(fields, &s.Fields)
	}
	return s, err
}
func (m *fixtureMetadata) Store(c context.Context, tx pgx.Tx, id string, before Snapshot, fields []Field) (string, error) {
	if m.beforeStore != nil {
		m.beforeStore()
	}
	b, err := json.Marshal(fields)
	if err != nil {
		return "", err
	}
	var revision string
	err = tx.QueryRow(c, "UPDATE "+pgx.Identifier{m.namespace, "fixture_metadata"}.Sanitize()+" SET present=true,fields=$2,revision=revision+1 WHERE id=$1 RETURNING revision::text", id, b).Scan(&revision)
	if err == nil && m.storeFault != nil {
		err = m.storeFault
	}
	return revision, err
}

type fixture struct {
	pool      *pgxpool.Pool
	namespace string
	executor  Executor
	metadata  *fixtureMetadata
}

func (f *fixture) table() string { return pgx.Identifier{f.namespace, tableName}.Sanitize() }
func (f *fixture) request(fields []Field) Request {
	return Request{TableID: tableID, ExpectedRevision: "0", Fields: fields}
}

func newFixture(t *testing.T, before []Field, rows ...[]any) *fixture {
	t.Helper()
	raw := os.Getenv("WEAVEOS_B2_TEST_DATABASE_URL")
	if raw == "" {
		t.Fatal("WEAVEOS_B2_TEST_DATABASE_URL must identify the dedicated temporary B2 PostgreSQL database")
	}
	cfg, err := pgxpool.ParseConfig(raw)
	if err != nil {
		t.Fatal("invalid B2 test database configuration")
	}
	if cfg.ConnConfig.Database != "weaveos_b2_isolated_test" || !strings.HasPrefix(cfg.ConnConfig.Host, "/tmp/weaveos-b2-") {
		t.Fatal("DDL tests require the dedicated B2 test database and private Unix socket; refusing another database")
	}
	cfg.MaxConns = 8
	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	var version int
	var actualDB string
	if err = pool.QueryRow(context.Background(), "SELECT current_setting('server_version_num')::int,current_database()").Scan(&version, &actualDB); err != nil || version < 180000 || actualDB != "weaveos_b2_isolated_test" {
		t.Fatalf("dedicated PostgreSQL 18 fixture unavailable: version=%d database=%s error=%v", version, actualDB, err)
	}
	var suffix [8]byte
	if _, err = rand.Read(suffix[:]); err != nil {
		t.Fatal(err)
	}
	ns := "b2_" + hex.EncodeToString(suffix[:])
	qns := pgx.Identifier{ns}.Sanitize()
	if _, err = pool.Exec(context.Background(), "CREATE SCHEMA "+qns); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		c, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if _, e := pool.Exec(c, "DROP SCHEMA "+qns+" CASCADE"); e != nil {
			t.Errorf("cleanup isolated fixture: %v", e)
		}
	})
	meta := &fixtureMetadata{namespace: ns}
	f := &fixture{pool: pool, namespace: ns, metadata: meta}
	for _, sql := range []string{"CREATE TABLE " + qns + ".fixture_metadata(id text PRIMARY KEY,present boolean NOT NULL,revision bigint NOT NULL,fields jsonb NOT NULL)", "CREATE TABLE " + qns + ".fixture_references(field_id text,kind text,id text)"} {
		if _, err = pool.Exec(context.Background(), sql); err != nil {
			t.Fatal(err)
		}
	}
	b, _ := json.Marshal(before)
	if _, err = pool.Exec(context.Background(), "INSERT INTO "+qns+".fixture_metadata VALUES($1,$2,0,$3)", tableID, before != nil, b); err != nil {
		t.Fatal(err)
	}
	if before != nil {
		var defs, cols, args []string
		for i, field := range before {
			col := fixtureColumn(field.ID)
			def := pgx.Identifier{col}.Sanitize() + " " + string(field.Type)
			if field.Default != nil {
				switch field.Default.Type {
				case Boolean:
					if field.Default.Boolean {
						def += " DEFAULT true"
					} else {
						def += " DEFAULT false"
					}
				case Text:
					if field.Default.Text != "true" {
						t.Fatal("unexpected fixture default")
					}
					def += " DEFAULT 'true'"
				}
			}
			defs = append(defs, def)
			cols = append(cols, pgx.Identifier{col}.Sanitize())
			args = append(args, fmt.Sprintf("$%d", i+1))
		}
		if _, err = pool.Exec(context.Background(), "CREATE TABLE "+f.table()+" ("+strings.Join(defs, ",")+")"); err != nil {
			t.Fatal(err)
		}
		for _, row := range rows {
			if _, err = pool.Exec(context.Background(), "INSERT INTO "+f.table()+" ("+strings.Join(cols, ",")+") VALUES ("+strings.Join(args, ",")+")", row...); err != nil {
				t.Fatal(err)
			}
		}
	}
	f.executor = Executor{DB: pool, Namespace: ns, Metadata: meta, Guard: guardFunc(func(context.Context, pgx.Tx, string) error { return nil }), Limits: Limits{LockTimeout: time.Second, StatementTimeout: 3 * time.Second}}
	f.executor.Dependencies = dependencyFunc(func(c context.Context, tx pgx.Tx, id string, fields []string) ([]Reference, error) {
		// Registration in the fixture uses the same metadata row gate as Lock.
		r, err := tx.Query(c, "SELECT field_id,kind,id FROM "+qns+".fixture_references WHERE field_id=ANY($1::text[]) ORDER BY id", fields)
		if err != nil {
			return nil, err
		}
		defer r.Close()
		var refs []Reference
		for r.Next() {
			var ref Reference
			if err = r.Scan(&ref.FieldID, &ref.Kind, &ref.ID); err != nil {
				return nil, err
			}
			refs = append(refs, ref)
		}
		return refs, r.Err()
	})
	return f
}
func fixtureColumn(id string) string {
	switch id {
	case fieldID1:
		return column1
	case fieldID2:
		return column2
	case fieldID3:
		return column3
	}
	panic("unknown independent fixture ID")
}
func assertState(t *testing.T, f *fixture, columns []string, revision string) {
	t.Helper()
	var got []string
	var rev string
	if err := f.pool.QueryRow(context.Background(), "SELECT array_agg(column_name::text ORDER BY ordinal_position) FROM information_schema.columns WHERE table_schema=$1 AND table_name=$2", f.namespace, tableName).Scan(&got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, columns) {
		t.Fatalf("physical columns: %v, expected %v", got, columns)
	}
	if err := f.pool.QueryRow(context.Background(), "SELECT revision::text FROM "+pgx.Identifier{f.namespace, "fixture_metadata"}.Sanitize()).Scan(&rev); err != nil {
		t.Fatal(err)
	}
	if rev != revision {
		t.Fatalf("metadata revision: %s, expected %s", rev, revision)
	}
}

func TestSaveCreatesRealTypedTableOnlyAtSave(t *testing.T) {
	f := newFixture(t, nil)
	after := []Field{{ID: fieldID1, Name: "任意显示名", Type: Text}, {ID: fieldID2, Type: Boolean}}
	if _, err := BuildPlan(tableID, nil, after); err != nil {
		t.Fatalf("pure plan: %v", err)
	}
	var present bool
	if err := f.pool.QueryRow(context.Background(), "SELECT to_regclass($1) IS NOT NULL", f.table()).Scan(&present); err != nil || present {
		t.Fatal("plan must not create the physical table")
	}
	result, err := f.executor.Save(context.Background(), f.request(after))
	if err != nil {
		t.Fatalf("Save must create physical typed table: %v", err)
	}
	assertState(t, f, []string{column1, column2}, "1")
	if result.Revision != "1" {
		t.Fatal("metadata result missing")
	}
	var types []string
	if err = f.pool.QueryRow(context.Background(), "SELECT array_agg(data_type::text ORDER BY ordinal_position) FROM information_schema.columns WHERE table_schema=$1 AND table_name=$2", f.namespace, tableName).Scan(&types); err != nil || !reflect.DeepEqual(types, []string{"text", "boolean"}) {
		t.Fatalf("real types, no JSONB/EAV: %v %v", types, err)
	}
}

func TestSaveNewColumnDefaultsAndNull(t *testing.T) {
	f := newFixture(t, []Field{{ID: fieldID1, Type: Text}}, []any{"old"})
	after := []Field{{ID: fieldID1, Type: Text}, {ID: fieldID2, Type: Boolean, Default: boolValue(true)}, {ID: fieldID3, Type: Text}}
	if _, err := f.executor.Save(context.Background(), f.request(after)); err != nil {
		t.Fatalf("default/NULL on existing records: %v", err)
	}
	var value bool
	var empty *string
	if err := f.pool.QueryRow(context.Background(), "SELECT "+column2+","+column3+" FROM "+f.table()).Scan(&value, &empty); err != nil || !value || empty != nil {
		t.Fatalf("expected true and NULL: %v %v %v", value, empty, err)
	}
}

func TestSaveRequiredOldRowsNeedDefaultOrBackfill(t *testing.T) {
	for _, mode := range []string{"missing", "default", "backfill", "empty"} {
		t.Run(mode, func(t *testing.T) {
			var rows [][]any
			if mode != "empty" {
				rows = [][]any{{"old"}}
			}
			f := newFixture(t, []Field{{ID: fieldID1, Type: Text}}, rows...)
			after := []Field{{ID: fieldID1, Type: Text}, {ID: fieldID2, Type: Boolean, Required: true}}
			req := f.request(after)
			if mode == "default" {
				req.Fields[1].Default = boolValue(false)
			}
			if mode == "backfill" {
				req.Backfills = map[string]Value{fieldID2: {Type: Boolean, Boolean: true}}
			}
			_, err := f.executor.Save(context.Background(), req)
			if mode == "missing" {
				if !errors.Is(err, ErrRequiredBackfill) {
					t.Fatalf("required existing rows must fail without default/backfill: %v", err)
				}
				assertState(t, f, []string{column1}, "0")
				return
			}
			if err != nil {
				t.Fatalf("required %s Save: %v", mode, err)
			}
			assertState(t, f, []string{column1, column2}, "1")
			var nullable string
			if err = f.pool.QueryRow(context.Background(), "SELECT is_nullable FROM information_schema.columns WHERE table_schema=$1 AND column_name=$2", f.namespace, column2).Scan(&nullable); err != nil || nullable != "NO" {
				t.Fatal("required column must truly be NOT NULL")
			}
			if mode != "empty" {
				var v bool
				if err = f.pool.QueryRow(context.Background(), "SELECT "+column2+" FROM "+f.table()).Scan(&v); err != nil || v != (mode == "backfill") {
					t.Fatalf("required old value: %v %v", v, err)
				}
			}
		})
	}
}

func TestSaveIncompatibleConversionRollsBackAllDDL(t *testing.T) {
	f := newFixture(t, []Field{{ID: fieldID1, Type: Text}}, []any{"true"}, []any{"cannot convert"})
	req := f.request([]Field{{ID: fieldID2, Type: Boolean, Default: boolValue(true)}, {ID: fieldID1, Type: Boolean}})
	_, err := f.executor.Save(context.Background(), req)
	var pg *pgconn.PgError
	if !errors.As(err, &pg) || pg.Code != "22P02" {
		t.Fatalf("actual incompatible data must fail at PostgreSQL cast: %v", err)
	}
	assertState(t, f, []string{column1}, "0")
	var values []string
	if err = f.pool.QueryRow(context.Background(), "SELECT array_agg("+column1+" ORDER BY "+column1+") FROM "+f.table()).Scan(&values); err != nil || !reflect.DeepEqual(values, []string{"cannot convert", "true"}) {
		t.Fatalf("old values must survive whole Save failure: %v %v", values, err)
	}
}

func TestSaveCompatiblePhysicalConversion(t *testing.T) {
	f := newFixture(t, []Field{{ID: fieldID1, Type: Text}}, []any{"true"}, []any{"false"})
	if _, err := f.executor.Save(context.Background(), f.request([]Field{{ID: fieldID1, Type: Boolean}})); err != nil {
		t.Fatalf("explicit physical primitive cast proof: %v", err)
	}
	var values []bool
	if err := f.pool.QueryRow(context.Background(), "SELECT array_agg("+column1+" ORDER BY "+column1+") FROM "+f.table()).Scan(&values); err != nil || !reflect.DeepEqual(values, []bool{false, true}) {
		t.Fatalf("typed values: %v %v", values, err)
	}
}

func TestSaveTypeChangeReplacesOldDefaultAtomically(t *testing.T) {
	f := newFixture(t, []Field{{ID: fieldID1, Type: Text, Default: textValue("true")}}, []any{"true"})
	if _, err := f.executor.Save(context.Background(), f.request([]Field{{ID: fieldID1, Type: Boolean, Default: boolValue(false)}})); err != nil {
		t.Fatalf("old default must not obstruct explicit physical cast: %v", err)
	}
	if _, err := f.pool.Exec(context.Background(), "INSERT INTO "+f.table()+" DEFAULT VALUES"); err != nil {
		t.Fatal(err)
	}
	var values []bool
	if err := f.pool.QueryRow(context.Background(), "SELECT array_agg("+column1+" ORDER BY "+column1+") FROM "+f.table()).Scan(&values); err != nil || !reflect.DeepEqual(values, []bool{false, true}) {
		t.Fatalf("old value true/new default false: %v %v", values, err)
	}
}

func TestSaveDefaultChangeOnlyAffectsFutureRecords(t *testing.T) {
	f := newFixture(t, []Field{{ID: fieldID1, Type: Boolean, Default: boolValue(true)}}, []any{true})
	if _, err := f.executor.Save(context.Background(), f.request([]Field{{ID: fieldID1, Type: Boolean, Default: boolValue(false)}})); err != nil {
		t.Fatalf("typed default change: %v", err)
	}
	if _, err := f.pool.Exec(context.Background(), "INSERT INTO "+f.table()+" DEFAULT VALUES"); err != nil {
		t.Fatal(err)
	}
	var values []bool
	if err := f.pool.QueryRow(context.Background(), "SELECT array_agg("+column1+" ORDER BY "+column1+") FROM "+f.table()).Scan(&values); err != nil || !reflect.DeepEqual(values, []bool{false, true}) {
		t.Fatalf("default change must preserve old values: %v %v", values, err)
	}
}

func TestSaveExistingNullsCannotBecomeRequiredAndPartialDDLRemainsRolledBack(t *testing.T) {
	f := newFixture(t, []Field{{ID: fieldID1, Type: Text}}, []any{nil})
	_, err := f.executor.Save(context.Background(), f.request([]Field{{ID: fieldID2, Type: Boolean}, {ID: fieldID1, Type: Text, Required: true}}))
	var pg *pgconn.PgError
	if !errors.As(err, &pg) || pg.Code != "23502" {
		t.Fatalf("real NULL constraint must reject whole Save: %v", err)
	}
	assertState(t, f, []string{column1}, "0")
}

func TestSaveBackfillCannotOverwriteExistingOrUnknownFields(t *testing.T) {
	for _, id := range []string{fieldID1, fieldID3} {
		t.Run(id, func(t *testing.T) {
			f := newFixture(t, []Field{{ID: fieldID1, Type: Text}}, []any{"kept"})
			req := f.request([]Field{{ID: fieldID1, Type: Text}, {ID: fieldID2, Type: Boolean}})
			req.Backfills = map[string]Value{id: {Type: Text, Text: "overwrite"}}
			if _, err := f.executor.Save(context.Background(), req); !errors.Is(err, ErrInvalid) {
				t.Fatalf("backfill must be limited to new typed fields: %v", err)
			}
			assertState(t, f, []string{column1}, "0")
		})
	}
}

func TestSaveDependencyRegistrationCannotRaceDelete(t *testing.T) {
	f := newFixture(t, []Field{{ID: fieldID1, Type: Text}})
	ctx := context.Background()
	registration, err := f.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer registration.Rollback(ctx)
	if _, err = f.metadata.Lock(ctx, registration, tableID); err != nil {
		t.Fatal(err)
	}
	if _, err = registration.Exec(ctx, "INSERT INTO "+pgx.Identifier{f.namespace, "fixture_references"}.Sanitize()+" VALUES($1,'enabled_flow','registered-before-save')", fieldID1); err != nil {
		t.Fatal(err)
	}
	started := make(chan uint32, 1)
	f.executor.Guard = guardFunc(func(c context.Context, tx pgx.Tx, id string) error {
		var pid uint32
		err := tx.QueryRow(c, "SELECT pg_backend_pid()").Scan(&pid)
		started <- pid
		return err
	})
	finished := make(chan error, 1)
	go func() { _, err := f.executor.Save(ctx, f.request(nil)); finished <- err }()
	var pid uint32
	select {
	case pid = <-started:
	case err = <-finished:
		t.Fatalf("Save must reach its transaction: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("Save did not start")
	}
	deadline := time.NewTimer(3 * time.Second)
	defer deadline.Stop()
	tick := time.NewTicker(5 * time.Millisecond)
	defer tick.Stop()
	for {
		var blocked bool
		if err = f.pool.QueryRow(ctx, "SELECT cardinality(pg_blocking_pids($1))>0", pid).Scan(&blocked); err != nil {
			t.Fatal(err)
		}
		if blocked {
			break
		}
		select {
		case err = <-finished:
			t.Fatalf("Save bypassed registration gate: %v", err)
		case <-deadline.C:
			t.Fatal("Save did not wait on the real registration lock")
		case <-tick.C:
		}
	}
	if err = registration.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case err = <-finished:
		var blocked *DependencyError
		if !errors.As(err, &blocked) || len(blocked.References) != 1 || blocked.References[0].ID != "registered-before-save" {
			t.Fatalf("new committed reference must block delete: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Save did not finish after registration")
	}
	assertState(t, f, []string{column1}, "0")
}

func TestSaveMetadataFailureRollsBackPhysicalAndMetadata(t *testing.T) {
	f := newFixture(t, []Field{{ID: fieldID1, Name: "old", Type: Text}}, []any{"old value"})
	fault := errors.New("injected metadata/layout/audit failure")
	f.metadata.storeFault = fault
	_, err := f.executor.Save(context.Background(), f.request([]Field{{ID: fieldID1, Name: "new", Type: Text}, {ID: fieldID2, Type: Boolean}}))
	if !errors.Is(err, fault) {
		t.Fatalf("fixture metadata/audit failure must be returned: %v", err)
	}
	assertState(t, f, []string{column1}, "0")
	var fields []byte
	if err = f.pool.QueryRow(context.Background(), "SELECT fields FROM "+pgx.Identifier{f.namespace, "fixture_metadata"}.Sanitize()).Scan(&fields); err != nil || !strings.Contains(string(fields), "old") {
		t.Fatal("metadata must roll back too")
	}
}

func TestSaveDeletionRequiresCurrentImpactConfirmation(t *testing.T) {
	f := newFixture(t, []Field{{ID: fieldID1, Type: Text}, {ID: fieldID2, Type: Boolean}}, []any{"first", true}, []any{nil, false})
	req := f.request([]Field{{ID: fieldID2, Type: Boolean}})
	_, err := f.executor.Save(context.Background(), req)
	var needed *ConfirmationError
	if !errors.As(err, &needed) || !reflect.DeepEqual(needed.Impacts, []ColumnImpact{{FieldID: fieldID1, NonNullRows: 1}}) {
		t.Fatalf("populated delete must report current affected field/count: %v", err)
	}
	assertState(t, f, []string{column1, column2}, "0")
	// Same count, different record data. The root-owned proof must detect it.
	if _, err = f.pool.Exec(context.Background(), "UPDATE "+f.table()+" SET "+column1+"='changed' WHERE "+column1+"='first'"); err != nil {
		t.Fatal(err)
	}
	stale := errors.New("stale user consent")
	req.Confirmation = confirmationFunc(func(c context.Context, tx pgx.Tx, id string, impact []ColumnImpact) error {
		var current string
		if e := tx.QueryRow(c, "SELECT "+column1+" FROM "+f.table()+" WHERE "+column1+" IS NOT NULL").Scan(&current); e != nil {
			return e
		}
		if current != "first" {
			return stale
		}
		return nil
	})
	if _, err = f.executor.Save(context.Background(), req); !errors.Is(err, stale) {
		t.Fatalf("same-count changed data must reject stale confirmation: %v", err)
	}
	assertState(t, f, []string{column1, column2}, "0")
	req.Confirmation = confirmationFunc(func(c context.Context, tx pgx.Tx, id string, impact []ColumnImpact) error {
		var current string
		if e := tx.QueryRow(c, "SELECT "+column1+" FROM "+f.table()+" WHERE "+column1+" IS NOT NULL").Scan(&current); e != nil {
			return e
		}
		if current != "changed" || !reflect.DeepEqual(impact, []ColumnImpact{{FieldID: fieldID1, NonNullRows: 1}}) {
			return stale
		}
		return nil
	})
	if _, err = f.executor.Save(context.Background(), req); err != nil {
		t.Fatalf("current consent must allow isolated delete: %v", err)
	}
	assertState(t, f, []string{column2}, "1")
}

func TestSaveNullOnlyColumnNeedsNoDataDeletionConsent(t *testing.T) {
	f := newFixture(t, []Field{{ID: fieldID1, Type: Text}, {ID: fieldID2, Type: Boolean}}, []any{nil, true})
	if _, err := f.executor.Save(context.Background(), f.request([]Field{{ID: fieldID2, Type: Boolean}})); err != nil {
		t.Fatalf("NULL-only column has no populated data to confirm: %v", err)
	}
	assertState(t, f, []string{column2}, "1")
}

func TestSaveDependenciesBlockDeleteAndTypeChange(t *testing.T) {
	for _, mode := range []string{"delete", "type"} {
		t.Run(mode, func(t *testing.T) {
			f := newFixture(t, []Field{{ID: fieldID1, Type: Text}}, []any{"true"})
			q := pgx.Identifier{f.namespace, "fixture_references"}.Sanitize()
			if _, err := f.pool.Exec(context.Background(), "INSERT INTO "+q+" VALUES($1,'enabled_flow','flow-1'),($1,'in_flight_instance','instance-1')", fieldID1); err != nil {
				t.Fatal(err)
			}
			var after []Field
			if mode == "type" {
				after = []Field{{ID: fieldID1, Type: Boolean}}
			}
			req := f.request(after)
			var consentCalls int
			req.Confirmation = confirmationFunc(func(context.Context, pgx.Tx, string, []ColumnImpact) error { consentCalls++; return nil })
			_, err := f.executor.Save(context.Background(), req)
			var blocked *DependencyError
			if !errors.As(err, &blocked) || len(blocked.References) != 2 {
				t.Fatalf("enabled/in-flight references must be reported: %v", err)
			}
			if consentCalls != 0 {
				t.Fatal("consent must not bypass dependencies")
			}
			assertState(t, f, []string{column1}, "0")
		})
	}
}

func TestSaveMissingOrFailedDependencyProtectionIsClosed(t *testing.T) {
	for _, missing := range []bool{true, false} {
		t.Run(fmt.Sprint(missing), func(t *testing.T) {
			f := newFixture(t, []Field{{ID: fieldID1, Type: Text}})
			want := ErrDependenciesUnavailable
			if missing {
				f.executor.Dependencies = nil
			} else {
				want = errors.New("dependency registry unavailable")
				f.executor.Dependencies = dependencyFunc(func(context.Context, pgx.Tx, string, []string) ([]Reference, error) { return nil, want })
			}
			_, err := f.executor.Save(context.Background(), f.request(nil))
			if !errors.Is(err, want) {
				t.Fatalf("cannot treat missing/failed registry as no references: %v", err)
			}
			assertState(t, f, []string{column1}, "0")
		})
	}
}

func TestSaveDisplayRenameKeepsDataAndPhysicalIdentifier(t *testing.T) {
	f := newFixture(t, []Field{{ID: fieldID1, Name: "old", Type: Text}}, []any{"kept"})
	r, err := f.executor.Save(context.Background(), f.request([]Field{{ID: fieldID1, Name: `显示名";DROP TABLE x;--`, Type: Text}}))
	if err != nil {
		t.Fatalf("rename metadata Save: %v", err)
	}
	if len(r.Plan.Changes) != 0 {
		t.Fatal("rename must not issue physical schema changes")
	}
	assertState(t, f, []string{column1}, "1")
	var value string
	if err = f.pool.QueryRow(context.Background(), "SELECT "+column1+" FROM "+f.table()).Scan(&value); err != nil || value != "kept" {
		t.Fatal("rename must preserve records")
	}
}

func TestSaveTypedDefaultIsDataNotSQL(t *testing.T) {
	f := newFixture(t, []Field{{ID: fieldID1, Type: Text}}, []any{"old"})
	value := `'); DROP SCHEMA auth CASCADE;--\quote'`
	if _, err := f.executor.Save(context.Background(), f.request([]Field{{ID: fieldID1, Type: Text}, {ID: fieldID2, Type: Text, Default: textValue(value)}})); err != nil {
		t.Fatalf("quoted typed default: %v", err)
	}
	var got string
	if err := f.pool.QueryRow(context.Background(), "SELECT "+column2+" FROM "+f.table()).Scan(&got); err != nil || got != value {
		t.Fatalf("default must round-trip literally: %q %v", got, err)
	}
	assertState(t, f, []string{column1, column2}, "1")
}

func TestSaveGuardDenialAndRevisionConflictDoNotMutate(t *testing.T) {
	for _, mode := range []string{"guard", "revision", "missing_guard", "limits"} {
		t.Run(mode, func(t *testing.T) {
			f := newFixture(t, []Field{{ID: fieldID1, Type: Text}})
			req := f.request([]Field{{ID: fieldID1, Type: Text}, {ID: fieldID2, Type: Boolean}})
			want := ErrInvalid
			switch mode {
			case "guard":
				want = errors.New("not permitted")
				f.executor.Guard = guardFunc(func(context.Context, pgx.Tx, string) error { return want })
			case "revision":
				want = ErrRevisionConflict
				req.ExpectedRevision = "stale"
			case "missing_guard":
				f.executor.Guard = nil
			case "limits":
				f.executor.Limits = Limits{}
			}
			_, err := f.executor.Save(context.Background(), req)
			if !errors.Is(err, want) {
				t.Fatalf("guard/conflict/config must reject before mutations: %v", err)
			}
			assertState(t, f, []string{column1}, "0")
		})
	}
}

func TestSaveLockTimeoutRollsBack(t *testing.T) {
	f := newFixture(t, []Field{{ID: fieldID1, Type: Text}})
	ctx := context.Background()
	blocker, err := f.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer blocker.Rollback(ctx)
	if _, err = blocker.Exec(ctx, "LOCK TABLE "+f.table()+" IN ACCESS SHARE MODE"); err != nil {
		t.Fatal(err)
	}
	f.executor.Limits = Limits{LockTimeout: 50 * time.Millisecond, StatementTimeout: time.Second}
	_, err = f.executor.Save(ctx, f.request([]Field{{ID: fieldID1, Type: Text}, {ID: fieldID2, Type: Boolean}}))
	var pg *pgconn.PgError
	if !errors.As(err, &pg) || pg.Code != "55P03" {
		t.Fatalf("real schema lock timeout required: %v", err)
	}
	assertState(t, f, []string{column1}, "0")
}

func TestSaveStatementTimeoutAndCancellationRollBack(t *testing.T) {
	for _, mode := range []string{"statement", "cancel"} {
		t.Run(mode, func(t *testing.T) {
			f := newFixture(t, []Field{{ID: fieldID1, Type: Text}})
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if mode == "statement" {
				f.executor.Limits.StatementTimeout = 20 * time.Millisecond
				f.executor.Guard = guardFunc(func(c context.Context, tx pgx.Tx, id string) error {
					_, err := tx.Exec(c, "SELECT pg_sleep(0.08)")
					return err
				})
			} else {
				f.metadata.beforeStore = cancel
			}
			_, err := f.executor.Save(ctx, f.request([]Field{{ID: fieldID1, Type: Text}, {ID: fieldID2, Type: Boolean}}))
			if mode == "statement" {
				var pg *pgconn.PgError
				if !errors.As(err, &pg) || pg.Code != "57014" {
					t.Fatalf("real statement timeout must fail whole Save: %v", err)
				}
			} else if !errors.Is(err, context.Canceled) {
				t.Fatalf("canceled request must fail before commit: %v", err)
			}
			assertState(t, f, []string{column1}, "0")
		})
	}
}

func TestSaveConcurrentRevisionCAS(t *testing.T) {
	f := newFixture(t, []Field{{ID: fieldID1, Type: Text}})
	entered := make(chan struct{})
	release := make(chan struct{})
	var stores atomic.Int32
	f.metadata.beforeStore = func() {
		if stores.Add(1) == 1 {
			close(entered)
			<-release
		}
	}
	err1 := make(chan error, 1)
	go func() {
		_, err := f.executor.Save(context.Background(), f.request([]Field{{ID: fieldID1, Type: Text}, {ID: fieldID2, Type: Boolean}}))
		err1 <- err
	}()
	select {
	case <-entered:
	case err := <-err1:
		t.Fatalf("first transaction must reach metadata after DDL: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("first Save did not reach synchronization barrier")
	}
	defer func() {
		select {
		case <-release:
		default:
			close(release)
		}
	}()
	err2 := make(chan error, 1)
	go func() {
		_, err := f.executor.Save(context.Background(), f.request([]Field{{ID: fieldID1, Type: Text}, {ID: fieldID3, Type: Text}}))
		err2 <- err
	}()
	close(release)
	if err := <-err1; err != nil {
		t.Fatalf("first Save: %v", err)
	}
	if err := <-err2; !errors.Is(err, ErrRevisionConflict) {
		t.Fatalf("concurrent stale schema Save must conflict: %v", err)
	}
	assertState(t, f, []string{column1, column2}, "1")
}

type commitLossDB struct {
	pool    *pgxpool.Pool
	commits *atomic.Int32
}

func (d commitLossDB) BeginTx(c context.Context, o pgx.TxOptions) (pgx.Tx, error) {
	tx, err := d.pool.BeginTx(c, o)
	if err != nil {
		return nil, err
	}
	return commitLossTx{Tx: tx, commits: d.commits}, nil
}

type commitLossTx struct {
	pgx.Tx
	commits *atomic.Int32
}

func (t commitLossTx) Commit(c context.Context) error {
	t.commits.Add(1)
	if err := t.Tx.Commit(c); err != nil {
		return err
	}
	return io.ErrUnexpectedEOF
}

func TestSaveCommitUncertaintyDoesNotRetryOrReportRollback(t *testing.T) {
	f := newFixture(t, []Field{{ID: fieldID1, Type: Text}})
	var commits atomic.Int32
	f.executor.DB = commitLossDB{pool: f.pool, commits: &commits}
	_, err := f.executor.Save(context.Background(), f.request([]Field{{ID: fieldID1, Type: Text}, {ID: fieldID2, Type: Boolean}}))
	if !errors.Is(err, ErrCommitUnknown) || !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("injected lost result after REAL commit is unknown, not no-effect: %v", err)
	}
	if commits.Load() != 1 {
		t.Fatalf("ambiguous Save must not be replayed: %d commits", commits.Load())
	}
	assertState(t, f, []string{column1, column2}, "1")
}

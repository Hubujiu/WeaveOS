package appschema

import (
	"context"
	"github.com/jackc/pgx/v5"
	"testing"
)

func TestCommonPhysicalTypesAreRealPGColumns(t *testing.T) {
	f := newFixture(t, nil)
	fields := []Field{{ID: fieldID1, Type: Numeric, Precision: 38, Scale: 2, Default: &Value{Type: Numeric, Text: "1.24"}}, {ID: fieldID2, Type: Date, Default: &Value{Type: Date, Text: "2024-02-29"}}, {ID: fieldID3, Type: Timestamp}}
	r, e := f.executor.Save(context.Background(), f.request(fields))
	if e != nil {
		t.Fatal("approved physical primitives unavailable", e)
	}
	if r.Revision != "1" {
		t.Fatal("save revision")
	}
	for _, expected := range []struct{ id, sqlType string }{{fieldID1, "numeric(38,2)"}, {fieldID2, "date"}, {fieldID3, "timestamp with time zone"}} {
		var typ string
		if e = f.pool.QueryRow(context.Background(), "SELECT format_type(atttypid,atttypmod) FROM pg_attribute WHERE attrelid=to_regclass($1) AND attname=$2", f.namespace+"."+tableName, physicalID("f_", expected.id)).Scan(&typ); e != nil || typ != expected.sqlType {
			t.Fatalf("field %s physical type %q want %q: %v", expected.id, typ, expected.sqlType, e)
		}
	}
	if _, e = f.pool.Exec(context.Background(), "INSERT INTO "+f.table()+" DEFAULT VALUES"); e != nil {
		t.Fatal(e)
	}
	var amount, date string
	var timestampAbsent bool
	columns := []string{pgx.Identifier{physicalID("f_", fieldID1)}.Sanitize(), pgx.Identifier{physicalID("f_", fieldID2)}.Sanitize(), pgx.Identifier{physicalID("f_", fieldID3)}.Sanitize()}
	if e = f.pool.QueryRow(context.Background(), "SELECT "+columns[0]+"::text,"+columns[1]+"::text,"+columns[2]+" IS NULL FROM "+f.table()).Scan(&amount, &date, &timestampAbsent); e != nil || amount != "1.24" || date != "2024-02-29" || !timestampAbsent {
		t.Fatalf("real native defaults must preserve decimal, leap date and NULL: %q %q %v %v", amount, date, timestampAbsent, e)
	}
}

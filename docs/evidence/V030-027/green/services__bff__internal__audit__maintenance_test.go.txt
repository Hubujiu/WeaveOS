package audit

import (
	"context"
	"encoding/json"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"os"
	"testing"
	"time"
)

func pool(t *testing.T, key string) *pgxpool.Pool {
	t.Helper()
	p, err := pgxpool.New(context.Background(), os.Getenv(key))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(p.Close)
	if err := p.Ping(context.Background()); err != nil {
		t.Fatal("isolated storage unavailable")
	}
	return p
}

func rolePool(t *testing.T, key, role string) *pgxpool.Pool {
	t.Helper()
	cfg, err := pgxpool.ParseConfig(os.Getenv(key))
	if err != nil {
		t.Fatal(err)
	}
	if role != "auth_reader" && role != "auth_maintenance" {
		t.Fatal("invalid isolated role")
	}
	cfg.AfterConnect = func(ctx context.Context, c *pgx.Conn) error { _, err := c.Exec(ctx, "SET ROLE "+role); return err }
	p, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(p.Close)
	return p
}

func TestControlledMaintenanceAndReaderUseTheirActualRestrictedRoles(t *testing.T) {
	l, _ := clean(t)
	if _, err := l.Exec(context.Background(), source(t, "infra/runtime/roles.sql")); err != nil {
		t.Fatal(err)
	}
	if _, err := db(t, "WEAVEOS_TEST_ARCHIVE_DATABASE_URL").Exec(context.Background(), source(t, "infra/runtime/cold-roles.sql")); err != nil {
		t.Fatal(err)
	}
	event(t, l, "auth.authentication_events", eventID, "2026-08-01T00:00:00Z", "controlled")
	maintLive := rolePool(t, "WEAVEOS_TEST_DATABASE_URL", "auth_maintenance")
	maintCold := rolePool(t, "WEAVEOS_TEST_ARCHIVE_DATABASE_URL", "auth_maintenance")
	if _, err := Maintain(context.Background(), maintLive, maintCold, now); err != nil {
		t.Fatal("controlled role cannot perform approved archive operation:", err)
	}
	event(t, l, "auth.authentication_events", "10000000-0000-4000-8000-000000000002", "2026-09-26T00:00:00Z", "reader")
	reader, request := readerFixture(t, l)
	reader.Live = rolePool(t, "WEAVEOS_TEST_DATABASE_URL", "auth_reader")
	reader.Authentication.DB = reader.Live
	events, err := reader.List(request, 100)
	if err != nil || len(events) != 1 {
		t.Fatal("restricted reader must read current hot events only")
	}
}
func clean(t *testing.T) (*pgxpool.Pool, *pgxpool.Pool) {
	t.Helper()
	l := pool(t, "WEAVEOS_TEST_DATABASE_URL")
	c := pool(t, "WEAVEOS_TEST_ARCHIVE_DATABASE_URL")
	for _, v := range []struct {
		p *pgxpool.Pool
		q string
	}{{l, "TRUNCATE applications.workflow_publications,applications.workflow_engine_receipts,applications.workflow_instances,applications.workflow_deployments,applications.workflow_versions,applications.workflow_definitions,applications.record_change_values,applications.record_change_events,applications.field_option_tombstones,applications.record_drafts,applications.record_write_audit,applications.record_command_fences,applications.table_field_dependencies,applications.form_views,applications.fields,applications.logical_tables,applications.directories,auth.authentication_events,auth.invitations,auth.password_credentials,auth.users,personnel.department_members,personnel.member_configuration,personnel.member_identities,personnel.drafts,personnel.table_presets,applications.operations,applications.grant_fields,applications.grants,applications.menu_resources,applications.group_members,applications.permission_groups,applications.apps"}, {c, "TRUNCATE archive.authentication_events"}} {
		if _, err := v.p.Exec(context.Background(), v.q); err != nil {
			t.Fatal(err)
		}
	}
	return l, c
}

const eventID = "10000000-0000-4000-8000-000000000001"

var now = time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)

func event(t *testing.T, p *pgxpool.Pool, table, id, at, request string) {
	t.Helper()
	if table != "auth.authentication_events" && table != "archive.authentication_events" {
		t.Fatal("invalid fixture table")
	}
	_, err := p.Exec(context.Background(), "INSERT INTO "+table+" (id,event_type,outcome,client_ip,user_agent,request_id,occurred_at) VALUES ($1,'login','failure','198.51.100.2','Synthetic/1',$2,$3)", id, request, at)
	if err != nil {
		t.Fatal(err)
	}
}
func count(t *testing.T, p *pgxpool.Pool, table string) int {
	t.Helper()
	var n int
	if err := p.QueryRow(context.Background(), "SELECT count(*) FROM "+table).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}
func TestMonthlyArchiveAndAnnualExpiry(t *testing.T) {
	l, c := clean(t)
	ctx := context.Background()
	event(t, l, "auth.authentication_events", eventID, "2026-08-31T23:59:59Z", "previous-month")
	event(t, l, "auth.authentication_events", "10000000-0000-4000-8000-000000000002", "2026-09-01T00:00:00Z", "current-month")
	event(t, l, "auth.authentication_events", "10000000-0000-4000-8000-000000000003", "2025-09-26T12:00:00Z", "expired-boundary")
	event(t, c, "archive.authentication_events", "10000000-0000-4000-8000-000000000004", "2025-09-26T11:59:59Z", "expired-cold")
	event(t, c, "archive.authentication_events", "10000000-0000-4000-8000-000000000005", "2025-09-26T12:00:01Z", "still-retained")
	result, err := Maintain(ctx, l, c, now)
	if err != nil {
		t.Fatal(err)
	}
	if count(t, l, "auth.authentication_events") != 1 || count(t, c, "archive.authentication_events") != 2 {
		t.Fatal("completed months must leave hot storage; exact one-calendar-year expiry applies in both databases")
	}
	if result.Archived != 1 || result.Expired != 2 {
		t.Fatal("maintenance must report actual committed row counts")
	}
	var ip, ua, request string
	if err := c.QueryRow(ctx, "SELECT client_ip::text,user_agent,request_id FROM archive.authentication_events WHERE id=$1", eventID).Scan(&ip, &ua, &request); err != nil {
		t.Fatal(err)
	}
	if ip != "198.51.100.2/32" || ua != "Synthetic/1" || request != "previous-month" {
		t.Fatal("archive must preserve original event fields")
	}
	result, err = Maintain(ctx, l, c, now)
	if err != nil || result != (Result{}) {
		t.Fatal("rerun must be idempotent")
	}
}
func TestArchiveConflictAndUnavailableColdPreserveHotHistory(t *testing.T) {
	l, c := clean(t)
	ctx := context.Background()
	event(t, l, "auth.authentication_events", eventID, "2026-08-01T00:00:00Z", "original")
	event(t, c, "archive.authentication_events", eventID, "2026-08-01T00:00:00Z", "conflicting-history")
	if _, err := Maintain(ctx, l, c, now); err == nil {
		t.Fatal("conflicting event-id must fail closed, never overwrite historical data")
	}
	if count(t, l, "auth.authentication_events") != 1 {
		t.Fatal("cold conflict must not delete hot history")
	}
	c.Close()
	if _, err := Maintain(ctx, l, c, now); err == nil {
		t.Fatal("cold failure must be reported")
	}
	if count(t, l, "auth.authentication_events") != 1 {
		t.Fatal("cold failure must preserve hot history")
	}
}
func TestCommittedCopyThenRetryIsSafe(t *testing.T) {
	l, c := clean(t)
	event(t, l, "auth.authentication_events", eventID, "2026-08-01T00:00:00Z", "same")
	event(t, c, "archive.authentication_events", eventID, "2026-08-01T00:00:00Z", "same")
	if _, err := Maintain(context.Background(), l, c, now); err != nil {
		t.Fatal(err)
	}
	if count(t, l, "auth.authentication_events") != 0 || count(t, c, "archive.authentication_events") != 1 {
		t.Fatal("retry after cold commit/hot interruption must retain exactly one event")
	}
}
func TestLeapYearRetentionUsesCalendarYear(t *testing.T) {
	l, c := clean(t)
	event(t, c, "archive.authentication_events", eventID, "2023-02-28T12:00:00Z", "year-boundary")
	event(t, c, "archive.authentication_events", "10000000-0000-4000-8000-000000000002", "2023-02-28T12:00:01Z", "retained")
	if _, err := Maintain(context.Background(), l, c, time.Date(2024, 2, 29, 12, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	if count(t, c, "archive.authentication_events") != 1 {
		t.Fatal("one year is a calendar interval, including leap-day boundary")
	}
}
func TestAuditReadRequiresCurrentBootstrapAndDoesNotReadCold(t *testing.T) {
	l, c := clean(t)
	event(t, l, "auth.authentication_events", eventID, "2026-09-26T00:00:00Z", "hot-only")
	event(t, c, "archive.authentication_events", "10000000-0000-4000-8000-000000000002", "2026-08-01T00:00:00Z", "cold-only")
	reader, request := readerFixture(t, l)
	events, err := reader.List(request, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatal("current Bootstrap must read hot audit, with no implicit cold lookup")
	}
	var e map[string]json.RawMessage
	if err := json.Unmarshal(events[0], &e); err != nil {
		t.Fatal(err)
	}
	if len(e) != 12 {
		t.Fatal("audit output must contain only twelve approved fields")
	}
	if _, err := l.Exec(context.Background(), "UPDATE auth.users SET is_bootstrap_admin=false"); err != nil {
		t.Fatal(err)
	}
	if _, err := reader.List(request, 100); err == nil {
		t.Fatal("ordinary user cannot read audit")
	}
	if _, err := l.Exec(context.Background(), "UPDATE auth.users SET is_bootstrap_admin=true,status='disabled',auth_version=auth_version+1"); err != nil {
		t.Fatal(err)
	}
	if _, err := reader.List(request, 100); err == nil {
		t.Fatal("disabled/stale Bootstrap cannot read audit")
	}
}

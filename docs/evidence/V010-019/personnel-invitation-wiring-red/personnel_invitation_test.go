package auth

import (
	"context"
	"errors"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/personnel"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"github.com/jackc/pgx/v5"
	"testing"
)

func TestInvitationUsesLivePersonnelQualificationInOwnedTransactionQ25(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	user := f.user(t, "synthetic-personnel-manager", false)
	var identity string
	if err := f.pool.QueryRow(ctx, "INSERT INTO personnel.identities(name) VALUES('invitation-manager') RETURNING id::text").Scan(&identity); err != nil {
		t.Fatal(err)
	}
	if _, err := f.pool.Exec(ctx, "INSERT INTO personnel.identity_permissions(identity_id,permission_code) VALUES($1,'personnel.manage')", identity); err != nil {
		t.Fatal(err)
	}
	if _, err := f.pool.Exec(ctx, "INSERT INTO personnel.member_identities(user_id,identity_id) VALUES($1,$2)", user, identity); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = f.pool.Exec(ctx, "DELETE FROM personnel.member_identities WHERE identity_id=$1", identity)
		_, _ = f.pool.Exec(ctx, "DELETE FROM personnel.identity_permissions WHERE identity_id=$1", identity)
		_, _ = f.pool.Exec(ctx, "DELETE FROM personnel.identities WHERE id=$1", identity)
	})
	people := &personnel.Application{Pool: f.pool}
	application := applicationFor(f)
	application.InvitationAuthorizer = func(ctx context.Context, tx pgx.Tx, p session.Principal) error {
		err := people.AuthorizeWrite(ctx, tx, p)
		if errors.Is(err, personnel.ErrDenied) {
			return &Failure{Code: "COMMON_PERMISSION_DENIED"}
		}
		return err
	}
	actor := applicationAdmin(user)
	actor.BootstrapAdmin = false
	result, err := application.CreateInvitation(ctx, actor, applicationMetadata())
	if err != nil || result.ID == "" || result.Code == "" {
		t.Fatalf("non-Root live manager must create through authentication owner: %v", err)
	}
	if _, err := application.Register(ctx, "synthetic-invited-by-manager", "Aa1!", result.Code, applicationMetadata()); err != nil {
		t.Fatal("existing one-use authentication rule must be reused")
	}
	_, err = application.Register(ctx, "synthetic-second-reuse", "Aa1!", result.Code, applicationMetadata())
	requireFailure(t, err, "INVITATION_ALREADY_USED")
	if _, err := f.pool.Exec(ctx, "DELETE FROM personnel.identity_permissions WHERE identity_id=$1", identity); err != nil {
		t.Fatal(err)
	}
	_, err = application.CreateInvitation(ctx, actor, applicationMetadata())
	requireFailure(t, err, "COMMON_PERMISSION_DENIED")
	var count int
	if err := f.pool.QueryRow(ctx, "SELECT count(*) FROM auth.invitations WHERE created_by=$1", user).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatal("revoked qualification must not create or audit an invitation")
	}
}

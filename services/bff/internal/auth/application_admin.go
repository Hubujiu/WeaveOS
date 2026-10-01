package auth

import (
	"context"
	"errors"
	"strconv"

	"github.com/Hubujiu/WeaveOS/services/bff/internal/invitation"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/persistence/authsql"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// CreateInvitation owns one authentication-domain transaction. The caller must
// authenticate its session; the transaction rechecks the current admin state.
func (a *Application) CreateInvitation(ctx context.Context, actor session.Principal, meta RequestMetadata) (InvitationResult, error) {
	code, err := invitation.Generate()
	if err != nil {
		return InvitationResult{}, err
	}
	tx, err := a.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return InvitationResult{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err := tx.Exec(ctx, "SELECT personnel.lock_query_revisions()"); err != nil {
		return InvitationResult{}, err
	}
	q := authsql.New(tx)
	if a.InvitationAuthorizer != nil {
		if err := a.InvitationAuthorizer(ctx, tx, actor); err != nil {
			return InvitationResult{}, err
		}
	} else {
		// A legacy isolated auth composition has only its trusted Bootstrap capability.
		users, err := q.LockAdminUsers(ctx, []pgtype.UUID{uuidValue(actor.UserID)})
		if err != nil {
			return InvitationResult{}, err
		}
		if len(users) != 1 || users[0].Status != "active" || !users[0].IsBootstrapAdmin || strconv.FormatInt(users[0].AuthVersion, 10) != actor.Record.AuthVersion {
			return InvitationResult{}, &Failure{Code: "COMMON_PERMISSION_DENIED"}
		}
	}
	id, err := q.CreateAdminInvitation(ctx, authsql.CreateAdminInvitationParams{CodeHash: code.Digest[:], CreatedBy: uuidValue(actor.UserID)})
	if err != nil {
		return InvitationResult{}, err
	}
	if err := q.AppendAuthEvent(ctx, eventParams(meta, "invitation_created", "success", actor.UserID, "", actor.SessionRef, "", "")); err != nil {
		a.alert()
		return InvitationResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return InvitationResult{}, err
	}
	return InvitationResult{ID: id.String(), Code: code.Value}, nil
}

func (a *Application) ResetPassword(ctx context.Context, actor session.Principal, target string, meta RequestMetadata) error {
	id := uuidValue(target)
	if !id.Valid || id.String() != target {
		return &Failure{Code: "COMMON_INVALID_ARGUMENT"}
	}
	hash, err := hashPassword("Abc@123456")
	if err != nil {
		return err
	}
	tx, err := a.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	q := authsql.New(tx)
	users, err := q.LockAdminUsers(ctx, []pgtype.UUID{uuidValue(actor.UserID), id})
	if err != nil {
		return err
	}
	actorValid, targetFound := false, false
	for _, u := range users {
		if u.ID.String() == actor.UserID {
			actorValid = u.Status == "active" && u.IsBootstrapAdmin && strconv.FormatInt(u.AuthVersion, 10) == actor.Record.AuthVersion
		}
		if u.ID.String() == target {
			targetFound = true
		}
	}
	if !actorValid {
		return &Failure{Code: "COMMON_PERMISSION_DENIED"}
	}
	if !targetFound {
		return &Failure{Code: "USER_NOT_FOUND"}
	}
	rows, err := q.UpdateResetCredential(ctx, authsql.UpdateResetCredentialParams{UserID: id, PasswordHash: hash})
	if err != nil || rows != 1 {
		return errors.New("credential mutation unavailable")
	}
	rows, err = q.BumpAuthVersion(ctx, id)
	if err != nil || rows != 1 {
		return errors.New("version mutation unavailable")
	}
	if err := q.AppendAuthEvent(ctx, eventParams(meta, "password_reset", "success", actor.UserID, target, actor.SessionRef, "", "")); err != nil {
		a.alert()
		return err
	}
	return tx.Commit(ctx)
}

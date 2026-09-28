package auth

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/netip"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/Hubujiu/WeaveOS/services/bff/internal/persistence/authsql"
	"github.com/jackc/pgx/v5/pgtype"
)

func uuid() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	b[6] = (b[6] & 15) | 64
	b[8] = (b[8] & 63) | 128
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[:4], b[4:6], b[6:8], b[8:10], b[10:]), nil
}

func uuidValue(id string) pgtype.UUID {
	var v pgtype.UUID
	if id != "" {
		_ = v.Scan(id)
	}
	return v
}

func textValue(s string) pgtype.Text { return pgtype.Text{String: s, Valid: s != ""} }

func (a *Application) accountFingerprint(account string) string {
	mac := hmac.New(sha256.New, a.AuditKey)
	_, _ = mac.Write([]byte(strings.Trim(account, " ")))
	return a.AuditKeyID + ":" + hex.EncodeToString(mac.Sum(nil))
}

func (a *Application) alert() {
	if a.Logger != nil {
		a.Logger.Error("authentication audit persistence failed")
	}
}

func (a *Application) event(ctx context.Context, meta RequestMetadata, kind, outcome, actor, subject, ref, reason, fingerprint string) error {
	return authsql.New(a.Pool).AppendAuthEvent(ctx, eventParams(meta, kind, outcome, actor, subject, ref, reason, fingerprint))
}

func eventParams(meta RequestMetadata, kind, outcome, actor, subject, ref, reason, fingerprint string) authsql.AppendAuthEventParams {
	var ip *netip.Addr
	if addr, err := netip.ParseAddr(meta.ClientIP); err == nil {
		ip = &addr
	}
	ua := strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, meta.UserAgent)
	if utf8.RuneCountInString(ua) > 2048 {
		ua = string([]rune(ua)[:2048])
	}
	return authsql.AppendAuthEventParams{EventType: kind, Outcome: outcome, ActorUserID: uuidValue(actor), SubjectUserID: uuidValue(subject), AccountFingerprint: textValue(fingerprint), ClientIp: ip, UserAgent: textValue(ua), SessionRef: uuidValue(ref), ReasonCode: textValue(reason), RequestID: meta.RequestID}
}

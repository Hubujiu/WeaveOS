package identity

import (
	"context"
	"errors"
	"regexp"
)

// Context contains only the stable subject and non-credential session reference.
type Context struct {
	SubjectID string
	SessionID string
}

type contextKey struct{}

var canonicalUUID = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

var ErrInvalidIdentity = errors.New("invalid trusted identity")

func WithTrusted(ctx context.Context, value Context) (context.Context, error) {
	if ctx == nil || !canonicalUUID.MatchString(value.SubjectID) || !canonicalUUID.MatchString(value.SessionID) {
		return nil, ErrInvalidIdentity
	}
	return context.WithValue(ctx, contextKey{}, value), nil
}

func From(ctx context.Context) (Context, bool) {
	if ctx == nil {
		return Context{}, false
	}
	value, ok := ctx.Value(contextKey{}).(Context)
	return value, ok
}

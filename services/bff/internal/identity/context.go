package identity

import (
	"context"
	"errors"
)

// Context contains only the stable subject and non-credential session reference.
type Context struct {
	SubjectID string
	SessionID string
}

func WithTrusted(ctx context.Context, value Context) (context.Context, error) {
	return ctx, errors.New("identity context not implemented")
}

func From(context.Context) (Context, bool) {
	return Context{}, false
}

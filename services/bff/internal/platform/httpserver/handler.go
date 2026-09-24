package httpserver

import (
	"context"
	"net/http"
)

type ReadyCheck func(context.Context) error

// NewHandler is an interface-only RED scaffold.
func NewHandler(_ ReadyCheck) http.Handler { return http.NotFoundHandler() }

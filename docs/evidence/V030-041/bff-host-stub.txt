package main

import (
	"context"
	"net/http"
)

// Declaration only until Root observes actual host-composition RED in isolated CI.
type bffHost struct{ Handler http.Handler }

func buildHost(context.Context, context.Context, config) (*bffHost, error) { return nil, nil }
func (*bffHost) Quiesce()                                                  {}
func (*bffHost) Close() error                                              { return nil }
func (*bffHost) WorkersDone() <-chan struct{}                              { return nil }
func (*bffHost) Err() error                                                { return nil }

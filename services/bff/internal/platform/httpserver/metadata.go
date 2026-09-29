package httpserver

import "context"

type RequestMetadata struct{ RequestID, ClientIP, UserAgent string }

// Minimal declaration for FR-02 RED; context extraction is not implemented yet.
func Metadata(context.Context) RequestMetadata { return RequestMetadata{} }

package security

import "net/http"

func ValidSource(r *http.Request, trustedOrigin string) bool { return false }
func ValidCSRF(r *http.Request, digest string) bool { return false }

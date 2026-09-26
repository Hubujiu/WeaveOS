package security

import (
	"crypto/subtle"
	"net/http"
	"net/url"
)

func ValidSource(r *http.Request, trustedOrigin string) bool {
	trusted, err := url.Parse(trustedOrigin)
	if err != nil || trusted.Scheme != "https" || trusted.Host == "" || trusted.User != nil || trusted.RawQuery != "" || trusted.Fragment != "" || (trusted.Path != "" && trusted.Path != "/") {
		return false
	}
	if origins, ok := r.Header["Origin"]; ok {
		return len(origins) == 1 && origins[0] == trusted.Scheme+"://"+trusted.Host
	}
	refs := r.Header.Values("Referer")
	if len(refs) != 1 {
		return false
	}
	ref, err := url.Parse(refs[0])
	return err == nil && ref.User == nil && ref.Scheme == trusted.Scheme && ref.Host == trusted.Host
}
func ValidCSRF(r *http.Request, digest string) bool {
	headers := r.Header.Values("X-CSRF-Token")
	if len(headers) != 1 {
		return false
	}
	var token string
	count := 0
	for _, c := range r.Cookies() {
		if c.Name == "__Host-csrf" {
			token = c.Value
			count++
		}
	}
	if count != 1 || !MatchesCSRFToken(token, digest) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(token), []byte(headers[0])) == 1
}

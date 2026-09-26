package session

import (
	"encoding/base64"
	"net/http"
	"time"
)

const SessionCookieName = "__Host-session"
const CSRFCookieName = "__Host-csrf"

func IssueCookies(w http.ResponseWriter, sid, csrf string) error {
	for _, value := range []string{sid, csrf} {
		raw, err := base64.RawURLEncoding.DecodeString(value)
		if err != nil || len(raw) != 32 || base64.RawURLEncoding.EncodeToString(raw) != value {
			return ErrInvalid
		}
	}
	for _, c := range []*http.Cookie{cookie(SessionCookieName, sid, true, 3600), cookie(CSRFCookieName, csrf, false, 3600)} {
		http.SetCookie(w, c)
	}
	return nil
}
func ClearCookies(w http.ResponseWriter) {
	for _, c := range []*http.Cookie{cookie(SessionCookieName, "", true, -1), cookie(CSRFCookieName, "", false, -1)} {
		c.Expires = time.Unix(1, 0).UTC()
		http.SetCookie(w, c)
	}
}
func cookie(name, value string, httpOnly bool, maxAge int) *http.Cookie {
	return &http.Cookie{Name: name, Value: value, Path: "/", Secure: true, HttpOnly: httpOnly, SameSite: http.SameSiteLaxMode, MaxAge: maxAge}
}

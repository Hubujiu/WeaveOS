package session

import "net/http"

func IssueCookies(w http.ResponseWriter, sid, csrf string) error { return ErrInvalid }
func ClearCookies(w http.ResponseWriter) {}

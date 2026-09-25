package security

import "errors"

func NewCSRFToken() (string, string, error) {
	return "", "", errors.New("CSRF token generation not implemented")
}

func MatchesCSRFToken(string, string) bool {
	return false
}

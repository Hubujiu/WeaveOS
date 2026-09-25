package security

import "strconv"

func CurrentUserMatchesSession(snapshot, status string, currentVersion int64) bool {
	return status == "active" && currentVersion > 0 && snapshot == strconv.FormatInt(currentVersion, 10)
}

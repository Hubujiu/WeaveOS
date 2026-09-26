package auth

func validPassword(string) bool           { return false }
func hashPassword(string) (string, error) { return "", nil }
func verifyPassword(string, string) bool  { return false }

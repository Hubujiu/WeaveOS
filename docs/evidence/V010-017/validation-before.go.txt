package auth

import (
	"net/http"
	"strings"
)

type violation struct {
	Location string `json:"location"`
	Field    string `json:"field"`
	Code     string `json:"code"`
	Message  string `json:"message"`
}

func validateCredentials(w http.ResponseWriter, r *http.Request, account, password string, invitation *string) bool {
	var violations []violation
	add := func(field, code, message string) {
		violations = append(violations, violation{"body", field, code, message})
	}
	account = strings.Trim(account, " ")
	if account == "" {
		add("account", "VALIDATION_REQUIRED", "请填写账号")
	} else if !accountValid(account) {
		add("account", "COMMON_INVALID_ARGUMENT", "账号格式不符合要求")
	}
	if password == "" {
		add("password", "VALIDATION_REQUIRED", "请填写密码")
	} else if invitation != nil && !validPassword(password) {
		add("password", "AUTH_PASSWORD_POLICY_VIOLATION", "密码不符合当前密码规则")
	}
	if invitation != nil && *invitation == "" {
		add("invitationCode", "VALIDATION_REQUIRED", "请填写邀请码")
	}
	if len(violations) == 0 {
		return true
	}
	reply(w, r, 400, "COMMON_VALIDATION_FAILED", struct {
		Violations []violation `json:"violations"`
	}{violations})
	return false
}

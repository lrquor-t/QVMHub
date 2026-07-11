package service

import "qvmhub/service/security"

// CheckLoginAllowed 代理到 security 子包
func CheckLoginAllowed(ip, username string) (bool, string) {
	return security.CheckLoginAllowed(ip, username)
}

// RecordLoginFailure 代理到 security 子包
func RecordLoginFailure(ip, username string) {
	security.RecordLoginFailure(ip, username)
}

// ClearLoginFailures 代理到 security 子包
func ClearLoginFailures(ip, username string) {
	security.ClearLoginFailures(ip, username)
}

package service

// JWT secret function delegates - forward to service/security subpackage
// Maintains backward compatibility for callers using service.XXX()
import (
	securitypkg "qvmhub/service/security"
)

// GenerateRandomSecret delegates to security.GenerateRandomSecret
func GenerateRandomSecret() (string, error) {
	return securitypkg.GenerateRandomSecret()
}

// RotateJWTSecret delegates to security.RotateJWTSecret
func RotateJWTSecret() (string, error) {
	return securitypkg.RotateJWTSecret()
}

// StartJWTSecretRotator delegates to security.StartJWTSecretRotator
func StartJWTSecretRotator() {
	securitypkg.StartJWTSecretRotator()
}

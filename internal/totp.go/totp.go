package totp

// TOTP defines the interface for TOTP operations
type TOTP interface {
	GenerateSecret(issuer, accountName string, size int) (string, string, error)
	GenerateCode(secret string) (string, error)
	VerifyCode(secret, code string) bool
}

package totp

import "image"

// TOTP defines the interface for TOTP operations
type TOTP interface {
	GenerateSecret(issuer, accountName string) (string, error)
	GenerateQRCode(secret, issuer, accountName string) (image.Image, error)
	GenerateCode(secret string) (string, error)
	VerifyCode(secret, code string) bool
}

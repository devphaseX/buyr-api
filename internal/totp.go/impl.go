package totp

import (
	"encoding/base64"
	"fmt"
	"time"

	"github.com/pquerna/otp/totp"
	"github.com/skip2/go-qrcode"
)

// totpImpl implements the TOTP interface
type totpImpl struct{}

// New creates a new instance of the TOTP implementation
func New() TOTP {
	return &totpImpl{}
}

// GenerateSecret generates a new TOTP secret
func (t *totpImpl) GenerateSecret(issuer, accountName string, size int) (string, string, error) {
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      issuer,
		AccountName: accountName,
	})
	if err != nil {
		return "", "", err
	}

	// Get the provisioning URI
	uri := key.URL()

	// Generate QR code as PNG bytes
	qrBytes, err := qrcode.Encode(uri, qrcode.Medium, size)
	if err != nil {
		return "", "", err
	}

	// Encode to base64
	return key.Secret(), fmt.Sprintf("data:image/png;base64,%s", base64.StdEncoding.EncodeToString(qrBytes)), nil
}

// GenerateCode generates a TOTP code for the given secret
func (t *totpImpl) GenerateCode(secret string) (string, error) {
	code, err := totp.GenerateCode(secret, time.Now())
	if err != nil {
		return "", err
	}
	return code, nil
}

// VerifyCode verifies a TOTP code against the given secret
func (t *totpImpl) VerifyCode(secret, code string) bool {
	return totp.Validate(code, secret)
}

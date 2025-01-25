package totp

import (
	"image"
	"time"

	"github.com/pquerna/otp/totp"
)

// totpImpl implements the TOTP interface
type totpImpl struct{}

// New creates a new instance of the TOTP implementation
func New() TOTP {
	return &totpImpl{}
}

// GenerateSecret generates a new TOTP secret
func (t *totpImpl) GenerateSecret(issuer, accountName string) (string, error) {
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      issuer,
		AccountName: accountName,
	})
	if err != nil {
		return "", err
	}
	return key.Secret(), nil
}

// GenerateQRCode generates a QR code image for the TOTP secret
func (t *totpImpl) GenerateQRCode(secret, issuer, accountName string) (image.Image, error) {
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      issuer,
		AccountName: accountName,
		Secret:      []byte(secret),
	})
	if err != nil {
		return nil, err
	}

	// Generate the QR code image
	img, err := key.Image(200, 200) // 200x200 pixels
	if err != nil {
		return nil, err
	}

	return img, nil
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

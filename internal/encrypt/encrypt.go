package encrypt

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"

	"golang.org/x/crypto/scrypt"
)

// EncryptSecret encrypts the two-factor authentication secret
func EncryptSecret(secret, passphrase string) (string, error) {
	// Derive a 32-byte key from the passphrase using scrypt
	salt := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return "", err
	}

	key, err := scrypt.Key([]byte(passphrase), salt, 32768, 8, 1, 32)
	if err != nil {
		return "", err
	}

	// Create AES cipher block
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	// Create GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	// Create a nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	// Encrypt the secret
	encrypted := gcm.Seal(nonce, nonce, []byte(secret), nil)

	// Combine salt and encrypted data
	encryptedData := append(salt, encrypted...)

	// Encode to base64
	return base64.StdEncoding.EncodeToString(encryptedData), nil
}

// DecryptSecret decrypts the two-factor authentication secret
func DecryptSecret(encryptedSecret, passphrase string) (string, error) {
	// Decode from base64
	encryptedData, err := base64.StdEncoding.DecodeString(encryptedSecret)
	if err != nil {
		return "", err
	}

	// Extract salt (first 16 bytes)
	salt := encryptedData[:16]
	ciphertext := encryptedData[16:]

	// Derive key using same scrypt parameters
	key, err := scrypt.Key([]byte(passphrase), salt, 32768, 8, 1, 32)
	if err != nil {
		return "", err
	}

	// Create AES cipher block
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	// Create GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	// Extract nonce
	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return "", errors.New("invalid ciphertext")
	}
	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]

	// Decrypt
	decrypted, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}

	return string(decrypted), nil
}

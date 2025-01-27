package encrypt

import (
	"crypto/rand"
	"math/big"
)

const uppercaseAlphanumeric = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

// GenerateRecoveryCodes generates a set of recovery codes.
func GenerateRecoveryCodes(count int, length int) ([]string, error) {
	codes := make([]string, count)
	for i := 0; i < count; i++ {
		code, err := GenerateRandomString(length)
		if err != nil {
			return nil, err
		}
		codes[i] = code
	}
	return codes, nil
}

// generateRandomString generates a random uppercase alphanumeric string of the specified length.
func GenerateRandomString(length int) (string, error) {
	result := make([]byte, length)
	for i := 0; i < length; i++ {
		// Generate a random index into the uppercaseAlphanumeric string
		randomIndex, err := rand.Int(rand.Reader, big.NewInt(int64(len(uppercaseAlphanumeric))))
		if err != nil {
			return "", err
		}
		result[i] = uppercaseAlphanumeric[randomIndex.Int64()]
	}
	return string(result), nil
}

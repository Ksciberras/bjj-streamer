package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
)

func newToken() (string, []byte, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", nil, err
	}
	value := base64.RawURLEncoding.EncodeToString(raw)
	hash := sha256.Sum256([]byte(value))
	return value, hash[:], nil
}

func tokenHash(value string) []byte {
	hash := sha256.Sum256([]byte(value))
	return hash[:]
}

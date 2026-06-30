package util

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"strings"
)

const (
	APITokenPlainPrefix = "pmt_"
	apiTokenPrefixSize  = 12
)

func GenerateAPIToken() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return APITokenPlainPrefix + base64.RawURLEncoding.EncodeToString(raw), nil
}

func HashAPIToken(token string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(token)))
	return hex.EncodeToString(sum[:])
}

func APITokenLookupPrefix(token string) string {
	trimmed := strings.TrimSpace(token)
	if len(trimmed) <= apiTokenPrefixSize {
		return trimmed
	}
	return trimmed[:apiTokenPrefixSize]
}

func APITokenLastFour(token string) string {
	trimmed := strings.TrimSpace(token)
	if len(trimmed) <= 4 {
		return trimmed
	}
	return trimmed[len(trimmed)-4:]
}

func IsAPIToken(token string) bool {
	return strings.HasPrefix(strings.TrimSpace(token), APITokenPlainPrefix)
}

func EqualAPITokenHash(storedHash, token string) bool {
	nextHash := HashAPIToken(token)
	return subtle.ConstantTimeCompare([]byte(storedHash), []byte(nextHash)) == 1
}

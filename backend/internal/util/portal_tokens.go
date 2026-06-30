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
	PortalTokenPlainPrefix = "pmp_"
	portalTokenPrefixSize  = 12
)

func GeneratePortalToken() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return PortalTokenPlainPrefix + base64.RawURLEncoding.EncodeToString(raw), nil
}

func HashPortalToken(token string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(token)))
	return hex.EncodeToString(sum[:])
}

func PortalTokenLookupPrefix(token string) string {
	trimmed := strings.TrimSpace(token)
	if len(trimmed) <= portalTokenPrefixSize {
		return trimmed
	}
	return trimmed[:portalTokenPrefixSize]
}

func PortalTokenLastFour(token string) string {
	trimmed := strings.TrimSpace(token)
	if len(trimmed) <= 4 {
		return trimmed
	}
	return trimmed[len(trimmed)-4:]
}

func EqualPortalTokenHash(storedHash, token string) bool {
	nextHash := HashPortalToken(token)
	return subtle.ConstantTimeCompare([]byte(storedHash), []byte(nextHash)) == 1
}

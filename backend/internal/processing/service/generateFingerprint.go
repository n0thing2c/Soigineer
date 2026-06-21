package service

import (
	"crypto/sha256"
	"encoding/hex"
)

func GenerateFingerprint(appName string, level string, category string, normalizedMsg string) string {
	input := appName + "|" + level + "|" + category + "|" + normalizedMsg
	fingerprint := sha256.Sum256([]byte(input))
	return hex.EncodeToString(fingerprint[:])
}

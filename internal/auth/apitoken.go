package auth

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

func GenerateAPIToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate api token: %w", err)
	}
	return hex.EncodeToString(b), nil
}

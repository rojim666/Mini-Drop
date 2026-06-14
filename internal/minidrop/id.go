package minidrop

import (
	"crypto/rand"
	"encoding/hex"
)

func GenerateID(prefix string) string {
	buf := make([]byte, 6)
	if _, err := rand.Read(buf); err != nil {
		return prefix + "_fallback"
	}

	return prefix + "_" + hex.EncodeToString(buf)
}

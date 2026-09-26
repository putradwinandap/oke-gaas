package identity

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

// New returns a prefixed, cryptographically random identifier suitable for domain entities.
func New(prefix string) (string, error) {
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "", fmt.Errorf("generate %s id: %w", prefix, err)
	}

	return prefix + "_" + hex.EncodeToString(bytes[:]), nil
}

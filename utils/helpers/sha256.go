package helpers

import (
	"crypto/sha256"
	"encoding/hex"
)

// Hash the given bytes using SHA256.
func Hash(data []byte) string {
	hasher := sha256.New()
	hasher.Write(data)
	return hex.EncodeToString(hasher.Sum(nil))
}

// ValidateHash validate whether the given password is match with the given hash.
func ValidateHash(password, hash string) bool {
	hashed := Hash([]byte(password))
	return string(hashed) == hash
}

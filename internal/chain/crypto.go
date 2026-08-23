package chain

import (
	"crypto/sha256"
	"encoding/hex"
)

const ZeroHash = "0000000000000000000000000000000000000000000000000000000000000000"

func Digest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func ValidHash(value string) bool {
	if len(value) != sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

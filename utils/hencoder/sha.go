package hencoder

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

func Sha256(data []byte) []byte {
	h := sha256.New()
	h.Write(data)
	return h.Sum(nil)
}

func Sha256ToString(data []byte) string {
	return hex.EncodeToString(Sha256(data))
}

func Sha256Hash(v string) string {
	h := sha256.New()
	h.Write([]byte(v))
	return fmt.Sprintf("%x", h.Sum(nil))
}

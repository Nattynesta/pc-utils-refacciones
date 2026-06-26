package main

import (
	"crypto/sha256"
	"encoding/hex"
)

func hashPassword(password string) string {
	h := sha256.Sum256([]byte(password))
	return hex.EncodeToString(h[:])
}

func verifyPassword(password, hash string) bool {
	return hashPassword(password) == hash
}

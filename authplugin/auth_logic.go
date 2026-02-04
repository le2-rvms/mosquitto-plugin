package main

import (
	"crypto/sha256"
	"encoding/hex"
)

// sha256PwdSalt 使用盐对密码做 SHA-256。
func sha256PwdSalt(pwd, salt string) string {
	sum := sha256.Sum256([]byte(pwd + salt))
	return hex.EncodeToString(sum[:])
}

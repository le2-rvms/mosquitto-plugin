package main

import (
	"crypto/sha256"
	"encoding/hex"
)

// sha256PwdSalt 使用盐对密码做 SHA-256，并返回十六进制字符串。
func sha256PwdSalt(password, salt string) string {
	sum := sha256.Sum256([]byte(password + salt))
	return hex.EncodeToString(sum[:])
}

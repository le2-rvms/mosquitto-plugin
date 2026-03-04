package main

import (
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
)

var (
	salt     = flag.String("salt", "", "salt")
	password = flag.String("password", "", "password")
)

func main() {
	flag.Parse()

	if *password == "" {
		flag.Usage()
		os.Exit(2)
	}

	// 与认证插件保持一致：password+salt 后做 SHA-256 十六进制输出。
	sum := sha256.Sum256([]byte(*password + *salt))
	fmt.Println(hex.EncodeToString(sum[:]))
}

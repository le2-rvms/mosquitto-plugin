package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"strings"
)

func main() {
	salt := flag.String("salt", "", "salt")
	flag.Parse()

	var pwd string
	if flag.NArg() > 0 {
		pwd = flag.Arg(0)
	} else {
		fmt.Print("Password: ")
		s, _ := bufio.NewReader(os.Stdin).ReadString('\n')
		pwd = strings.TrimRight(s, "\r\n")
	}

	en_pwd := sha256PwdSalt(pwd, *salt)
	fmt.Printf(en_pwd)
}

func sha256PwdSalt(pwd, salt string) string {
	sum := sha256.Sum256([]byte(pwd + salt))
	return hex.EncodeToString(sum[:])
}

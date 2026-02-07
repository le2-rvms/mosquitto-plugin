package main

import (
	"crypto/sha256"
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

	fmt.Printf("%x\n", sha256.Sum256([]byte(*password+*salt)))
}

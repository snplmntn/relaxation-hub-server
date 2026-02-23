package main

import (
	"fmt"
	"os"

	"golang.org/x/crypto/bcrypt"
)

func main() {
	hash, err := bcrypt.GenerateFromPassword([]byte("Sean1234!"), 10)
	if err != nil {
		fmt.Printf("FAILED: %v\n", err)
	} else {
		err = os.WriteFile("generated_hash.txt", hash, 0644)
		if err != nil {
			fmt.Printf("FAILED TO WRITE: %v\n", err)
		} else {
			fmt.Println("Hash successfully generated and written to generated_hash.txt")
		}
	}
}

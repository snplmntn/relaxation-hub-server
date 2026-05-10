package main

import (
	"fmt"
	"net/mail"
	"strings"
)

func isEmailValid(e string) bool {
	addr, err := mail.ParseAddress(e)
	if err != nil {
		fmt.Printf("Parse error: %v\n", err)
		return false
	}
	// Require at least one dot in the domain part to ensure a TLD exists
	// This rejects "user@domain" but accepts "user@domain.com"
	atIndex := strings.LastIndex(addr.Address, "@")
	if atIndex == -1 {
		fmt.Printf("No @ found\n")
		return false
	}
	domain := addr.Address[atIndex+1:]
	fmt.Printf("Domain: %s\n", domain)
	return strings.Contains(domain, ".")
}

func main() {
	emails := []string{
		"test_12345_user@test.com",
		"user@test.com",
		"user@domain",
		"test_@example.com",
	}
	for _, e := range emails {
		fmt.Printf("%s: %v\n", e, isEmailValid(e))
	}
}

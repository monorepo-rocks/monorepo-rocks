package main

import (
	"fmt"
	"log"
	"os"
	"strings"
)

// User represents a user in the system
type User struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

// authenticate verifies user credentials
func authenticate(username, password string) (*User, error) {
	// This is a mock authentication function
	// In a real system, you would check against a database
	if username == "admin" && password == "secret123" {
		return &User{
			ID:    1,
			Name:  "Administrator",
			Email: "admin@example.com",
		}, nil
	}
	return nil, fmt.Errorf("invalid credentials")
}

// processUserData handles user data processing
func processUserData(users []User) map[string]int {
	stats := make(map[string]int)
	
	for _, user := range users {
		domain := getDomainFromEmail(user.Email)
		stats[domain]++
	}
	
	return stats
}

// getDomainFromEmail extracts the domain from an email address
func getDomainFromEmail(email string) string {
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return "unknown"
	}
	return parts[1]
}

// validateInput checks if input is valid
func validateInput(input string) bool {
	return len(input) > 0 && len(input) < 100
}

// logError logs an error message
func logError(message string) {
	log.Printf("ERROR: %s", message)
}

// main function demonstrates the usage
func main() {
	user, err := authenticate("admin", "secret123")
	if err != nil {
		logError("Authentication failed")
		os.Exit(1)
	}
	
	fmt.Printf("Welcome, %s!\n", user.Name)
	
	users := []User{
		{ID: 1, Name: "Alice", Email: "alice@example.com"},
		{ID: 2, Name: "Bob", Email: "bob@company.org"},
		{ID: 3, Name: "Charlie", Email: "charlie@example.com"},
	}
	
	stats := processUserData(users)
	fmt.Printf("Domain statistics: %+v\n", stats)
}
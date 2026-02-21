package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://postgres:qzdyqtdvcpmomujwionp:dxaTC3aZqTV4Elfy@aws-1-ap-southeast-1.pooler.supabase.com:5432/postgres"
	}
	pool, err := pgxpool.New(context.Background(), dbURL)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()

	var actorID int
	var fullName, email, role string
	err = pool.QueryRow(context.Background(), "SELECT user_id, COALESCE(full_name, ''), COALESCE(primary_email, ''), COALESCE(role, '') FROM users WHERE user_id = 4118").Scan(&actorID, &fullName, &email, &role)
	if err != nil {
		fmt.Println("Error querying user 4118:", err)
	} else {
		fmt.Printf("User 4118: Name:%q Email:%q Role:%q\n", fullName, email, role)
	}
}

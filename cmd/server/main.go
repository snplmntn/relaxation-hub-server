package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/snplmntn/relaxation-hub-server/internal/config"
	"github.com/snplmntn/relaxation-hub-server/internal/db"
	"github.com/snplmntn/relaxation-hub-server/internal/handler"
	"github.com/snplmntn/relaxation-hub-server/internal/middleware"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
	"github.com/snplmntn/relaxation-hub-server/internal/service"
)


func main() {
	config, err := config.LoadConfig()
	if err != nil {
		log.Fatal("Error loading .env file: ", err)
	}
	pool, err := db.InitDB(config.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v\n", err)
	}
	defer db.CloseDB(pool)

	// Wire dependencies
	userRepo := repository.NewUserRepository(pool)
	authService := service.NewAuthService(userRepo, config)
	authHandler := &handler.AuthHandler{AuthService: authService}
	
	r := chi.NewRouter()

	r.Use(chiMiddleware.Logger)

	r.Route("/api/v1", func(r chi.Router) {
		r.Post("/register", authHandler.HandleSignup)
		r.Post("/login", authHandler.HandleLogin) 

		// Apply auth middleware to all subsequent routes in this group
		r.Group(func(r chi.Router) {
			r.Use(func(next http.Handler) http.Handler {
				return middleware.AuthMiddleware(next, config.JWTKey)
			})

			// Example: Protect admin routes with role middleware
			r.Route("/admin", func(r chi.Router) {
				// r.Use(func(next http.Handler) http.Handler {
				// 	return middleware.RoleMiddleware([]string{"admin"}, next)
				// })
				r.Get("/", func(w http.ResponseWriter, r *http.Request) {
					w.Write([]byte("Hello authenticated user"))
				})
				// Add admin routes here, e.g., r.Get("/users", adminHandler.ListUsers)
			})
		})

		// Other protected routes can go here
	})

	fmt.Printf("Starting Relaxation Hub Server on port: %s...\n", config.Port)
	if err := http.ListenAndServe(":"+config.Port, r); err != nil {
		fmt.Printf("Error starting server: %s\n", err)
	}
}
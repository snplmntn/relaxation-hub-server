package middleware

import (
	"context"
	"net/http"
	"slices"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
)

type contextKey string
const (
	userIDKey contextKey = "user_id"
	roleKey   contextKey = "role"
)

func AuthMiddleware(next http.Handler, jwtSecretKey string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "Authorization Header Required", http.StatusUnauthorized)
			return
		}

		headerParts := strings.Split(authHeader, " ")
		if len(headerParts) != 2 || headerParts[0] != "Bearer" {
			http.Error(w, "Invalid Authorization header format", http.StatusUnauthorized)
			return
		}

		tokenString := headerParts[1]
		
		claims := &model.Claims{}
		token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (any, error) {
			return []byte(jwtSecretKey), nil
		})

		if err != nil || !token.Valid {
			http.Error(w, "Invalid token", http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), userIDKey, claims.UserID)
		ctx = context.WithValue(ctx, roleKey, claims.Role)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func RoleMiddleware (allowedRoles []string, next http.Handler) http.Handler {
	return http.HandlerFunc(func (w http.ResponseWriter, r *http.Request) {
		role := r.Context().Value(roleKey).(string)

		if len(allowedRoles) == 0 {
			http.Error(w, "No allowed roles permitted.", http.StatusUnauthorized)
			return
		}

		if role == "" {
			http.Error(w, "Role unidentified.", http.StatusUnauthorized)
			return
		}

		if !slices.Contains(allowedRoles, role) {
			http.Error(w, "Route unautorized.", http.StatusUnauthorized)
		}

		next.ServeHTTP(w, r)
	})
}
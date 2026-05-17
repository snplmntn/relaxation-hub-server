package middleware

import (
	"context"
	"net/http"
	"slices"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/response"
)

type contextKey string

const (
	userIDKey contextKey = "user_id"
	roleKey   contextKey = "role"
)

var (
	AdminOperationalRoles = []string{model.RoleAdmin, model.RoleSuperAdmin}
	SuperAdminOnlyRoles   = []string{model.RoleSuperAdmin}
)

// parseToken extracts and validates a JWT from the Authorization header.
// Returns claims if valid, nil if missing/invalid.
func parseToken(r *http.Request, jwtSecretKey string) *model.Claims {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return nil
	}

	headerParts := strings.Fields(authHeader)
	if len(headerParts) != 2 || !strings.EqualFold(headerParts[0], "Bearer") {
		return nil
	}

	tokenString := headerParts[1]
	claims := &model.Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return []byte(jwtSecretKey), nil
	})

	if err != nil || !token.Valid {
		return nil
	}
	return claims
}

func AuthMiddleware(next http.Handler, jwtSecretKey string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims := parseToken(r, jwtSecretKey)
		if claims == nil {
			response.RespondError(w, http.StatusUnauthorized, "Invalid or missing token")
			return
		}

		ctx := context.WithValue(r.Context(), userIDKey, claims.UserID)
		ctx = context.WithValue(ctx, roleKey, claims.Role)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// OptionalAuthMiddleware validates the token if present, but allows the request to proceed anonymously if missing or invalid.
func OptionalAuthMiddleware(next http.Handler, jwtSecretKey string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims := parseToken(r, jwtSecretKey)
		if claims == nil {
			// No valid token, proceed anonymously
			next.ServeHTTP(w, r)
			return
		}

		// Valid token, set context
		ctx := context.WithValue(r.Context(), userIDKey, claims.UserID)
		ctx = context.WithValue(ctx, roleKey, claims.Role)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func RoleMiddleware(allowedRoles []string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		roleVal := r.Context().Value(roleKey)
		if roleVal == nil {
			response.RespondError(w, http.StatusForbidden, "Role unidentified.")
			return
		}
		role := roleVal.(string)

		if len(allowedRoles) == 0 {
			response.RespondError(w, http.StatusForbidden, "No allowed roles permitted.")
			return
		}

		if role == "" {
			response.RespondError(w, http.StatusForbidden, "Role unidentified.")
			return
		}

		if !slices.Contains(allowedRoles, role) {
			response.RespondError(w, http.StatusForbidden, "Route unauthorized.")
			return
		}

		next.ServeHTTP(w, r)
	})
}

// GetUserID extracts the authenticated user ID from the request context.
func GetUserID(r *http.Request) (int64, bool) {
	val := r.Context().Value(userIDKey)
	switch v := val.(type) {
	case int:
		return int64(v), true
	case int64:
		return v, true
	default:
		return 0, false
	}
}

// GetUserRole extracts the authenticated user role from the request context.
func GetUserRole(r *http.Request) (string, bool) {
	val := r.Context().Value(roleKey)
	role, ok := val.(string)
	return role, ok
}

// SetUserRole sets the user role in context (for testing)
func SetUserRole(ctx context.Context, role string) context.Context {
	return context.WithValue(ctx, roleKey, role)
}

// claimsKey is the context key for storing full claims
const claimsKey contextKey = "claims"

// GetClaims extracts the full JWT claims from the request context.
// Returns nil if no claims are present (unauthenticated request).
func GetClaims(r *http.Request) *model.Claims {
	userID, ok := GetUserID(r)
	if !ok {
		return nil
	}
	role, _ := GetUserRole(r)
	return &model.Claims{
		UserID: int(userID),
		Role:   role,
	}
}

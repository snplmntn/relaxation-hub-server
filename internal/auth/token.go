package auth

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
)

func GenerateToken(userID int, role string, jwtKeyString string) (string, error) {
	if jwtKeyString == "" {
		return "", fmt.Errorf("JWT_KEY environment variable not found")
	}
	// 4. Generate the JWT
	expirationTime := time.Now().Add(24 * time.Hour) // Token is valid for 1 day
	claims := &model.Claims{
		UserID: userID,
		Role: role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(jwtKeyString))
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %w", err)
	}

	return tokenString, nil
}

func ValidateToken(tokenString string, jwtKeyString string) (model.Claims, error) {
	if jwtKeyString == "" {
		return model.Claims{}, fmt.Errorf("JWT_KEY environment variable not found")
	}
	
	claims := &model.Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (any, error) {
		return []byte(jwtKeyString), nil
	})

	if err != nil || !token.Valid {
		return model.Claims{}, fmt.Errorf("invalid token")
	}
	return *claims, nil
}
package api

import (
	"time"

	"github.com/golang-jwt/jwt/v4"
)

// JWT secret for signing and verifying tokens
var jwtSecret = []byte("supersecretkey")

type CustomClaims struct {
	SubID string `json:"user_id"`
	jwt.RegisteredClaims
}

func generateJWT(subId string) (string, error) {
	claims := CustomClaims{
		SubID: subId,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour * 24)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSecret)
}

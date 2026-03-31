package service

import (
	"backend/internal/config"
	"fmt"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type AuthServiceImpl struct {
	secret     []byte
	expiration time.Duration
}

func NewAuthService(cfg *config.Config) *AuthServiceImpl {
	return &AuthServiceImpl{
		secret:     cfg.JWTSecret,
		expiration: cfg.JWTExpiration,
	}
}

func (s *AuthServiceImpl) GenerateToken(username string) (string, error) {
	// Debug override for local testing without full auth service
	if debugID := os.Getenv("DEBUG_USER_ID"); debugID != "" {
		username = debugID
	}

	claims := jwt.MapClaims{
		"username": username,
		"userId":   username, // Explicitly provide both for compatibility
		"exp":      time.Now().Add(s.expiration).Unix(),
		"iat":      time.Now().Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS512, claims)

	return token.SignedString(s.secret)
}

func (s *AuthServiceImpl) ValidateToken(tokenStr string) (map[string]interface{}, error) {
	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return s.secret, nil
	})
	if err != nil {
		return nil, err
	}
	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		return claims, nil
	}
	return nil, fmt.Errorf("invalid token")
}

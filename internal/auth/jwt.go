package auth

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type JWTConfig struct {
	Secret           string        `mapstructure:"secret"`
	ExpiresIn        time.Duration `mapstructure:"expires_in"`
	RefreshExpiresIn time.Duration `mapstructure:"refresh_expires_in"`
}

type Claims struct {
	UserID   string `json:"sub"`
	Username string `json:"username"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

type JWTService struct {
	secret           []byte
	expiresIn        time.Duration
	refreshExpiresIn time.Duration
}

func NewJWTService(cfg *JWTConfig) *JWTService {
	if cfg.ExpiresIn == 0 {
		cfg.ExpiresIn = time.Hour
	}
	if cfg.RefreshExpiresIn == 0 {
		cfg.RefreshExpiresIn = 7 * 24 * time.Hour
	}
	return &JWTService{
		secret:           []byte(cfg.Secret),
		expiresIn:        cfg.ExpiresIn,
		refreshExpiresIn: cfg.RefreshExpiresIn,
	}
}

func (s *JWTService) GenerateAccessToken(userID, username, role string) (string, time.Time, error) {
	expiresAt := time.Now().Add(s.expiresIn)
	claims := Claims{
		UserID:   userID,
		Username: username,
		Role:     role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	str, err := token.SignedString(s.secret)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("sign access token: %w", err)
	}
	return str, expiresAt, nil
}

func (s *JWTService) ValidateToken(tokenStr string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return s.secret, nil
	})
	if err != nil {
		return nil, fmt.Errorf("parse token: %w", err)
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token claims")
	}
	return claims, nil
}

func (s *JWTService) GenerateRefreshToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate refresh token: %w", err)
	}
	return hex.EncodeToString(b), nil
}

func (s *JWTService) RefreshExpiresIn() time.Duration {
	return s.refreshExpiresIn
}

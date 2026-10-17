package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	AccessTokenTTL  = 15 * time.Minute
	RefreshTokenTTL = 7 * 24 * time.Hour
	tokenIssuer     = "sandboxed-autograder"
)

var ErrInvalidAccessToken = errors.New("invalid or expired access token")

type accessClaims struct {
	Role string `json:"role"`
	jwt.RegisteredClaims
}

// Tokens issues and validates short-lived access JWTs.
type Tokens struct {
	secret []byte
	now    func() time.Time
}

func NewTokens(secret []byte) (*Tokens, error) {
	if len(secret) < 32 {
		return nil, errors.New("JWT secret must be at least 32 bytes")
	}
	return &Tokens{secret: append([]byte(nil), secret...), now: time.Now}, nil
}

func (t *Tokens) IssueAccessToken(user User) (string, error) {
	now := t.now()
	claims := accessClaims{
		Role: user.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    tokenIssuer,
			Subject:   strconv.FormatInt(user.ID, 10),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(AccessTokenTTL)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(t.secret)
	if err != nil {
		return "", fmt.Errorf("sign access token: %w", err)
	}
	return signed, nil
}

func (t *Tokens) ParseAccessToken(raw string) (int64, error) {
	claims := &accessClaims{}
	token, err := jwt.ParseWithClaims(raw, claims, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, ErrInvalidAccessToken
		}
		return t.secret, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(tokenIssuer), jwt.WithExpirationRequired(), jwt.WithLeeway(0),
		jwt.WithTimeFunc(t.now))
	if err != nil || token == nil || !token.Valid || claims.Subject == "" {
		return 0, ErrInvalidAccessToken
	}
	userID, err := strconv.ParseInt(claims.Subject, 10, 64)
	if err != nil || userID <= 0 {
		return 0, ErrInvalidAccessToken
	}
	return userID, nil
}

func NewRefreshToken() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate refresh token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func HashRefreshToken(raw string) string {
	hash := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(hash[:])
}

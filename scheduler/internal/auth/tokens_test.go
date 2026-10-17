package auth

import (
	"testing"
	"time"
)

func TestAccessTokenValidityAndExpiry(t *testing.T) {
	now := time.Now().UTC()
	tokens, err := NewTokens([]byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatal(err)
	}
	tokens.now = func() time.Time { return now }
	raw, err := tokens.IssueAccessToken(User{ID: 12, Role: RoleStudent})
	if err != nil {
		t.Fatal(err)
	}
	if id, err := tokens.ParseAccessToken(raw); err != nil || id != 12 {
		t.Fatalf("ParseAccessToken = %d, %v; want 12, nil", id, err)
	}

	now = now.Add(AccessTokenTTL + time.Second)
	if _, err := tokens.ParseAccessToken(raw); err == nil {
		t.Fatal("expired access token was accepted")
	}
}

func TestNewTokensRequiresLongSecret(t *testing.T) {
	if _, err := NewTokens([]byte("too short")); err == nil {
		t.Fatal("NewTokens accepted a short secret")
	}
}

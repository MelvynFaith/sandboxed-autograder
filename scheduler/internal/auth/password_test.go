package auth

import (
	"strings"
	"testing"
)

func TestPasswordHashAndVerify(t *testing.T) {
	hash, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(hash, "$argon2id$v=19$") {
		t.Fatalf("hash does not use Argon2id: %q", hash)
	}
	valid, err := VerifyPassword(hash, "correct horse battery staple")
	if err != nil || !valid {
		t.Fatalf("correct password verification = %v, %v", valid, err)
	}
	valid, err = VerifyPassword(hash, "wrong")
	if err != nil || valid {
		t.Fatalf("wrong password verification = %v, %v", valid, err)
	}
}

func TestVerifyPasswordRejectsMalformedHash(t *testing.T) {
	if _, err := VerifyPassword("not-a-hash", "password"); err == nil {
		t.Fatal("malformed password hash was accepted")
	}
}

func TestHashPasswordRejectsEmptyAndOversizedPasswords(t *testing.T) {
	for _, password := range []string{"", strings.Repeat("a", 1025)} {
		if _, err := HashPassword(password); err == nil {
			t.Fatalf("HashPassword accepted input with length %d", len(password))
		}
	}
}

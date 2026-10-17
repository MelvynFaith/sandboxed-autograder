package auth

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/crypto/argon2"
)

var passwordHashSlots = make(chan struct{}, 3)

const (
	argonMemory      = 64 * 1024
	argonIterations  = 3
	argonParallelism = 2
	argonSaltLength  = 16
	argonKeyLength   = 32
)

// HashPassword stores passwords using Argon2id with a random salt.
func HashPassword(password string) (string, error) {
	return HashPasswordContext(context.Background(), password)
}

func HashPasswordContext(ctx context.Context, password string) (string, error) {
	if password == "" || len(password) > 1024 {
		return "", errors.New("password must be between 1 and 1024 bytes")
	}
	if err := acquirePasswordHashSlot(ctx); err != nil {
		return "", err
	}
	defer releasePasswordHashSlot()
	salt := make([]byte, argonSaltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate password salt: %w", err)
	}
	hash := argon2.IDKey([]byte(password), salt, argonIterations, argonMemory, argonParallelism, argonKeyLength)
	encodedSalt := base64.RawStdEncoding.EncodeToString(salt)
	encodedHash := base64.RawStdEncoding.EncodeToString(hash)
	return fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s",
		argonMemory, argonIterations, argonParallelism, encodedSalt, encodedHash), nil
}

// VerifyPassword compares a password with a PHC-encoded Argon2id hash.
func VerifyPassword(encodedHash, password string) (bool, error) {
	return VerifyPasswordContext(context.Background(), encodedHash, password)
}

func VerifyPasswordContext(ctx context.Context, encodedHash, password string) (bool, error) {
	if err := acquirePasswordHashSlot(ctx); err != nil {
		return false, err
	}
	defer releasePasswordHashSlot()
	parts := strings.Split(encodedHash, "$")
	if len(parts) != 6 || parts[1] != "argon2id" || parts[2] != "v=19" {
		return false, errors.New("invalid Argon2id password hash")
	}
	var memory, iterations, parallelism uint64
	for _, field := range strings.Split(parts[3], ",") {
		keyValue := strings.SplitN(field, "=", 2)
		if len(keyValue) != 2 {
			return false, errors.New("invalid Argon2id parameters")
		}
		value, err := strconv.ParseUint(keyValue[1], 10, 32)
		if err != nil {
			return false, errors.New("invalid Argon2id parameters")
		}
		switch keyValue[0] {
		case "m":
			memory = value
		case "t":
			iterations = value
		case "p":
			parallelism = value
		default:
			return false, errors.New("invalid Argon2id parameters")
		}
	}
	if memory < 8*1024 || memory > 1024*1024 ||
		iterations == 0 || iterations > 10 ||
		parallelism == 0 || parallelism > 16 {
		return false, errors.New("Argon2id parameters are outside safe bounds")
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil || len(salt) < 8 || len(salt) > 64 {
		return false, errors.New("invalid Argon2id salt")
	}
	expected, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil || len(expected) < 16 || len(expected) > 64 {
		return false, errors.New("invalid Argon2id digest")
	}
	actual := argon2.IDKey([]byte(password), salt, uint32(iterations), uint32(memory), uint8(parallelism), uint32(len(expected)))
	return subtle.ConstantTimeCompare(actual, expected) == 1, nil
}

func acquirePasswordHashSlot(ctx context.Context) error {
	select {
	case passwordHashSlots <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func releasePasswordHashSlot() {
	<-passwordHashSlots
}

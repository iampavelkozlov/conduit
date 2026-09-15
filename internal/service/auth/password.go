package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

const (
	argon2Version     = argon2.Version
	argon2MemoryKiB   = 19 * 1024
	argon2Iterations  = 2
	argon2Parallelism = 1
	argon2SaltLength  = 16
	argon2KeyLength   = 32
)

type PasswordManager struct {
	pepper string
}

func NewPasswordManager(pepper string) *PasswordManager {
	return &PasswordManager{pepper: pepper}
}

func (m *PasswordManager) Hash(password string) (string, error) {
	salt := make([]byte, argon2SaltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("read salt: %w", err)
	}

	hash := argon2.IDKey([]byte(password+m.pepper), salt, argon2Iterations, argon2MemoryKiB, argon2Parallelism, argon2KeyLength)
	return fmt.Sprintf(
		"$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2Version,
		argon2MemoryKiB,
		argon2Iterations,
		argon2Parallelism,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(hash),
	), nil
}

func (m *PasswordManager) Verify(password, encodedHash string) (bool, error) {
	parts := strings.Split(encodedHash, "$")
	if len(parts) != 6 {
		return false, errors.New("invalid password hash format")
	}
	if parts[1] != "argon2id" {
		return false, errors.New("unsupported password hash algorithm")
	}

	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2Version || parts[2] != fmt.Sprintf("v=%d", version) {
		return false, errors.New("unsupported password hash version")
	}
	var memory, iterations uint32
	var parallelism uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &iterations, &parallelism); err != nil {
		return false, errors.New("invalid password hash parameters")
	}
	if parts[3] != fmt.Sprintf("m=%d,t=%d,p=%d", memory, iterations, parallelism) ||
		memory == 0 || memory > 64*1024 || iterations == 0 || iterations > 10 || parallelism == 0 || parallelism > 4 {
		return false, errors.New("unsafe password hash parameters")
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false, fmt.Errorf("decode password salt: %w", err)
	}
	expectedHash, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false, fmt.Errorf("decode password hash: %w", err)
	}
	if len(salt) < 8 || len(salt) > 64 || len(expectedHash) != argon2KeyLength {
		return false, errors.New("invalid password hash length")
	}

	actualHash := argon2.IDKey([]byte(password+m.pepper), salt, iterations, memory, parallelism, argon2KeyLength)
	return subtle.ConstantTimeCompare(actualHash, expectedHash) == 1, nil
}

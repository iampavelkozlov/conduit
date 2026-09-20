package auth

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPasswordManager(t *testing.T) {
	manager := NewPasswordManager("test-pepper")
	hash, err := manager.Hash("correct horse battery staple")
	require.NoError(t, err)

	ok, err := manager.Verify("correct horse battery staple", hash)
	require.NoError(t, err)
	require.True(t, ok)

	ok, err = manager.Verify("wrong password", hash)
	require.NoError(t, err)
	require.False(t, ok)
}

func TestPasswordManagerRejectsMalformedHashes(t *testing.T) {
	manager := NewPasswordManager("test-pepper")
	hash, err := manager.Hash("password")
	require.NoError(t, err)

	tests := map[string]string{
		"invalid format":        "not-an-argon-hash",
		"unsupported algorithm": strings.Replace(hash, "argon2id", "bcrypt", 1),
		"unsupported version":   strings.Replace(hash, "v=19", "v=16", 1),
		"malformed parameters":  strings.Replace(hash, "m=19456,t=2,p=1", "bad", 1),
		"excessive memory":      strings.Replace(hash, "m=19456", "m=1048576", 1),
		"zero parallelism":      strings.Replace(hash, "p=1", "p=0", 1),
		"invalid salt encoding": "$argon2id$v=19$m=19456,t=2,p=1$%$YWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWE",
		"invalid hash encoding": "$argon2id$v=19$m=19456,t=2,p=1$YWFhYWFhYWFhYWFhYWFhYQ$%",
		"short salt":            "$argon2id$v=19$m=19456,t=2,p=1$YQ$YWFhYWFhYWFhYWFhYWFhYQ",
		"short hash":            "$argon2id$v=19$m=19456,t=2,p=1$YWFhYWFhYWFhYWFhYWFhYQ$YQ",
	}

	for name, encoded := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := manager.Verify("password", encoded)
			require.Error(t, err)
		})
	}
}

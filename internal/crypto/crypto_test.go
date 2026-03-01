package crypto

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validKey() string {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	return hex.EncodeToString(key)
}

func TestEncryptDecrypt_RoundTrip(t *testing.T) {
	t.Parallel()
	key := validKey()
	plaintext := "my-secret-password"

	ciphertext, err := Encrypt(plaintext, key)
	require.NoError(t, err)
	assert.NotEmpty(t, ciphertext)
	assert.NotEqual(t, plaintext, ciphertext)

	decrypted, err := Decrypt(ciphertext, key)
	require.NoError(t, err)
	assert.Equal(t, plaintext, decrypted)
}

func TestEncryptDecrypt_EmptyInput(t *testing.T) {
	t.Parallel()
	key := validKey()

	ciphertext, err := Encrypt("", key)
	require.NoError(t, err)

	decrypted, err := Decrypt(ciphertext, key)
	require.NoError(t, err)
	assert.Equal(t, "", decrypted)
}

func TestDecrypt_WrongKey(t *testing.T) {
	t.Parallel()
	key1 := validKey()
	key2 := make([]byte, 32)
	for i := range key2 {
		key2[i] = byte(i + 1)
	}
	key2Hex := hex.EncodeToString(key2)

	ciphertext, err := Encrypt("secret", key1)
	require.NoError(t, err)

	_, err = Decrypt(ciphertext, key2Hex)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "decryption failed")
}

func TestEncrypt_InvalidKeyLength(t *testing.T) {
	t.Parallel()
	_, err := Encrypt("text", "abcd")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "key must be 32 bytes")
}

func TestDecrypt_InvalidBase64(t *testing.T) {
	t.Parallel()
	_, err := Decrypt("not-valid-base64!!!", validKey())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid base64")
}

func TestDecrypt_TooShort(t *testing.T) {
	t.Parallel()
	_, err := Decrypt("YWJj", validKey())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ciphertext too short")
}

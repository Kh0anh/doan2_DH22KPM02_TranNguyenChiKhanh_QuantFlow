package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
)

// DeriveKey produces a stable 32-byte AES-256 key from an arbitrary-length raw
// key string by computing its SHA-256 hash. This allows AES_KEY in .env /
// docker-compose.yml to be any printable string rather than requiring it to be
// exactly 32 bytes.
func DeriveKey(rawKey string) []byte {
	sum := sha256.Sum256([]byte(rawKey))
	return sum[:]
}

// Encrypt encrypts plaintext using AES-256-GCM and returns a base64-encoded
// string of the format: base64(nonce || ciphertext || GCM-tag).
//
// A fresh random 12-byte nonce is generated for every call, which guarantees
// ciphertext uniqueness even when the same plaintext is encrypted twice.
//
// SRS NFR-SEC-01: "API Secret Key encrypted AES-256-GCM before storing in DB."
// The result is stored in api_keys.secret_key_encrypted (VARCHAR 512).
func Encrypt(plaintext []byte, key []byte) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("aes: new cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("aes: new gcm: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize()) // 12 bytes for GCM
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("aes: generate nonce: %w", err)
	}

	// Seal appends ciphertext+tag to nonce so the result is: nonce || ciphertext || tag.
	sealed := gcm.Seal(nonce, nonce, plaintext, nil)
	return base64.StdEncoding.EncodeToString(sealed), nil
}

// Decrypt reverses Encrypt. It base64-decodes the blob, extracts the 12-byte
// nonce, and returns the original plaintext bytes.
func Decrypt(encoded string, key []byte) ([]byte, error) {
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("aes: base64 decode: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("aes: new cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("aes: new gcm: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return nil, fmt.Errorf("aes: ciphertext too short")
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("aes: decrypt: %w", err)
	}
	return plaintext, nil
}

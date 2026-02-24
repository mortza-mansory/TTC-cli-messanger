// Package crypto provides end-to-end encryption for SecTherminal messages.
//
// All clients share the same AES-256-GCM key derived from a common passphrase.
// The relay server only sees ciphertext and cannot read messages or usernames.
//
// Encryption scheme:
//   - Key:       SHA-256 of shared passphrase  (32 bytes → AES-256)
//   - Cipher:    AES-GCM  (authenticated encryption, IND-CCA2)
//   - Nonce:     12 random bytes prepended to ciphertext
//   - Encoding:  Base64 (standard) for safe JSON transport
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"
)

// sharedPassphrase is the secret baked into every client binary.
// Change this before shipping — any client with a different passphrase
// cannot read messages from clients with the original one.
const sharedPassphrase = "SecTherminal-global-relay-key-v1 @#$%^&*()"

// globalKey is derived once at startup from the shared passphrase.
var globalKey = sha256.Sum256([]byte(sharedPassphrase))

// GlobalCrypto wraps AES-256-GCM encrypt/decrypt operations.
// It is stateless and safe to use from multiple goroutines.
type GlobalCrypto struct {
	key [32]byte
}

// NewGlobalCrypto returns a GlobalCrypto ready to use.
func NewGlobalCrypto() *GlobalCrypto {
	return &GlobalCrypto{key: globalKey}
}

// Encrypt encrypts plaintext with AES-256-GCM and returns a Base64 string.
// A fresh random 12-byte nonce is prepended to each ciphertext, so the same
// plaintext produces different output on every call.
func (gc *GlobalCrypto) Encrypt(plaintext []byte) (string, error) {
	block, err := aes.NewCipher(gc.key[:])
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	// Seal appends the ciphertext (+tag) to nonce, producing: nonce || ciphertext || tag
	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt decrypts a Base64-encoded AES-256-GCM ciphertext produced by Encrypt.
// Returns an error if the message was tampered with or the key is wrong.
func (gc *GlobalCrypto) Decrypt(encrypted string) ([]byte, error) {
	data, err := base64.StdEncoding.DecodeString(encrypted)
	if err != nil {
		return nil, err
	}

	block, err := aes.NewCipher(gc.key[:])
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	if len(data) < gcm.NonceSize() {
		return nil, errors.New("ciphertext too short")
	}

	nonce := data[:gcm.NonceSize()]
	ciphertext := data[gcm.NonceSize():]

	return gcm.Open(nil, nonce, ciphertext, nil)
}

// GenerateAccessKey derives a deterministic access key from the shared secret.
// Both the client and server must agree on this value.
// To match the server's hardcoded key, change the server to call this and
// use the result instead of a literal string.
func (gc *GlobalCrypto) GenerateAccessKey() string {
	combined := append(gc.key[:], []byte("ACCESS_GRANTED")...)
	hash := sha256.Sum256(combined)
	return base64.StdEncoding.EncodeToString(hash[:16])
}

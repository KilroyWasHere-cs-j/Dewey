package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"io"

	"github.com/google/uuid"
)

// createKey generates a random 32-byte key, the size AES-256 requires.
// Callers are responsible for persisting/retrieving it — this only
// generates the raw key material, it doesn't store it anywhere.
//
// Returns:
//   - []byte: 32 bytes of random key material, ready to pass to encrypt/decrypt.
//   - error: non-nil only if the system's random source (crypto/rand) fails to
//     fill the key, which should be treated as fatal rather than retried.
func createKey() ([]byte, error) {
	key := make([]byte, 32) // 32 bytes = AES-256
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	return key, nil
}

// generateKeyID returns a random opaque identifier for a keys row's key_id
// column. It has no relationship to the raw key material — it's only a
// lookup handle for wherever the actual key bytes end up stored — so a
// plain random UUID is enough; nothing about it needs to be derived from
// or tied to the key itself.
func generateKeyID() string {
	return uuid.NewString()
}

// encrypt returns nonce+ciphertext, all authenticated via AES-256-GCM.
// key must be exactly 32 bytes (AES-256).
//
// Args:
//   - key: the AES-256 key, exactly 32 bytes (see createKey). Wrong length
//     fails cipher setup.
//   - plaintext: the raw data to encrypt.
//
// Returns:
//   - []byte: a fresh random nonce followed by the GCM ciphertext+auth tag,
//     in the single format decrypt expects — nothing else needs to be
//     tracked alongside it to decrypt later.
//   - error: non-nil if cipher/GCM setup fails, or if reading the random
//     nonce fails.
func encrypt(key, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	// Nonce must be unique per encryption with the same key — never reused.
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	// Seal appends the nonce as a prefix so decrypt can pull it back out.
	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}

// decrypt expects the format produced by encrypt: nonce+ciphertext.
//
// Args:
//   - key: the same 32-byte AES-256 key that was used to encrypt. A
//     mismatched key fails the GCM auth check below rather than silently
//     producing garbage plaintext.
//   - data: nonce+ciphertext exactly as returned by encrypt.
//
// Returns:
//   - []byte: the original plaintext.
//   - error: non-nil if data is shorter than a nonce, or if GCM's auth
//     check fails (wrong key, or the ciphertext was corrupted/tampered
//     with).
func decrypt(key, data []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return nil, errors.New("ciphertext too short")
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	return gcm.Open(nil, nonce, ciphertext, nil)
}

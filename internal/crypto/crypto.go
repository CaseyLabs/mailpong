// Package crypto provides encryption utilities for secure credential storage
package crypto

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/crypto/pbkdf2"
)

const (
	// keyFileName is the name of the file storing the encryption key
	keyFileName = "device.key"

	// saltSize is the size of the salt in bytes
	saltSize = 32

	// keySize is the size of the derived key in bytes (AES-256)
	keySize = 32

	// pbkdf2Iterations is the number of PBKDF2 iterations
	pbkdf2Iterations = 100000
)

// Encryptor provides AES-256-GCM encryption/decryption
type Encryptor struct {
	key []byte
	// fallback is the key re-derived from current machine data when it
	// differs from the stored key. Older builds encrypted with the derived
	// key, so values saved after a hostname or user change need it.
	fallback []byte
}

// NewEncryptor creates a new Encryptor using a device-specific key
// The key is stored in the data directory and generated if it doesn't exist
func NewEncryptor(dataDir string) (*Encryptor, error) {
	keyPath := filepath.Join(dataDir, keyFileName)

	key, fallback, err := loadOrCreateKey(keyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load or create key: %w", err)
	}

	return &Encryptor{key: key, fallback: fallback}, nil
}

// loadOrCreateKey loads the encryption key from disk, or creates a new one.
// It also returns the key re-derived from the stored salt when that differs
// from the stored key (nil otherwise). An existing key file is never
// overwritten: losing it makes every stored secret unreadable, so a damaged
// one is moved aside for recovery before a new key is created.
func loadOrCreateKey(keyPath string) (key, fallback []byte, err error) {
	data, err := os.ReadFile(keyPath)
	switch {
	case err == nil && len(data) == saltSize+keySize:
		key = data[saltSize:]
		if derived := deriveKey(data[:saltSize]); !bytes.Equal(derived, key) {
			fallback = derived
		}
		return key, fallback, nil
	case err == nil:
		aside := fmt.Sprintf("%s.bad-%d", keyPath, time.Now().Unix())
		if rerr := os.Rename(keyPath, aside); rerr != nil {
			return nil, nil, fmt.Errorf("key file has unexpected size %d and could not be moved aside: %w", len(data), rerr)
		}
	case !errors.Is(err, fs.ErrNotExist):
		return nil, nil, fmt.Errorf("failed to read key file: %w", err)
	}

	// Generate new key
	salt := make([]byte, saltSize)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, nil, fmt.Errorf("failed to generate salt: %w", err)
	}

	key = deriveKey(salt)

	// Store salt (we can regenerate key from salt + machine data)
	keyData := make([]byte, saltSize+keySize)
	copy(keyData[:saltSize], salt)
	copy(keyData[saltSize:], key)

	// Create directory if needed
	if err := os.MkdirAll(filepath.Dir(keyPath), 0700); err != nil {
		return nil, nil, fmt.Errorf("failed to create key directory: %w", err)
	}

	// Write key file with restricted permissions
	f, err := os.OpenFile(keyPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create key file: %w", err)
	}
	if _, err := f.Write(keyData); err != nil {
		f.Close()
		return nil, nil, fmt.Errorf("failed to write key file: %w", err)
	}
	if err := f.Close(); err != nil {
		return nil, nil, fmt.Errorf("failed to write key file: %w", err)
	}

	return key, nil, nil
}

// deriveKey derives an encryption key from salt and machine-specific data
func deriveKey(salt []byte) []byte {
	// Use machine-specific data as the base for key derivation
	// This includes hostname and user info for some uniqueness
	hostname, _ := os.Hostname()
	username := os.Getenv("USER")
	if username == "" {
		username = os.Getenv("USERNAME")
	}

	// Combine machine-specific data
	// Legacy identifier from Aerion; persisted, do not rename. (KDF input; changing it breaks stored secrets.)
	machineData := fmt.Sprintf("aerion:%s:%s:%d", hostname, username, os.Getuid())

	// Derive key using PBKDF2
	return pbkdf2.Key([]byte(machineData), salt, pbkdf2Iterations, keySize, sha256.New)
}

// Encrypt encrypts plaintext using AES-256-GCM
// Returns base64-encoded ciphertext (nonce prepended)
func (e *Encryptor) Encrypt(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}

	block, err := aes.NewCipher(e.key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	// Generate random nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Encrypt and prepend nonce
	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)

	// Return base64-encoded result
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt decrypts base64-encoded ciphertext (with prepended nonce)
func (e *Encryptor) Decrypt(ciphertext string) (string, error) {
	if ciphertext == "" {
		return "", nil
	}

	// Decode base64
	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", fmt.Errorf("failed to decode ciphertext: %w", err)
	}

	plaintext, err := open(e.key, data)
	if err != nil && e.fallback != nil {
		if p, ferr := open(e.fallback, data); ferr == nil {
			return p, nil
		}
	}
	return plaintext, err
}

// open decrypts data (nonce prepended) with AES-256-GCM under key.
func open(key, data []byte) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}

	// Extract nonce and ciphertext
	nonce, ciphertextBytes := data[:nonceSize], data[nonceSize:]

	// Decrypt
	plaintext, err := gcm.Open(nil, nonce, ciphertextBytes, nil)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt: %w", err)
	}

	return string(plaintext), nil
}

package encryption

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"

	"golang.org/x/crypto/pbkdf2"
	"golang.org/x/xerrors"
)

const (
	SaltSize       = 32
	IterationCount = 600000
	KeySize        = 32
)

func deriveKey(channelID string, salt []byte) []byte {
	return pbkdf2.Key([]byte(channelID), salt, IterationCount, KeySize, sha256.New)
}

func Encrypt(channelID string, data []byte) ([]byte, error) {
	salt := make([]byte, SaltSize)
	if _, err := rand.Read(salt); err != nil {
		return nil, xerrors.Errorf("failed to generate random salt: %w", err)
	}

	key := deriveKey(channelID, salt)

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, xerrors.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, xerrors.Errorf("failed to create GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, xerrors.Errorf("failed to generate nonce: %w", err)
	}

	ciphertext := gcm.Seal(nonce, nonce, data, nil)

	result := make([]byte, len(salt)+len(ciphertext))
	copy(result[:len(salt)], salt)
	copy(result[len(salt):], ciphertext)

	return result, nil
}

func Decrypt(channelID string, encryptedData []byte) ([]byte, error) {
	if len(encryptedData) < SaltSize {
		return nil, xerrors.New("encrypted data too short")
	}

	salt := encryptedData[:SaltSize]
	ciphertext := encryptedData[SaltSize:]

	key := deriveKey(channelID, salt)

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, xerrors.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, xerrors.Errorf("failed to create GCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, xerrors.New("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, xerrors.Errorf("failed to decrypt data: %w", err)
	}

	return plaintext, nil
}

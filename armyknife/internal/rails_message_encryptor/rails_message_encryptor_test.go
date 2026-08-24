package rails_message_encryptor_test

import (
	"armyknife/internal/rails_message_encryptor"
	"testing"
)

func TestEncryptAES_GCM(t *testing.T) {
	secret := "0123456789abcdef0123456789abcdef"

	message := "test"
	encryptor := rails_message_encryptor.NewRailsMessageEncryptor(secret, &rails_message_encryptor.Options{})
	encrypted, err := encryptor.Encrypt(message)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	decrypted, err := encryptor.Decrypt(*encrypted)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if *decrypted != message {
		t.Errorf("Expected %q, got %q", message, *decrypted)
	}
}

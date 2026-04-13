package rails_message_encryptor

import (
	"armyknife/internal/marshal/ruby"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"

	"golang.org/x/xerrors"
)

const (
	AuthTagLength = 16
	Separator     = "--"
)

type Cipher string

const (
	AESGCM Cipher = "AES-GCM"
)

type RailsMessageEncryptor struct {
	secret []byte
	cipher Cipher
}

type Options struct {
	Cipher Cipher
}

func NewRailsMessageEncryptor(secret string, options *Options) *RailsMessageEncryptor {
	c := AESGCM
	if options != nil {
		if options.Cipher != "" {
			c = options.Cipher
		}
	}

	return &RailsMessageEncryptor{
		secret: []byte(secret),
		cipher: c,
	}
}

func (m *RailsMessageEncryptor) Encrypt(value string) (*string, error) {
	switch m.cipher {
	case AESGCM:
		decodedSecret := make([]byte, hex.DecodedLen(len(m.secret)))
		if _, err := hex.Decode(decodedSecret, m.secret); err != nil {
			return nil, xerrors.Errorf("failed to decode hex secret: %w", err)
		}
		block, err := aes.NewCipher(decodedSecret)
		if err != nil {
			return nil, xerrors.Errorf("failed to create new cipher: %w", err)
		}

		gcm, err := cipher.NewGCM(block)
		if err != nil {
			return nil, xerrors.Errorf("failed to create new GCM: %w", err)
		}

		iv := make([]byte, gcm.NonceSize())
		if _, err := rand.Read(iv); err != nil {
			return nil, xerrors.Errorf("failed to generate random iv: %w", err)
		}

		serialized, err := ruby.Dump(value)
		if err != nil {
			return nil, xerrors.Errorf("failed to serialize value: %w", err)
		}
		result := gcm.Seal(nil, iv, []byte(serialized), nil)
		l := len(result)
		encryptedData := result[:l-AuthTagLength]
		tag := result[l-AuthTagLength:]

		s := strings.Join([]string{
			base64.StdEncoding.EncodeToString(encryptedData),
			base64.StdEncoding.EncodeToString(iv),
			base64.StdEncoding.EncodeToString(tag),
		}, Separator)
		return &s, nil

	default:
		return nil, errors.New("unsupported cipher")
	}
}

func (m *RailsMessageEncryptor) Decrypt(message string) (*string, error) {
	switch m.cipher {
	case AESGCM:
		parts := strings.Split(message, Separator)
		if len(parts) != 3 {
			return nil, errors.New("invalid message format")
		}

		data, err := base64.StdEncoding.DecodeString(parts[0])
		if err != nil {
			return nil, xerrors.Errorf("failed to decode base64 data: %w", err)
		}

		iv, err := base64.StdEncoding.DecodeString(parts[1])
		if err != nil {
			return nil, xerrors.Errorf("failed to decode base64 iv: %w", err)
		}

		tag, err := base64.StdEncoding.DecodeString(parts[2])
		if err != nil {
			return nil, xerrors.Errorf("failed to decode base64 tag: %w", err)
		}

		decodedSecret := make([]byte, hex.DecodedLen(len(m.secret)))
		if _, err := hex.Decode(decodedSecret, m.secret); err != nil {
			return nil, xerrors.Errorf("failed to decode hex secret: %w", err)
		}
		block, err := aes.NewCipher(decodedSecret)
		if err != nil {
			return nil, xerrors.Errorf("failed to create new cipher: %w", err)
		}

		gcm, err := cipher.NewGCM(block)
		if err != nil {
			return nil, xerrors.Errorf("failed to create new GCM: %w", err)
		}

		result, err := gcm.Open(nil, iv, append(data, tag...), nil)
		if err != nil {
			return nil, xerrors.Errorf("failed to decrypt data: %w", err)
		}

		deserialized, err := ruby.Load(string(result))
		if err != nil {
			return nil, xerrors.Errorf("failed to deserialize data: %w", err)
		}
		s := deserialized.(string)
		return &s, nil
	default:
		return nil, errors.New("unsupported cipher")
	}
}

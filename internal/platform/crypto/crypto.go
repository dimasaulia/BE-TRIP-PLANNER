// Package crypto holds the token helpers: opaque random tokens with hash-only
// storage, AES-256-GCM for refresh tokens and HMAC for short lived cookies.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/open-suite/boilerplate-golang/internal/platform/config"
)

var ErrKeyMissing = errors.New("TOKEN_ENC_KEY is not configured")

type Keys struct {
	encKey  []byte
	signKey []byte
}

func New(cfg config.Config) (*Keys, error) {
	keys := &Keys{}

	if cfg.Auth.TokenEncKey != "" {
		key, err := parseKey(cfg.Auth.TokenEncKey)
		if err != nil {
			return nil, err
		}
		keys.encKey = key

		mac := hmac.New(sha256.New, key)
		mac.Write([]byte("cookie-signing"))
		keys.signKey = mac.Sum(nil)
		return keys, nil
	}

	// Without a configured key cookies are still signed, with a per process key.
	keys.signKey = make([]byte, 32)
	if _, err := rand.Read(keys.signKey); err != nil {
		return nil, err
	}
	return keys, nil
}

// parseKey accepts 64 hex characters, base64 of 32 bytes, or a raw 32 byte string.
func parseKey(value string) ([]byte, error) {
	value = strings.TrimSpace(value)

	if decoded, err := hex.DecodeString(value); err == nil && len(decoded) == 32 {
		return decoded, nil
	}
	for _, encoding := range []*base64.Encoding{base64.StdEncoding, base64.RawStdEncoding, base64.URLEncoding, base64.RawURLEncoding} {
		if decoded, err := encoding.DecodeString(value); err == nil && len(decoded) == 32 {
			return decoded, nil
		}
	}
	if len(value) == 32 {
		return []byte(value), nil
	}

	return nil, fmt.Errorf("TOKEN_ENC_KEY must be 32 bytes (hex, base64 or raw)")
}

// RandomToken returns n random bytes as unpadded base64url.
func RandomToken(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// HashToken is the only form of a session or invite token that is stored.
func HashToken(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}

func (k *Keys) Encrypt(plaintext []byte) ([]byte, error) {
	if k.encKey == nil {
		return nil, ErrKeyMissing
	}

	gcm, err := k.gcm()
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}

	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

func (k *Keys) Decrypt(ciphertext []byte) ([]byte, error) {
	if k.encKey == nil {
		return nil, ErrKeyMissing
	}

	gcm, err := k.gcm()
	if err != nil {
		return nil, err
	}
	if len(ciphertext) < gcm.NonceSize() {
		return nil, errors.New("ciphertext too short")
	}

	nonce, body := ciphertext[:gcm.NonceSize()], ciphertext[gcm.NonceSize():]
	return gcm.Open(nil, nonce, body, nil)
}

func (k *Keys) gcm() (cipher.AEAD, error) {
	block, err := aes.NewCipher(k.encKey)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

// Sign returns `<payload>.<mac>` with both parts base64url encoded.
func (k *Keys) Sign(payload []byte) string {
	mac := hmac.New(sha256.New, k.signKey)
	mac.Write(payload)

	return base64.RawURLEncoding.EncodeToString(payload) + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func (k *Keys) Verify(signed string) ([]byte, bool) {
	parts := strings.Split(signed, ".")
	if len(parts) != 2 {
		return nil, false
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, false
	}
	given, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, false
	}

	mac := hmac.New(sha256.New, k.signKey)
	mac.Write(payload)
	if subtle.ConstantTimeCompare(mac.Sum(nil), given) != 1 {
		return nil, false
	}

	return payload, true
}

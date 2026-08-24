package crypto

import (
	"testing"
)

const testKey = "0123456789abcdef0123456789abcdef"

func TestEncryptDecryptRoundtrip(t *testing.T) {
	c, err := NewCipher(testKey)
	if err != nil {
		t.Fatalf("NewCipher: %v", err)
	}
	plaintext := "hunter2-secret-password"
	enc, err := c.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	if enc == "" || enc == plaintext {
		t.Fatalf("ciphertext must not equal plaintext")
	}
	dec, err := c.Decrypt(enc)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if dec != plaintext {
		t.Fatalf("roundtrip mismatch: got %q want %q", dec, plaintext)
	}
}

func TestCiphertextIsRandomized(t *testing.T) {
	c, _ := NewCipher(testKey)
	a, _ := c.Encrypt("same-value")
	b, _ := c.Encrypt("same-value")
	if a == b {
		t.Fatalf("encrypting the same value twice must produce different ciphertext")
	}
}

func TestDecryptWithWrongKeyFails(t *testing.T) {
	c, _ := NewCipher(testKey)
	enc, _ := c.Encrypt("secret")
	other, err := NewCipher("fedcba9876543210fedcba9876543210")
	if err != nil {
		t.Fatalf("NewCipher: %v", err)
	}
	if _, err := other.Decrypt(enc); err == nil {
		t.Fatalf("decrypting with a different key must fail")
	}
}

func TestNewCipherRejectsShortKey(t *testing.T) {
	if _, err := NewCipher("short"); err == nil {
		t.Fatalf("short key must be rejected")
	}
	if _, err := NewCipher(""); err == nil {
		t.Fatalf("empty key must be rejected")
	}
}

func TestNewCipherAcceptsBase64Key(t *testing.T) {
	// base64("0123456789abcdef0123456789abcdef")
	const b64 = "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY="
	c, err := NewCipher(b64)
	if err != nil {
		t.Fatalf("base64 key must be accepted: %v", err)
	}
	enc, err := c.Encrypt("secret-value")
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	dec, err := c.Decrypt(enc)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if dec != "secret-value" {
		t.Fatalf("roundtrip mismatch: %q", dec)
	}
}

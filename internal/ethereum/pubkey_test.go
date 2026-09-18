package ethereum

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/asn1"
	"encoding/pem"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
)

func TestPublicKeyPEMToAddress(t *testing.T) {
	// Generate a real secp256k1 key and use it to create a valid PEM fixture.
	ethPrivKey, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("crypto.GenerateKey failed: %v", err)
	}
	ethPubKey := ethPrivKey.Public().(*ecdsa.PublicKey)
	expectedAddr := crypto.PubkeyToAddress(*ethPubKey)

	validPEM, err := MarshalPublicKeyToPKIXPEM(ethPubKey)
	if err != nil {
		t.Fatalf("MarshalPublicKeyToPKIXPEM failed: %v", err)
	}

	// Verify the round-trip: PEM -> address should match direct derivation.
	got, err := PublicKeyPEMToAddress(validPEM)
	if err != nil {
		t.Fatalf("PublicKeyPEMToAddress() error = %v", err)
	}
	if got != expectedAddr.Hex() {
		t.Errorf("PublicKeyPEMToAddress() = %q, want %q", got, expectedAddr.Hex())
	}

	tests := []struct {
		name        string
		pem         string
		wantAddr    string
		wantErr     bool
		errContains string
	}{
		{
			name:     "valid secp256k1 PEM produces correct address",
			pem:      validPEM,
			wantAddr: expectedAddr.Hex(),
			wantErr:  false,
		},
		{
			name:        "empty string returns error",
			pem:         "",
			wantErr:     true,
			errContains: "invalid PEM",
		},
		{
			name:        "invalid base64 content",
			pem:         "-----BEGIN PUBLIC KEY-----\nnot-valid-base64!!!\n-----END PUBLIC KEY-----\n",
			wantErr:     true,
			errContains: "invalid PEM",
		},
		{
			name:        "wrong PEM block type",
			pem:         "-----BEGIN RSA PUBLIC KEY-----\nMFwwDQYJKoZIhvcNAQEBBQADSwAwSAJBALjEu9y+\n-----END RSA PUBLIC KEY-----\n",
			wantErr:     true,
			errContains: "invalid PEM block type",
		},
		{
			name:        "P-256 key (not secp256k1) returns error",
			pem:         generateP256PublicKeyPEM(t),
			wantErr:     true,
			errContains: "invalid secp256k1 public key",
		},
		{
			name:        "non-PEM text",
			pem:         "this is not a PEM block at all",
			wantErr:     true,
			errContains: "invalid PEM",
		},
		{
			name:     "PEM with leading/trailing whitespace is trimmed",
			pem:      "  \n" + validPEM + "\n  ",
			wantAddr: expectedAddr.Hex(),
			wantErr:  false,
		},
		{
			name:     "PEM with newline padding inside is still valid",
			pem:      strings.TrimSpace(validPEM),
			wantAddr: expectedAddr.Hex(),
			wantErr:  false,
		},
		{
			name:        "truncated public key bytes (too short)",
			pem:         generateTruncatedPublicKeyPEM(t, 32),
			wantErr:     true,
			errContains: "invalid public key length",
		},
		{
			name:        "compressed public key format (0x02/0x03 prefix, not 0x04)",
			pem:         generateCompressedPublicKeyPEM(t),
			wantErr:     true,
			errContains: "expected 0x04 (uncompressed)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			addr, err := PublicKeyPEMToAddress(tt.pem)
			if tt.wantErr {
				if err == nil {
					t.Errorf("PublicKeyPEMToAddress() = %q, want error containing %q", addr, tt.errContains)
					return
				}
				if !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("PublicKeyPEMToAddress() error = %q, want error containing %q", err.Error(), tt.errContains)
				}
				return
			}
			if err != nil {
				t.Errorf("PublicKeyPEMToAddress() unexpected error: %v", err)
				return
			}
			if addr != tt.wantAddr {
				t.Errorf("PublicKeyPEMToAddress() = %q, want %q", addr, tt.wantAddr)
			}
		})
	}
}

func TestPublicKeyPEMToAddress_KnownFixture(t *testing.T) {
	// Use a deterministic private key to produce a known address.
	ethPrivKey, err := crypto.HexToECDSA("0000000000000000000000000000000000000000000000000000000000000001")
	if err != nil {
		t.Fatalf("HexToECDSA failed: %v", err)
	}
	ethPubKey := ethPrivKey.Public().(*ecdsa.PublicKey)
	addr := crypto.PubkeyToAddress(*ethPubKey)

	pemStr, err := MarshalPublicKeyToPKIXPEM(ethPubKey)
	if err != nil {
		t.Fatalf("MarshalPublicKeyToPKIXPEM failed: %v", err)
	}

	got, err := PublicKeyPEMToAddress(pemStr)
	if err != nil {
		t.Fatalf("PublicKeyPEMToAddress() error = %v", err)
	}
	if got != addr.Hex() {
		t.Errorf("PublicKeyPEMToAddress() = %q, want %q", got, addr.Hex())
	}
}

// generateP256PublicKeyPEM creates a PKIX PEM block containing a P-256 (NIST P-256) public key.
// Since stdlib x509.MarshalPKIXPublicKey does support P-256, we can use it directly.
func generateP256PublicKeyPEM(t *testing.T) string {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey failed: %v", err)
	}
	pubBytes, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		t.Fatalf("MarshalPKIXPublicKey failed: %v", err)
	}
	block := &pem.Block{Type: "PUBLIC KEY", Bytes: pubBytes}
	return string(pem.EncodeToMemory(block))
}

// generateTruncatedPublicKeyPEM creates a PEM with an invalid (too-short) public key payload.
func generateTruncatedPublicKeyPEM(t *testing.T, keyLen int) string {
	t.Helper()
	truncatedBytes := make([]byte, keyLen)
	derBytes := buildPKIXDER(t, truncatedBytes)
	block := &pem.Block{Type: "PUBLIC KEY", Bytes: derBytes}
	return string(pem.EncodeToMemory(block))
}

// generateCompressedPublicKeyPEM creates a PEM where the key starts with 0x02 (compressed).
func generateCompressedPublicKeyPEM(t *testing.T) string {
	t.Helper()
	ethPrivKey, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("crypto.GenerateKey failed: %v", err)
	}
	pubKey := ethPrivKey.Public().(*ecdsa.PublicKey)
	fullBytes := crypto.FromECDSAPub(pubKey) // 65 bytes: 0x04 || X || Y

	// Build a BIT STRING starting with 0x02 (compressed format) — not 0x04.
	// This should be rejected since we only accept uncompressed (0x04) format.
	compressedKey := make([]byte, len(fullBytes))
	compressedKey[0] = 0x02 // Wrong prefix
	copy(compressedKey[1:], fullBytes[1:])

	derBytes := buildPKIXDER(t, compressedKey)
	block := &pem.Block{Type: "PUBLIC KEY", Bytes: derBytes}
	return string(pem.EncodeToMemory(block))
}

// buildPKIXDER constructs a valid DER-encoded SubjectPublicKeyInfo for secp256k1 with the given raw key bytes.
func buildPKIXDER(t *testing.T, pubBytes []byte) []byte {
	t.Helper()
	type algorithmIdentifier struct {
		Algorithm  asn1.ObjectIdentifier
		Parameters asn1.ObjectIdentifier
	}
	type subjectPublicKeyInfo struct {
		Algorithm        algorithmIdentifier
		SubjectPublicKey asn1.BitString `asn1:"tag:3"`
	}
	spki := subjectPublicKeyInfo{
		Algorithm: algorithmIdentifier{
			Algorithm:  asn1.ObjectIdentifier{1, 2, 840, 10045, 2, 1},
			Parameters: asn1.ObjectIdentifier{1, 3, 132, 0, 10},
		},
		SubjectPublicKey: asn1.BitString{
			Bytes:     pubBytes,
			BitLength: len(pubBytes) * 8,
		},
	}
	der, err := asn1.Marshal(spki)
	if err != nil {
		t.Fatalf("asn1.Marshal(SubjectPublicKeyInfo) failed: %v", err)
	}
	return der
}

func TestMarshalPublicKeyToPKIXPEM(t *testing.T) {
	ethPrivKey, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("crypto.GenerateKey failed: %v", err)
	}
	pubKey := ethPrivKey.Public().(*ecdsa.PublicKey)

	pemStr, err := MarshalPublicKeyToPKIXPEM(pubKey)
	if err != nil {
		t.Fatalf("MarshalPublicKeyToPKIXPEM failed: %v", err)
	}

	// Verify it's a valid PEM.
	if !strings.HasPrefix(pemStr, "-----BEGIN PUBLIC KEY-----") {
		t.Errorf("MarshalPublicKeyToPKIXPEM() output does not start with PEM header")
	}
	if !strings.Contains(pemStr, "-----END PUBLIC KEY-----") {
		t.Errorf("MarshalPublicKeyToPKIXPEM() output does not contain PEM footer")
	}

	// Verify round-trip.
	addr, err := PublicKeyPEMToAddress(pemStr)
	if err != nil {
		t.Fatalf("PublicKeyPEMToAddress() failed on round-trip PEM: %v", err)
	}
	expected := crypto.PubkeyToAddress(*pubKey)
	if addr != expected.Hex() {
		t.Errorf("Round-trip address = %q, want %q", addr, expected.Hex())
	}
}

package ethereum

import (
	"crypto/ecdsa"
	"encoding/asn1"
	"encoding/pem"
	"errors"
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/crypto"
)

// pkixPublicKey mirrors the ASN.1 structure of SubjectPublicKeyInfo (RFC 5280).
// This allows us to manually parse PKIX-encoded secp256k1 public keys that the
// standard library's x509.ParsePKIXPublicKey does not support.
type pkixPublicKey struct {
	Algo      asn1.RawValue
	BitString asn1.RawValue
}

// PublicKeyPEMToAddress parses a PEM-encoded PKIX public key and returns the corresponding Ethereum address (with EIP-55 checksum).
// The input must be a PKIX-format PEM block of type "PUBLIC KEY" containing a secp256k1 ECDSA public key.
// Returns an error if the PEM is malformed, the key type is unsupported, or the curve is not secp256k1.
func PublicKeyPEMToAddress(pemStr string) (string, error) {
	pemStr = strings.TrimSpace(pemStr)
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return "", errors.New("invalid PEM: no valid block found, expected 'PUBLIC KEY' type")
	}
	if block.Type != "PUBLIC KEY" {
		return "", fmt.Errorf("invalid PEM block type: expected 'PUBLIC KEY', got '%s'", block.Type)
	}

	var pki pkixPublicKey
	if _, err := asn1.Unmarshal(block.Bytes, &pki); err != nil {
		return "", fmt.Errorf("failed to parse PKIX public key: %w", err)
	}

	// pki.BitString.Bytes contains: [unused_bits_byte (0x00)] + [uncompressed_pubkey (65 bytes)]
	// We strip the first byte (unused bits = 0) to get the actual 65-byte public key.
	raw := pki.BitString.Bytes
	if len(raw) < 1 {
		return "", errors.New("BIT STRING content is empty")
	}
	pubBytes := raw[1:]
	if len(pubBytes) != 65 {
		return "", fmt.Errorf("invalid public key length: expected 65 bytes (uncompressed 0x04||X||Y), got %d", len(pubBytes))
	}
	if pubBytes[0] != 0x04 {
		return "", fmt.Errorf("invalid public key format: expected 0x04 (uncompressed), got 0x%02x", pubBytes[0])
	}

	// crypto.UnmarshalPubkey validates the point is on secp256k1 and returns the parsed key.
	ecdsaPub, err := crypto.UnmarshalPubkey(pubBytes)
	if err != nil {
		return "", fmt.Errorf("failed to parse secp256k1 public key: %w", err)
	}
	if ecdsaPub.Curve != crypto.S256() {
		return "", fmt.Errorf("unexpected curve: %s (only secp256k1 is supported)", ecdsaPub.Curve.Params().Name)
	}

	addr := crypto.PubkeyToAddress(*ecdsaPub)
	return addr.Hex(), nil
}

// MarshalPublicKeyToPKIXPEM marshals a secp256k1 ECDSA public key to PKIX PEM format.
// This is the inverse of PublicKeyPEMToAddress for a given key pair.
// Uses asn1.Marshal with struct tags to produce RFC 5280-compatible DER encoding.
func MarshalPublicKeyToPKIXPEM(pub *ecdsa.PublicKey) (string, error) {
	pubBytes := crypto.FromECDSAPub(pub)
	if len(pubBytes) == 0 {
		return "", errors.New("invalid public key: failed to marshal to bytes")
	}

	// Build the DER-encoded SubjectPublicKeyInfo using asn1.Marshal with struct tags.
	// AlgorithmIdentifier: { OID(id-ecPublicKey), OID(secp256k1) }
	// subjectPublicKey: BIT STRING { 0x00 (unused bits), uncompressed pubkey bytes }
	type algorithmIdentifier struct {
		Algorithm  asn1.ObjectIdentifier
		Parameters asn1.ObjectIdentifier
	}
	type subjectPublicKeyInfo struct {
		Algorithm        algorithmIdentifier
		SubjectPublicKey asn1.BitString `asn1:"tag:3"` // BIT STRING tag = 3
	}

	spki := subjectPublicKeyInfo{
		Algorithm: algorithmIdentifier{
			Algorithm:  asn1.ObjectIdentifier{1, 2, 840, 10045, 2, 1}, // id-ecPublicKey
			Parameters: asn1.ObjectIdentifier{1, 3, 132, 0, 10},       // secp256k1
		},
		SubjectPublicKey: asn1.BitString{
			Bytes:     pubBytes, // Already includes 0x04 prefix
			BitLength: len(pubBytes) * 8,
		},
	}

	derBytes, err := asn1.Marshal(spki)
	if err != nil {
		return "", fmt.Errorf("failed to marshal SubjectPublicKeyInfo: %w", err)
	}

	block := &pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: derBytes,
	}
	return string(pem.EncodeToMemory(block)), nil
}

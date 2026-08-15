package ethereum

import (
	"fmt"
	"strings"
)

func ValidateAddress(address string) bool {
	if len(address) < 42 {
		return false
	}

	if address[:2] != "0x" {
		return false
	}

	hexPart := address[2:]
	if len(hexPart) != 40 {
		return false
	}

	return isHexString(hexPart)
}

func ValidateContractAddress(address string) bool {
	return ValidateAddress(address)
}

func IsPureNumber(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return len(s) > 0
}

func NormalizeHex(address string) string {
	if address == "" {
		return ""
	}

	lower := strings.ToLower(address)

	if strings.HasPrefix(lower, "0x") {
		return lower[2:]
	}

	if len(address) == 64 && isHexString(address) {
		return strings.ToLower(address)
	}

	if len(address) == 40 && isHexString(address) {
		return strings.ToLower(address)
	}

	return ""
}

func isHexString(s string) bool {
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

func HexToAddress(hex string) ([]byte, error) {
	hex = strings.TrimPrefix(hex, "0x")
	if len(hex) != 40 {
		return nil, fmt.Errorf("invalid address length: expected 40 hex characters, got %d", len(hex))
	}

	if !isHexString(hex) {
		return nil, fmt.Errorf("invalid hex characters in address")
	}

	result := make([]byte, 20)
	for i := 0; i < 20; i++ {
		var val byte
		c := hex[i*2]
		switch {
		case c >= '0' && c <= '9':
			val = c - '0'
		case c >= 'a' && c <= 'f':
			val = c - 'a' + 10
		case c >= 'A' && c <= 'F':
			val = c - 'A' + 10
		default:
			return nil, fmt.Errorf("invalid hex character: %c", c)
		}
		val <<= 4

		c = hex[i*2+1]
		switch {
		case c >= '0' && c <= '9':
			val |= c - '0'
		case c >= 'a' && c <= 'f':
			val |= c - 'a' + 10
		case c >= 'A' && c <= 'F':
			val |= c - 'A' + 10
		default:
			return nil, fmt.Errorf("invalid hex character: %c", c)
		}
		result[i] = val
	}

	return result, nil
}

func AddressToHex(address []byte) string {
	if len(address) != 20 {
		return ""
	}

	const hexChars = "0123456789abcdef"
	result := make([]byte, 42)
	result[0] = '0'
	result[1] = 'x'
	for i, b := range address {
		result[i*2+2] = hexChars[b>>4]
		result[i*2+3] = hexChars[b&0x0f]
	}
	return string(result)
}

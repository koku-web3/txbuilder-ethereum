package ethereum

import (
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/crypto"
)

const (
	addressLen = 40
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

// IsEIP55Format 检查地址是否使用 EIP-55 混合大小写校验和格式。
// 如果十六进制部分同时包含大写和小写字母，则返回 true。
func IsEIP55Format(address string) bool {
	if len(address) < 2 || address[:2] != "0x" {
		return false
	}
	hexPart := address[2:]
	if len(hexPart) != addressLen {
		return false
	}
	hasUpper := false
	hasLower := false
	for _, c := range hexPart {
		if c >= 'A' && c <= 'F' {
			hasUpper = true
		}
		if c >= 'a' && c <= 'f' {
			hasLower = true
		}
		if hasUpper && hasLower {
			return true
		}
	}
	return false
}

// ConvertToChecksumAddress 将小写地址转换为 EIP-55 校验和格式。
// 地址必须是有效的以太坊地址（0x 前缀 + 40 位十六进制字符）。
// 返回带混合大小写校验和编码的地址。
//
// EIP-55 算法逻辑：
// 1. 将地址转为小写（不含 0x 前缀）
// 2. 用 Keccak-256 对小写地址字符串进行哈希
// 3. 遍历地址的每个字符，如果是对应哈希 nibble >= 8 的字母，则大写
// 示例：0xd8da6bf26964af9d7eed9e03e53415d37aa96045
//
//	→ 0xd8dA6BF26964aF9D7eEd9e03E53415D37aA96045
func ConvertToChecksumAddress(address string) string {
	if len(address) < 2 || address[:2] != "0x" {
		return address
	}
	hexPart := address[2:]
	if len(hexPart) != addressLen {
		return address
	}

	// 对小写十六进制地址（不含 0x 前缀）进行哈希
	hash := crypto.Keccak256([]byte(strings.ToLower(hexPart)))

	result := make([]byte, 42)
	copy(result[:2], address[:2])

	for i := 0; i < addressLen; i++ {
		addrChar := hexPart[i]
		hashByte := hash[i/2]
		if i%2 == 0 {
			hashByte = hashByte >> 4
		} else {
			hashByte &= 0x0f
		}
		// 只有当字符是字母且对应哈希 nibble > 7 时才大写
		if addrChar >= 'a' && addrChar <= 'f' && hashByte > 7 {
			result[i+2] = addrChar - 32
		} else {
			result[i+2] = addrChar
		}
	}
	return string(result)
}

// VerifyChecksum 验证 EIP-55 校验和地址是否正确。
// 如果地址不是 EIP-55 格式（全小写或全大写），则返回 true（无需校验）。
// 如果校验和不匹配则返回 false。
func VerifyChecksum(address string) bool {
	if !IsEIP55Format(address) {
		return true
	}
	checksummed := ConvertToChecksumAddress(address)
	return checksummed == address
}

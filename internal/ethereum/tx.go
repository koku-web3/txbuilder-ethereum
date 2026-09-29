package ethereum

import (
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strings"
)

// BuildERC20TransferData 构建 ERC20 transfer 调用的 calldata
// 格式：methodID(4字节) + 地址(32字节, 左补零) + 金额(32字节, 左补零)
// transfer(address to, uint256 amount)
func BuildERC20TransferData(to string, amount *big.Int) ([]byte, error) {
	methodID := []byte{0xa9, 0x05, 0x9c, 0xbb}

	paddedAddr, err := PadAddressTo32BytesBytes(to)
	if err != nil {
		return nil, err
	}
	paddedAmount, err := PadAmountTo32Bytes(amount)
	if err != nil {
		return nil, err
	}

	data := make([]byte, 0, 4+32+32)
	data = append(data, methodID...)
	data = append(data, paddedAddr...)
	data = append(data, paddedAmount...)

	return data, nil
}

// PadAddressTo32Bytes 将地址补零到 32 字节（Hex 编码字符串）
// Ethereum ABI 编码要求参数固定 32 字节
// 示例：0x1234 -> 0x0000...00001234（左补零至 32 字节）
//
// 校验失败时返回 error，避免把非法地址静默补零成零地址后
// 查询到「零地址」的余额并返回看似正确的结果。
func PadAddressTo32Bytes(address string) (string, error) {
	addrBytes, err := parseAddressBytes(address)
	if err != nil {
		return "", err
	}

	padded := make([]byte, 32)
	copy(padded[32-len(addrBytes):], addrBytes)
	return hex.EncodeToString(padded), nil
}

// PadAddressTo32BytesBytes 将地址补零到 32 字节（字节数组版本）
// 以太坊地址是 20 字节的 hex 编码（40 个十六进制字符），需要先转换为字节数组再补零
func PadAddressTo32BytesBytes(address string) ([]byte, error) {
	addrBytes, err := parseAddressBytes(address)
	if err != nil {
		return nil, err
	}

	padded := make([]byte, 32)
	copy(padded[32-len(addrBytes):], addrBytes)
	return padded, nil
}

// parseAddressBytes 解析地址为字节数组，校验长度不超过 32 字节。
// 超长地址会导致后续 copy 时切片下标为负，触发 runtime panic，必须提前拦截。
func parseAddressBytes(address string) ([]byte, error) {
	address = strings.TrimPrefix(address, "0x")
	address = strings.ToLower(address)

	addrBytes, err := hex.DecodeString(address)
	if err != nil {
		return nil, fmt.Errorf("invalid address %q: %w", address, err)
	}
	if len(addrBytes) > 32 {
		return nil, fmt.Errorf("invalid address %q: %d bytes exceeds 32", address, len(addrBytes))
	}
	return addrBytes, nil
}

// PadAmountTo32Bytes 将金额补零到 32 字节
// Ethereum ABI 编码要求 uint256 参数固定 32 字节
// 示例：大数 0x1234567890 -> 0x0000...001234567890（左补零至 32 字节）
func PadAmountTo32Bytes(amount *big.Int) ([]byte, error) {
	amountBytes := amount.Bytes()
	if len(amountBytes) > 32 {
		return nil, fmt.Errorf("amount %s exceeds 32 bytes (uint256)", amount.String())
	}
	padded := make([]byte, 32)
	copy(padded[32-len(amountBytes):], amountBytes)
	return padded, nil
}

// ParseAmount 将字符串金额解析为 *big.Int（单位：wei）
func ParseAmount(amount string) (*big.Int, error) {
	amountInt, ok := new(big.Int).SetString(amount, 10)
	if !ok {
		return nil, errors.New("invalid amount format")
	}
	return amountInt, nil
}

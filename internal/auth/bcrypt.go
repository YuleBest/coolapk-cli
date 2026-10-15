package auth

import (
	"fmt"
	"strings"

	"golang.org/x/crypto/blowfish"
)

// 酷安 v3 token 需要「指定盐」的 bcrypt，x/crypto/bcrypt 只暴露随机盐接口，
// 这里用 golang.org/x/crypto/blowfish 复刻 bcrypt 的哈希过程。
const (
	bcryptCost     = 4
	bcryptSaltSize = 16
	bcryptPrefix   = "$2y$04$"
	bcryptMagic    = "OrpheanBeholderScryDoubt"

	// bcrypt 使用自己的 base64 字符表，且不补位。
	bcryptAlphabet = "./ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	stdAlphabet    = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
)

// bcryptHash 返回 "$2y$04$" + 22 字符盐 + 31 字符摘要。
func bcryptHash(password string, salt []byte) (string, error) {
	// C 版 bcrypt 在扩展密钥时带上结尾的 NUL；少了这一步服务端会判定 token 无效。
	key := append([]byte(password), 0)
	cipher, err := blowfish.NewSaltedCipher(key, salt)
	if err != nil {
		return "", fmt.Errorf("auth: bcrypt 初始化失败: %w", err)
	}
	for i := 0; i < 1<<bcryptCost; i++ {
		blowfish.ExpandKey(key, cipher)
		blowfish.ExpandKey(salt, cipher)
	}

	data := []byte(bcryptMagic)
	for i := 0; i < len(data); i += 8 {
		for j := 0; j < 64; j++ {
			cipher.Encrypt(data[i:i+8], data[i:i+8])
		}
	}

	saltPart, err := bcryptBase64Encode(salt, 22)
	if err != nil {
		return "", err
	}
	// 与 C 实现保持一致，只编码前 23 字节。
	hashPart, err := bcryptBase64Encode(data[:23], 31)
	if err != nil {
		return "", err
	}
	return bcryptPrefix + saltPart + hashPart, nil
}

func bcryptBase64Encode(src []byte, size int) (string, error) {
	out := make([]byte, 0, size)
	var bits, nbits uint32
	for _, b := range src {
		bits = bits<<8 | uint32(b)
		nbits += 8
		for nbits >= 6 {
			nbits -= 6
			out = append(out, bcryptAlphabet[(bits>>nbits)&0x3f])
		}
	}
	if nbits > 0 {
		out = append(out, bcryptAlphabet[(bits<<(6-nbits))&0x3f])
	}
	if len(out) < size {
		return "", fmt.Errorf("auth: bcrypt 编码长度不足 (%d < %d)", len(out), size)
	}
	return string(out[:size]), nil
}

func bcryptBase64Decode(value string, size int) ([]byte, error) {
	out := make([]byte, 0, size)
	var bits, nbits uint32
	for i := 0; i < len(value); i++ {
		index := strings.IndexByte(bcryptAlphabet, value[i])
		if index < 0 {
			return nil, fmt.Errorf("%w: %q", errInvalidSalt, value[i])
		}
		bits = bits<<6 | uint32(index)
		nbits += 6
		if nbits >= 8 {
			nbits -= 8
			out = append(out, byte(bits>>nbits))
		}
	}
	return out, nil
}

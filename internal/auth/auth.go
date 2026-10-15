// Package auth 负责酷安 V6 接口要求的设备码与 X-App-Token。
//
// Token 是 2026 年客户端使用的 v3 方案：按时间戳从内嵌的 auth_blob 取出密码表片段，
// 再用 bcrypt(cost=4) 生成。算法参考 MIT 许可的 coolapk-desktop
// (src-tauri/src/coolapk/auth.rs) 与 coolapk-mcp (coolapk_mcp/auth/token_v3.py)，
// auth_blob.b64 取自 coolapk-desktop。
package auth

import (
	"crypto/md5"
	"crypto/rand"
	_ "embed"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
)

// AppID 与 AppCode 是酷安客户端版本标识，token 生成与请求头都依赖它们。
const (
	AppID   = "com.coolapk.market"
	AppCode = 2604201
)

//go:embed auth_blob.b64
var authBlobB64 string

var (
	blobOnce sync.Once
	blobData []byte
	blobErr  error
)

// errInvalidSalt 表示该时间戳推导出的盐含有 bcrypt 字符集之外的字符，
// 与参考实现一致，此时换下一个时间戳再试。
var errInvalidSalt = errors.New("auth: 盐含有 bcrypt 字符集之外的字符")

func authBlob() ([]byte, error) {
	blobOnce.Do(func() {
		blobData, blobErr = base64.StdEncoding.DecodeString(strings.TrimSpace(authBlobB64))
		if blobErr != nil {
			blobErr = fmt.Errorf("auth: 解码 auth_blob 失败: %w", blobErr)
		}
	})
	return blobData, blobErr
}

// Token 返回设备在 unix 秒时间戳下的 v3 X-App-Token。
// 个别时间点推导出的盐不满足 bcrypt 字符集，最多向后探测 15 秒。
func Token(device string, unix int64) (string, error) {
	blob, err := authBlob()
	if err != nil {
		return "", err
	}
	var lastErr error
	for offset := int64(0); offset <= 15; offset++ {
		token, err := tokenAt(blob, device, unix+offset)
		if err == nil {
			return token, nil
		}
		if !errors.Is(err, errInvalidSalt) {
			return "", err
		}
		lastErr = err
	}
	return "", fmt.Errorf("auth: 16 秒内没有可用盐: %w", lastErr)
}

func tokenAt(blob []byte, device string, unix int64) (string, error) {
	index := int((unix+AppCode)%100)*4 + 0x80
	if index+0x80 > len(blob) {
		return "", fmt.Errorf("auth: auth_blob 越界 (index=%d, len=%d)", index, len(blob))
	}
	segment, err := base64.StdEncoding.DecodeString(string(blob[index : index+0x80]))
	if err != nil {
		return "", fmt.Errorf("auth: 解码 auth_blob 片段失败: %w", err)
	}

	plain := fmt.Sprintf("%s&%s&%s&%d&%d", AppID, segment, md5Hex(device), unix, AppCode)
	password := md5Hex(base64.StdEncoding.EncodeToString([]byte(plain)))

	saltSource := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%x/%s", unix, md5Hex(plain))))
	saltSource = strings.TrimRight(saltSource, "=")
	if len(saltSource) < 22 {
		return "", errors.New("auth: 推导出的盐过短")
	}
	last := saltSource[21]
	salt22 := saltSource[:21] + string(stdAlphabet[(strings.IndexByte(stdAlphabet, last)-5+64)%64])

	salt, err := bcryptBase64Decode(salt22, bcryptSaltSize)
	if err != nil {
		return "", err
	}
	hash, err := bcryptHash(password, salt)
	if err != nil {
		return "", err
	}
	return "v3" + strings.TrimRight(base64.StdEncoding.EncodeToString([]byte(hash)), "="), nil
}

func md5Hex(value string) string {
	sum := md5.Sum([]byte(value))
	return hex.EncodeToString(sum[:])
}

// DeviceCode 生成酷安风格的 X-App-Device：设备信息串做 Base64，
// 去掉补位后再整体反转。设备信息里的随机部分每次生成都不同。
func DeviceCode() (string, error) {
	serial := make([]byte, 8)
	mac := make([]byte, 6)
	if _, err := rand.Read(serial); err != nil {
		return "", fmt.Errorf("auth: 生成设备码失败: %w", err)
	}
	if _, err := rand.Read(mac); err != nil {
		return "", fmt.Errorf("auth: 生成设备码失败: %w", err)
	}
	mac[0] = (mac[0] | 2) & 0xfe
	raw := fmt.Sprintf("%s; ; ; %02x:%02x:%02x:%02x:%02x:%02x; Xiaomi; Xiaomi; 23127PN0CC; BP2A.250605.031; null",
		hex.EncodeToString(serial), mac[0], mac[1], mac[2], mac[3], mac[4], mac[5])
	encoded := strings.TrimRight(base64.StdEncoding.EncodeToString([]byte(raw)), "=")
	return reverse(encoded), nil
}

func reverse(value string) string {
	runes := []rune(value)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return string(runes)
}

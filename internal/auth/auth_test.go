package auth

import (
	"encoding/base64"
	"strings"
	"testing"
)

// 该向量与 coolapk-desktop(Rust) / 参考 Python 实现在同一时间戳下的输出一致。
func TestTokenMatchesReferenceVector(t *testing.T) {
	const want = "v3JDJ5JDA0JE5qazRNREJsT0RBdk9HUmpOamt4TWVweGRValpNcG1xU094ajA5THpYaTdLTkhod1JzRWdH"
	got, err := Token("synthetic-go-probe-0001", 1770000000)
	if err != nil {
		t.Fatalf("Token() 出错: %v", err)
	}
	if got != want {
		t.Fatalf("Token() = %q, 期望 %q", got, want)
	}
}

func TestTokenStableForSameTimestamp(t *testing.T) {
	first, err := Token("device-x", 1770000000)
	if err != nil {
		t.Fatalf("Token() 出错: %v", err)
	}
	second, err := Token("device-x", 1770000000)
	if err != nil {
		t.Fatalf("Token() 出错: %v", err)
	}
	if first != second {
		t.Fatalf("同一时间戳应得到相同 token: %q != %q", first, second)
	}
	if !strings.HasPrefix(first, "v3") || len(first) < 70 {
		t.Fatalf("token 形态异常: %q", first)
	}
}

// 时间戳前移 1 秒时盐不同，token 必须跟着变。
func TestTokenChangesWithTimestamp(t *testing.T) {
	first, err := Token("device-x", 1770000000)
	if err != nil {
		t.Fatalf("Token() 出错: %v", err)
	}
	second, err := Token("device-x", 1770000001)
	if err != nil {
		t.Fatalf("Token() 出错: %v", err)
	}
	if first == second {
		t.Fatal("不同时间戳不应得到相同 token")
	}
}

func TestTokenProbesNextTimestamps(t *testing.T) {
	blob, err := authBlob()
	if err != nil {
		t.Fatalf("authBlob() 出错: %v", err)
	}
	// 找一个盐不合法的起始时间戳，确认会向后探测出可用 token。
	var start int64
	for unix := int64(1770000000); unix < 1770000100; unix++ {
		if _, err := tokenAt(blob, "device-x", unix); err != nil {
			start = unix
			break
		}
	}
	if start == 0 {
		t.Skip("100 秒内没有出现需要探测的时间戳")
	}
	if _, err := Token("device-x", start); err != nil {
		t.Fatalf("Token() 应能在探测后生成 token: %v", err)
	}
}

func TestDeviceCodeShape(t *testing.T) {
	code, err := DeviceCode()
	if err != nil {
		t.Fatalf("DeviceCode() 出错: %v", err)
	}
	if strings.Contains(code, "=") {
		t.Fatalf("设备码不应含补位符: %q", code)
	}
	raw, err := base64.StdEncoding.DecodeString(reverse(code))
	if err != nil {
		t.Fatalf("设备码应是反转的 Base64: %v", err)
	}
	if !strings.HasSuffix(string(raw), "; null") {
		t.Fatalf("设备信息串形态异常: %q", raw)
	}

	other, err := DeviceCode()
	if err != nil {
		t.Fatalf("DeviceCode() 出错: %v", err)
	}
	if code == other {
		t.Fatal("两次生成的设备码应当不同")
	}
}

func TestBcryptBase64RejectsForeignAlphabet(t *testing.T) {
	if _, err := bcryptBase64Decode("Njk4MDBlODAvOGRjNjkxM+", 16); err == nil {
		t.Fatal("含 '+' 的盐应当报错")
	} else if !strings.Contains(err.Error(), "字符集") {
		t.Fatalf("错误信息应说明字符集问题: %v", err)
	}
}

func TestBcryptBase64RoundTrip(t *testing.T) {
	salt, err := bcryptBase64Decode("Njk4MDBlODAvOGRjNjkxMe", bcryptSaltSize)
	if err != nil {
		t.Fatalf("解码失败: %v", err)
	}
	if len(salt) != bcryptSaltSize {
		t.Fatalf("盐长度 = %d, 期望 %d", len(salt), bcryptSaltSize)
	}
	encoded, err := bcryptBase64Encode(salt, 22)
	if err != nil {
		t.Fatalf("编码失败: %v", err)
	}
	if encoded != "Njk4MDBlODAvOGRjNjkxMe" {
		t.Fatalf("编码结果 = %q", encoded)
	}
}

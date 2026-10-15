package main

import (
	"io"
	"os"
	"strings"
	"testing"

	"github.com/YuleBest/coolapk-cli/internal/api"
)

func TestPickSearchKind(t *testing.T) {
	if _, _, err := pickSearchKind("", "", ""); err == nil {
		t.Fatal("未指定搜索类型应当报错")
	}
	if _, _, err := pickSearchKind("a", "b", ""); err == nil {
		t.Fatal("同时指定多个搜索类型应当报错")
	}
	kind, query, err := pickSearchKind(" 小米13 ", "", "")
	if err != nil {
		t.Fatalf("pickSearchKind() 出错: %v", err)
	}
	if kind != api.SearchFeed || query != "小米13" {
		t.Fatalf("kind = %q, query = %q", kind, query)
	}
	kind, _, err = pickSearchKind("", "", "微信")
	if err != nil || kind != api.SearchApk {
		t.Fatalf("kind = %q, err = %v", kind, err)
	}
}

func TestRunExitCodes(t *testing.T) {
	cases := []struct {
		name       string
		args       []string
		wantCode   int
		wantStderr string
		wantStdout string
	}{
		{name: "版本", args: []string{"version"}, wantCode: 0, wantStdout: "coolapk "},
		{name: "帮助", args: []string{"--help"}, wantCode: 0, wantStdout: "coolapk"},
		{name: "未知命令", args: []string{"foobar"}, wantCode: 2, wantStderr: "用法错误"},
		{name: "搜索缺少类型", args: []string{"search"}, wantCode: 2, wantStderr: "--feed"},
		{name: "搜索类型冲突", args: []string{"search", "--feed", "a", "--user", "b"}, wantCode: 2, wantStderr: "只能选一个"},
		{name: "搜索位置参数", args: []string{"search", "小米13"}, wantCode: 2, wantStderr: "位置参数"},
		{name: "搜索页码非法", args: []string{"search", "--feed", "a", "--page", "0"}, wantCode: 2, wantStderr: "--page"},
		{name: "动态 ID 非法", args: []string{"feed", "abc"}, wantCode: 2, wantStderr: "动态 ID 必须是数字"},
		{name: "动态 ID 缺失", args: []string{"feed"}, wantCode: 2, wantStderr: "需要一个动态 ID"},
		{name: "uid 非法", args: []string{"user", "wherewhere"}, wantCode: 2, wantStderr: "uid 必须是数字"},
		{name: "未知参数", args: []string{"search", "--nope"}, wantCode: 2, wantStderr: "用法错误"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			code, stdout, stderr := captureRun(t, testCase.args)
			if code != testCase.wantCode {
				t.Fatalf("退出码 = %d, 期望 %d (stderr: %s)", code, testCase.wantCode, stderr)
			}
			if testCase.wantStderr != "" && !strings.Contains(stderr, testCase.wantStderr) {
				t.Fatalf("stderr 缺少 %q: %s", testCase.wantStderr, stderr)
			}
			if testCase.wantStdout != "" && !strings.Contains(stdout, testCase.wantStdout) {
				t.Fatalf("stdout 缺少 %q: %s", testCase.wantStdout, stdout)
			}
		})
	}
}

func captureRun(t *testing.T, args []string) (int, string, string) {
	t.Helper()
	outReader, outWriter, err := os.Pipe()
	if err != nil {
		t.Fatalf("创建管道失败: %v", err)
	}
	errReader, errWriter, err := os.Pipe()
	if err != nil {
		t.Fatalf("创建管道失败: %v", err)
	}
	originalOut, originalErr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = outWriter, errWriter
	code := run(args)
	os.Stdout, os.Stderr = originalOut, originalErr
	outWriter.Close()
	errWriter.Close()
	stdout, _ := io.ReadAll(outReader)
	stderr, _ := io.ReadAll(errReader)
	return code, string(stdout), string(stderr)
}

package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadGeneratesAndPersistsDevice(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configHome)
	t.Setenv("COOLAPK_DEVICE", "")
	t.Setenv("COOLAPK_COOKIE", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() 出错: %v", err)
	}
	if cfg.Device == "" {
		t.Fatal("首次运行应生成设备码")
	}
	if cfg.Warning != "" {
		t.Fatalf("不应有警告: %s", cfg.Warning)
	}

	path := filepath.Join(configHome, "coolapk", "config.json")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("配置文件未写入: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("配置文件权限 = %v, 期望 0600", info.Mode().Perm())
	}

	again, err := Load()
	if err != nil {
		t.Fatalf("Load() 出错: %v", err)
	}
	if again.Device != cfg.Device {
		t.Fatalf("设备码应当复用: %q != %q", again.Device, cfg.Device)
	}
}

func TestEnvironmentOverridesFile(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("COOLAPK_DEVICE", "env-device")
	t.Setenv("COOLAPK_COOKIE", "uid=1; token=abc")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() 出错: %v", err)
	}
	if cfg.Device != "env-device" {
		t.Fatalf("device = %q", cfg.Device)
	}
	if cfg.Cookie != "uid=1; token=abc" {
		t.Fatalf("cookie = %q", cfg.Cookie)
	}
	if _, err := os.Stat(filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "coolapk", "config.json")); !os.IsNotExist(err) {
		t.Fatalf("环境变量提供设备码时不应写入配置文件: %v", err)
	}
}

func TestBrokenConfigReportsError(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configHome)
	t.Setenv("COOLAPK_DEVICE", "")
	t.Setenv("COOLAPK_COOKIE", "")
	dir := filepath.Join(configHome, "coolapk")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("创建目录失败: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte("{"), 0o600); err != nil {
		t.Fatalf("写入配置失败: %v", err)
	}
	if _, err := Load(); err == nil {
		t.Fatal("损坏的配置文件应当报错")
	}
}

// Package config 管理本地配置：设备标识与可选的登录 Cookie。
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/YuleBest/coolapk-cli/internal/auth"
)

// Config 是 coolapk 的本地配置，环境变量 COOLAPK_DEVICE / COOLAPK_COOKIE 优先。
type Config struct {
	Device string `json:"device"`
	Cookie string `json:"cookie"`

	// Warning 记录不影响运行但需要提示用户的问题，例如配置写盘失败。
	Warning string `json:"-"`
}

// Path 返回配置文件路径，通常是 ~/.config/coolapk/config.json。
func Path() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("无法定位用户配置目录: %w", err)
	}
	return filepath.Join(dir, "coolapk", "config.json"), nil
}

// Load 读取配置，缺失的设备码会就地生成并写入配置文件。
// 配置文件不可写不影响本次运行，只会通过 Warning 提醒。
func Load() (*Config, error) {
	path, pathErr := Path()
	cfg := &Config{}
	if pathErr == nil {
		data, err := os.ReadFile(path)
		switch {
		case err == nil:
			if err := json.Unmarshal(data, cfg); err != nil {
				return nil, fmt.Errorf("配置文件 %s 解析失败: %w", path, err)
			}
		case !os.IsNotExist(err):
			return nil, fmt.Errorf("读取配置文件 %s 失败: %w", path, err)
		}
	}

	if device := os.Getenv("COOLAPK_DEVICE"); device != "" {
		cfg.Device = device
	} else if cfg.Device == "" {
		device, err := auth.DeviceCode()
		if err != nil {
			return nil, err
		}
		cfg.Device = device
		if pathErr != nil {
			cfg.Warning = fmt.Sprintf("无法定位用户配置目录，本次设备码不会保存: %v", pathErr)
		} else if err := save(path, cfg); err != nil {
			cfg.Warning = fmt.Sprintf("设备码写入 %s 失败，本次仍可继续使用: %v", path, err)
		}
	}
	if cookie := os.Getenv("COOLAPK_COOKIE"); cookie != "" {
		cfg.Cookie = cookie
	}
	return cfg, nil
}

func save(path string, cfg *Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o600)
}

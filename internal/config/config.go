package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

const CurrentVersion = 1

const DefaultTOML = `# Bilibili CLI settings.
version = 1

[output]
# auto, rich, json, or yaml.
format = "auto"

[network]
timeout_seconds = 30

[safety]
# Block account write actions and dangerous operations.
read_only = false
# Ask before deleting a dynamic or unfollowing an account.
confirm_dangerous_actions = true
`

type Config struct {
	Version int           `toml:"version"`
	Output  OutputConfig  `toml:"output"`
	Network NetworkConfig `toml:"network"`
	Safety  SafetyConfig  `toml:"safety"`
}

type OutputConfig struct {
	Format string `toml:"format"`
}

type NetworkConfig struct {
	TimeoutSeconds int `toml:"timeout_seconds"`
}

type SafetyConfig struct {
	ReadOnly                bool `toml:"read_only"`
	ConfirmDangerousActions bool `toml:"confirm_dangerous_actions"`
}

func Default() Config {
	return Config{
		Version: CurrentVersion,
		Output: OutputConfig{
			Format: "auto",
		},
		Network: NetworkConfig{
			TimeoutSeconds: 30,
		},
		Safety: SafetyConfig{
			ReadOnly:                false,
			ConfirmDangerousActions: true,
		},
	}
}

type Store struct {
	Dir  string
	File string
}

func NewStore() *Store {
	if dir := strings.TrimSpace(os.Getenv("BILI_CONFIG_DIR")); dir != "" {
		return &Store{Dir: dir, File: filepath.Join(dir, "config.toml")}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	dir := filepath.Join(home, ".config", "bilibili-cli")
	return &Store{Dir: dir, File: filepath.Join(dir, "config.toml")}
}

func (s *Store) Load() (Config, error) {
	data, err := os.ReadFile(s.File)
	if errors.Is(err, os.ErrNotExist) {
		return Default(), nil
	}
	if err != nil {
		return Config{}, fmt.Errorf("读取配置失败: %w", err)
	}
	config, migrated, err := decode(data)
	if err != nil {
		return Config{}, err
	}
	if migrated {
		if err := s.Save(config); err != nil {
			return Config{}, err
		}
	}
	return config, nil
}

func (s *Store) Exists() bool {
	_, err := os.Stat(s.File)
	return err == nil
}

func (s *Store) Ensure() (Config, error) {
	data, err := os.ReadFile(s.File)
	if errors.Is(err, os.ErrNotExist) {
		config := Default()
		if err := s.write([]byte(DefaultTOML)); err != nil {
			return Config{}, err
		}
		return config, nil
	}
	if err != nil {
		return Config{}, fmt.Errorf("读取配置失败: %w", err)
	}
	config, migrated, err := decode(data)
	if err != nil {
		return Config{}, err
	}
	if migrated {
		if err := s.Save(config); err != nil {
			return Config{}, err
		}
	}
	return config, nil
}

func (s *Store) Save(config Config) error {
	config, _, err := migrate(config, config.Version)
	if err != nil {
		return err
	}
	config, _, err = validate(config)
	if err != nil {
		return err
	}
	data, err := toml.Marshal(config)
	if err != nil {
		return fmt.Errorf("编码配置失败: %w", err)
	}
	return s.write(data)
}

func (s *Store) write(data []byte) error {
	if err := os.MkdirAll(s.Dir, 0o700); err != nil {
		return fmt.Errorf("创建配置目录失败: %w", err)
	}
	temporary, err := os.CreateTemp(s.Dir, ".config-*.toml")
	if err != nil {
		return fmt.Errorf("创建临时配置失败: %w", err)
	}
	temporaryName := temporary.Name()
	defer os.Remove(temporaryName)
	if err := temporary.Chmod(0o600); err != nil {
		temporary.Close()
		return fmt.Errorf("设置配置权限失败: %w", err)
	}
	if _, err := temporary.Write(data); err != nil {
		temporary.Close()
		return fmt.Errorf("写入配置失败: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return fmt.Errorf("同步配置失败: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("关闭临时配置失败: %w", err)
	}
	if err := os.Rename(temporaryName, s.File); err != nil {
		return fmt.Errorf("替换配置失败: %w", err)
	}
	return nil
}

func decode(data []byte) (Config, bool, error) {
	var version struct {
		Version int `toml:"version"`
	}
	if err := toml.Unmarshal(data, &version); err != nil {
		return Config{}, false, fmt.Errorf("解析 config.toml 失败: %w", err)
	}
	config := Default()
	if err := toml.Unmarshal(data, &config); err != nil {
		return Config{}, false, fmt.Errorf("解析 config.toml 失败: %w", err)
	}
	config, migrated, err := migrate(config, version.Version)
	if err != nil {
		return Config{}, false, err
	}
	normalized, changed, err := validate(config)
	if err != nil {
		return Config{}, false, err
	}
	return normalized, migrated || changed, nil
}

func validate(config Config) (Config, bool, error) {
	changed := false
	format := strings.ToLower(strings.TrimSpace(config.Output.Format))
	if format == "" {
		format = Default().Output.Format
		changed = true
	}
	switch format {
	case "auto", "rich", "json", "yaml":
		config.Output.Format = format
	default:
		return Config{}, false, fmt.Errorf("output.format 不支持: %s", config.Output.Format)
	}
	if config.Network.TimeoutSeconds == 0 {
		config.Network.TimeoutSeconds = Default().Network.TimeoutSeconds
		changed = true
	}
	if config.Network.TimeoutSeconds < 1 || config.Network.TimeoutSeconds > 300 {
		return Config{}, false, fmt.Errorf("network.timeout_seconds 必须在 1 到 300 之间")
	}
	return config, changed, nil
}

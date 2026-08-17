package config

import "fmt"

func migrate(config Config, sourceVersion int) (Config, bool, error) {
	switch {
	case sourceVersion == 0:
		config.Version = CurrentVersion
		return config, true, nil
	case sourceVersion == CurrentVersion:
		return config, false, nil
	case sourceVersion > CurrentVersion:
		return Config{}, false, fmt.Errorf("config.toml 版本 %d 高于当前支持版本 %d", sourceVersion, CurrentVersion)
	default:
		return Config{}, false, fmt.Errorf("不支持的 config.toml 版本: %d", sourceVersion)
	}
}

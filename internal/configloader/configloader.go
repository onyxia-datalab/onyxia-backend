package configloader

import (
	"bytes"
	"fmt"
	"log/slog"
	"strings"

	"github.com/spf13/viper"
)

// Load reads default config, merges with external file and environment variables,
// and returns the config struct of type T.
func Load[T any](defaults []byte, filePath string) (T, error) {
	var cfg T

	viper.SetConfigType("yaml")

	if err := viper.ReadConfig(bytes.NewReader(defaults)); err != nil {
		return cfg, fmt.Errorf("failed to read embedded default config: %w", err)
	}
	slog.Info("Successfully loaded embedded default config")

	if filePath != "" {
		viper.SetConfigFile(filePath)
		if err := viper.MergeInConfig(); err == nil {
			slog.Info("Loaded external config file", slog.String("file", filePath))
		} else {
			slog.Warn("No external config file found", slog.String("file", filePath))
		}
	}

	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	if err := viper.Unmarshal(&cfg); err != nil {
		return cfg, fmt.Errorf("failed to parse environment configuration: %w", err)
	}

	return cfg, nil
}

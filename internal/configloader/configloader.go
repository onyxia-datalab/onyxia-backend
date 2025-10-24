package configloader

import (
	"bytes"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/spf13/viper"
)

const keyDelimiter = "::"

// Load reads default config, merges with external file and environment variables,
// and returns the config struct of type T.
func Load[T any](defaults []byte, filePath string) (T, error) {
	var cfg T

	// Create an isolated Viper instance to avoid shared state
	v := viper.NewWithOptions(viper.KeyDelimiter(keyDelimiter))
	v.SetConfigType("yaml")

	if err := v.ReadConfig(bytes.NewReader(defaults)); err != nil {
		return cfg, fmt.Errorf("failed to read embedded default config: %w", err)
	}
	slog.Info("Successfully loaded embedded default config")

	if filePath != "" {
		v.SetConfigFile(filePath)
		if err := v.MergeInConfig(); err == nil {
			slog.Info("Loaded external config file", slog.String("file", filePath))
		} else {
			slog.Warn("No external config file found", slog.String("file", filePath))
		}
	}

	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	for _, key := range v.AllKeys() {
		envKey := strings.ToUpper(strings.NewReplacer(".", "_", keyDelimiter, "_").Replace(key))
		if val, ok := os.LookupEnv(envKey); ok {
			v.Set(key, val)
		}
	}

	if err := v.Unmarshal(&cfg); err != nil {
		return cfg, fmt.Errorf("failed to parse environment configuration: %w", err)
	}

	return cfg, nil
}

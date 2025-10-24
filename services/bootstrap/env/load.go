package env

import (
	_ "embed"

	"github.com/onyxia-datalab/onyxia-backend/internal/configloader"
)

//go:embed env.default.yaml
var defaultConfig []byte

func New() (Env, error) {
	cfg, err := configloader.Load[Env](defaultConfig, "env.yaml")
	if err != nil {
		return Env{}, err
	}

	if err := ValidateCatalogsConfig(cfg.CatalogsConfig); err != nil {
		return Env{}, err
	}

	return cfg, nil
}

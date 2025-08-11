package bootstrap

import (
	"fmt"
	"log/slog"
)

type Application struct {
	Env *Env
}

func NewApplication() (*Application, error) {

	env, err := NewEnv()
	if err != nil {
		return nil, fmt.Errorf("failed to load environment: %w", err)

	}

	app := &Application{
		Env: &env,
	}

	slog.Info("Application initialized successfully")

	return app, nil
}

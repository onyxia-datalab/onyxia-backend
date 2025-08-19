package logging

import (
	"log/slog"

	"go.uber.org/zap"
	"go.uber.org/zap/exp/zapslog"
)

func NewLogger(
	zapLogger *zap.Logger,
	attrFunc AttrFunc,
	handlerOptions ...zapslog.HandlerOption,
) (*slog.Logger, func() error, error) {

	zl := zapLogger
	if zl == nil {
		built, err := zap.NewProduction()
		if err != nil {
			return nil, nil, err
		}
		zl = built
	}

	base := zapslog.NewHandler(
		zl.Core(),
		handlerOptions...,
	)

	h := newContextAttrsHandler(base, attrFunc)

	logger := slog.New(h)

	flush := func() error {
		return zl.Sync()
	}

	return logger, flush, nil
}

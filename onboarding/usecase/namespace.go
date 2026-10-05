package usecase

import (
	"context"
	"fmt"
	"log/slog"
	"maps"
	"time"

	"github.com/onyxia-datalab/onyxia-backend/onboarding/port"
)

func (s *onboardingUsecase) createNamespace(ctx context.Context, name string) error {
	result, err := s.namespaceService.CreateNamespace(
		ctx,
		name,
		s.getNamespaceAnnotations(ctx),
		s.namespace.NamespaceLabels,
	)

	if err != nil {
		slog.ErrorContext(ctx, "Failed to create namespace",
			slog.String("namespace", name),
			slog.Any("error", err),
		)
		return err
	}

	switch result {
	case port.NamespaceCreated:
		slog.InfoContext(ctx, "Namespace created",
			slog.String("namespace", name),
		)
	case port.NamespaceAlreadyExists:
		slog.InfoContext(ctx, "Namespace already exists",
			slog.String("namespace", name),
		)
	}

	return nil
}

func (s *onboardingUsecase) getNamespaceAnnotations(
	ctx context.Context,
) map[string]string {
	if !s.namespace.Annotation.Enabled {
		return nil
	}

	// Copy: the static map is shared configuration, and this function runs
	// concurrently for every request.
	annotations := make(map[string]string, len(s.namespace.Annotation.Static))
	maps.Copy(annotations, s.namespace.Annotation.Static)

	if s.namespace.Annotation.Dynamic.LastLoginTimestamp {
		annotations["onyxia_last_login_timestamp"] = fmt.Sprint(time.Now().UnixMilli())
	}

	if attributes, ok := s.userContextReader.GetAttributes(ctx); ok {
		for _, attr := range s.namespace.Annotation.Dynamic.UserAttributes {
			annotations[attr] = fmt.Sprint(attributes[attr])
		}
	}
	return annotations
}

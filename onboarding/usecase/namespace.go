package usecase

import (
	"context"
	"fmt"
	"log/slog"
	"maps"
	"time"

	"github.com/onyxia-datalab/onyxia-backend/onboarding/ports"
)

func (s *onboardingUsecase) createNamespace(
	ctx context.Context,
	name string,
	userAttributes map[string]any,
) error {
	result, err := s.namespaceService.CreateNamespace(
		ctx,
		name,
		s.getNamespaceAnnotations(userAttributes),
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
	case ports.NamespaceCreated:
		slog.InfoContext(ctx, "Namespace created",
			slog.String("namespace", name),
		)
	case ports.NamespaceAlreadyExists:
		slog.InfoContext(ctx, "Namespace already exists",
			slog.String("namespace", name),
		)
	}

	return nil
}

// getNamespaceAnnotations builds the annotations of a namespace onboarded
// by a user with the given attributes (token claims).
func (s *onboardingUsecase) getNamespaceAnnotations(userAttributes map[string]any) map[string]string {
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

	if userAttributes != nil {
		for _, attr := range s.namespace.Annotation.Dynamic.UserAttributes {
			annotations[attr] = fmt.Sprint(userAttributes[attr])
		}
	}
	return annotations
}
